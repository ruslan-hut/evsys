package webhook

import (
	"bytes"
	"context"
	"evsys/entity"
	"fmt"
	"io"
	"net/http"
	"time"
)

// run is the dispatcher loop: on every tick or nudge it starts one delivery
// goroutine per subscriber that does not already have one running.
func (w *Webhook) run(ctx context.Context) {
	ticker := time.NewTicker(w.tick)
	defer ticker.Stop()
	for {
		w.deliverAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-w.nudge:
		}
	}
}

func (w *Webhook) deliverAll(ctx context.Context) {
	subscribers, err := w.getSubscribers()
	if err != nil {
		w.logger.Error("webhook: read subscribers", err)
		return
	}
	for i := range subscribers {
		subscriber := subscribers[i]
		if !w.claim(subscriber.Name) {
			continue
		}
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer w.release(subscriber.Name)
			w.deliverSubscriber(ctx, subscriber)
		}()
	}
}

// claim marks a subscriber as having an active delivery goroutine; a second
// concurrent goroutine for the same subscriber would break per-subscriber ordering.
func (w *Webhook) claim(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.inFlight[name] {
		return false
	}
	w.inFlight[name] = true
	return true
}

func (w *Webhook) release(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.inFlight, name)
}

// deliverSubscriber drains the subscriber's queue strictly in order, oldest first.
// A failure stops the drain: the head of the queue keeps its position and newer
// events wait, so a stop event can never arrive before its start event. Only after
// maxDeliveryAge is the head abandoned and the queue moves on.
func (w *Webhook) deliverSubscriber(ctx context.Context, subscriber entity.WebhookSubscriber) {
	for ctx.Err() == nil {
		delivery, err := w.database.GetPendingWebhookDelivery(subscriber.Name, time.Now())
		if err != nil {
			w.logger.Error("webhook: read outbox for "+subscriber.Name, err)
			return
		}
		if delivery == nil {
			return
		}

		err = w.post(ctx, subscriber, delivery.Payload)
		if err == nil {
			if e := w.database.MarkWebhookDelivered(delivery.EventId, delivery.Subscriber); e != nil {
				w.logger.Error("webhook: mark delivered", e)
				return
			}
			continue
		}

		terminal := time.Since(delivery.CreatedAt) > w.maxAge
		nextAttempt := time.Now().Add(w.backoff(delivery.Attempts))
		if e := w.database.MarkWebhookFailed(delivery.EventId, delivery.Subscriber, err.Error(), nextAttempt, terminal); e != nil {
			w.logger.Error("webhook: mark failed", e)
			return
		}
		if terminal {
			w.logger.Error(fmt.Sprintf("webhook: giving up on event %s (%s) for %s after %d attempts",
				delivery.EventId, delivery.Type, subscriber.Name, delivery.Attempts+1), err)
			continue
		}
		return
	}
}

func (w *Webhook) backoff(attempts int) time.Duration {
	if attempts >= len(w.schedule) {
		return w.schedule[len(w.schedule)-1]
	}
	return w.schedule[attempts]
}

func (w *Webhook) post(ctx context.Context, subscriber entity.WebhookSubscriber, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, subscriber.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if subscriber.Token != "" {
		request.Header.Set("Authorization", "Token "+subscriber.Token)
	}

	response, err := w.client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("response status %s", response.Status)
	}
	return nil
}
