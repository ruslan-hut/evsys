package webhook

import (
	"context"
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	source             = "evsys"
	subscriberCacheTTL = time.Minute
	dispatchTick       = 15 * time.Second
	requestTimeout     = 15 * time.Second
	maxDeliveryAge     = 24 * time.Hour
)

// retrySchedule maps the number of failed attempts to the delay before the next one;
// past the last entry every retry waits the final delay, until maxDeliveryAge gives up.
var retrySchedule = []time.Duration{30 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}

// Webhook is an internal.EventHandler that persists every matching event to a Mongo
// outbox and delivers it to HTTP subscribers with retries. Delivery is at-least-once
// and ordered per subscriber; subscriber configuration lives in the
// webhook_subscribers collection so it can be edited without a restart.
type Webhook struct {
	database internal.Database
	logger   internal.LogHandler
	client   *http.Client

	sequence atomic.Int64
	nudge    chan struct{}
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	// dispatchMu spans sequence assignment and the outbox insert: events arrive on
	// concurrent charge-point goroutines, and without the lock a later sequence could
	// be inserted before an earlier one, breaking per-subscriber delivery order.
	dispatchMu sync.Mutex

	tick     time.Duration
	timeout  time.Duration
	maxAge   time.Duration
	schedule []time.Duration

	mu          sync.Mutex
	subscribers []entity.WebhookSubscriber
	cachedAt    time.Time
	inFlight    map[string]bool
}

// New creates a webhook service; call Start before registering it as an event listener.
func New(database internal.Database, logger internal.LogHandler) *Webhook {
	return &Webhook{
		database: database,
		logger:   logger,
		client:   &http.Client{},
		nudge:    make(chan struct{}, 1),
		tick:     dispatchTick,
		timeout:  requestTimeout,
		maxAge:   maxDeliveryAge,
		schedule: retrySchedule,
		inFlight: make(map[string]bool),
	}
}

// Start seeds the sequence counter from the outbox and launches the dispatcher.
func (w *Webhook) Start() error {
	sequence, err := w.database.GetMaxWebhookSequence()
	if err != nil {
		return fmt.Errorf("read webhook sequence: %w", err)
	}
	w.sequence.Store(sequence)

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.run(ctx)
	}()
	return nil
}

// Stop shuts the dispatcher down and waits for in-flight deliveries to finish;
// pending deliveries stay in the outbox and are picked up on the next Start.
func (w *Webhook) Stop() {
	if w.cancel != nil {
		w.cancel()
		w.wg.Wait()
	}
}

func (w *Webhook) OnStatusNotification(event *internal.EventMessage) {
	w.dispatch(TypeStatus, event)
}

func (w *Webhook) OnTransactionStart(event *internal.EventMessage) {
	w.dispatch(TypeTransactionStart, event)
}

func (w *Webhook) OnTransactionStop(event *internal.EventMessage) {
	w.dispatch(TypeTransactionStop, event)
}

func (w *Webhook) OnAuthorize(event *internal.EventMessage) {
	w.dispatch(TypeAuthorize, event)
}

func (w *Webhook) OnTransactionEvent(event *internal.EventMessage) {
	w.dispatch(TypeTransactionEvent, event)
}

func (w *Webhook) OnAlert(event *internal.EventMessage) {
	w.dispatch(TypeAlert, event)
}

func (w *Webhook) OnInfo(event *internal.EventMessage) {
	w.dispatch(TypeInfo, event)
}

// dispatch writes one outbox document per matching subscriber and nudges the
// dispatcher. The envelope is marshalled once, so every subscriber and every retry
// sends identical bytes.
func (w *Webhook) dispatch(eventType string, message *internal.EventMessage) {
	subscribers, err := w.getSubscribers()
	if err != nil {
		w.logger.Error("webhook: read subscribers", err)
		return
	}
	var matched []entity.WebhookSubscriber
	for i := range subscribers {
		if subscribers[i].Matches(eventType) {
			matched = append(matched, subscribers[i])
		}
	}
	if len(matched) == 0 {
		return
	}

	w.dispatchMu.Lock()
	defer w.dispatchMu.Unlock()

	envelope := Envelope{
		Id:       uuid.NewString(),
		Type:     eventType,
		Source:   source,
		Time:     time.Now(),
		Sequence: w.sequence.Add(1),
		Data:     message,
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		w.logger.Error("webhook: marshal envelope", err)
		return
	}

	now := time.Now()
	deliveries := make([]entity.WebhookDelivery, 0, len(matched))
	for _, subscriber := range matched {
		deliveries = append(deliveries, entity.WebhookDelivery{
			EventId:     envelope.Id,
			Subscriber:  subscriber.Name,
			Type:        eventType,
			Sequence:    envelope.Sequence,
			Payload:     payload,
			Status:      entity.WebhookPending,
			NextAttempt: now,
			CreatedAt:   now,
		})
	}
	if err = w.database.AddWebhookDeliveries(deliveries); err != nil {
		w.logger.Error("webhook: save deliveries", err)
		return
	}

	select {
	case w.nudge <- struct{}{}:
	default:
	}
}

// getSubscribers returns enabled subscribers, re-read from the database once a minute
// so edits made through the management UI apply without a restart.
func (w *Webhook) getSubscribers() ([]entity.WebhookSubscriber, error) {
	w.mu.Lock()
	if time.Since(w.cachedAt) < subscriberCacheTTL {
		subscribers := w.subscribers
		w.mu.Unlock()
		return subscribers, nil
	}
	w.mu.Unlock()

	subscribers, err := w.database.GetWebhookSubscribers()
	if err != nil {
		return nil, err
	}

	w.mu.Lock()
	w.subscribers = subscribers
	w.cachedAt = time.Now()
	w.mu.Unlock()
	return subscribers, nil
}
