package webhook

import (
	"context"
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// recordingServer captures every accepted request body and can be told to fail
// the first n requests.
type recordingServer struct {
	mu       sync.Mutex
	failures int
	requests int
	bodies   []Envelope
	tokens   []string
}

func (r *recordingServer) handler(t *testing.T) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.requests++
		if r.failures > 0 {
			r.failures--
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		var envelope Envelope
		if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		r.bodies = append(r.bodies, envelope)
		r.tokens = append(r.tokens, request.Header.Get("Authorization"))
		writer.WriteHeader(http.StatusOK)
	}
}

func (r *recordingServer) received() []Envelope {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Envelope{}, r.bodies...)
}

func (r *recordingServer) requestCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests
}

func enqueue(t *testing.T, w *Webhook, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		w.dispatch(TypeTransactionStart, &internal.EventMessage{TransactionId: i + 1})
	}
}

func TestDeliverSubscriberDrainsInOrder(t *testing.T) {
	server := &recordingServer{}
	ts := httptest.NewServer(server.handler(t))
	defer ts.Close()

	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "nomadus", URL: ts.URL, Token: "secret", Events: []string{"transaction.*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)
	enqueue(t, w, 3)

	w.deliverSubscriber(context.Background(), db.subscribers[0])

	envelopes := server.received()
	if len(envelopes) != 3 {
		t.Fatalf("expected 3 deliveries, got %d", len(envelopes))
	}
	for i := 1; i < len(envelopes); i++ {
		if envelopes[i].Sequence <= envelopes[i-1].Sequence {
			t.Errorf("delivery out of order: sequence %d after %d", envelopes[i].Sequence, envelopes[i-1].Sequence)
		}
	}
	if server.tokens[0] != "Token secret" {
		t.Errorf("authorization header = %q, want %q", server.tokens[0], "Token secret")
	}
	for _, delivery := range db.deliveries() {
		if delivery.Status != entity.WebhookDelivered {
			t.Errorf("delivery %d status = %q, want delivered", delivery.Sequence, delivery.Status)
		}
	}
}

// TestFailureBlocksQueue is the ordering guarantee: while the head of the queue
// cannot be delivered, newer events for the same subscriber must wait.
func TestFailureBlocksQueue(t *testing.T) {
	server := &recordingServer{failures: 1000}
	ts := httptest.NewServer(server.handler(t))
	defer ts.Close()

	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "nomadus", URL: ts.URL, Events: []string{"transaction.*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)
	w.schedule = []time.Duration{time.Hour}
	enqueue(t, w, 2)

	w.deliverSubscriber(context.Background(), db.subscribers[0])

	if got := server.requestCount(); got != 1 {
		t.Fatalf("expected exactly 1 request (head of queue), got %d", got)
	}
	outbox := db.deliveries()
	if outbox[0].Attempts != 1 || outbox[0].Status != entity.WebhookPending {
		t.Errorf("head delivery: attempts=%d status=%q, want 1 pending", outbox[0].Attempts, outbox[0].Status)
	}
	if !outbox[0].NextAttempt.After(time.Now().Add(30 * time.Minute)) {
		t.Errorf("head next_attempt %v not pushed out by the schedule", outbox[0].NextAttempt)
	}
	if outbox[1].Attempts != 0 {
		t.Errorf("second delivery was attempted while the head was blocked")
	}
}

func TestRetrySucceedsWhenDue(t *testing.T) {
	server := &recordingServer{failures: 1}
	ts := httptest.NewServer(server.handler(t))
	defer ts.Close()

	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "nomadus", URL: ts.URL, Events: []string{"transaction.*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db) // schedule {0}: retries are due immediately
	enqueue(t, w, 2)

	w.deliverSubscriber(context.Background(), db.subscribers[0])
	w.deliverSubscriber(context.Background(), db.subscribers[0])

	if got := len(server.received()); got != 2 {
		t.Fatalf("expected both events delivered after retry, got %d", got)
	}
	for _, delivery := range db.deliveries() {
		if delivery.Status != entity.WebhookDelivered {
			t.Errorf("delivery %d status = %q, want delivered", delivery.Sequence, delivery.Status)
		}
	}
}

// TestTerminalFailureUnblocksQueue: once the head is older than maxAge it is
// abandoned, and only then does the queue move on.
func TestTerminalFailureUnblocksQueue(t *testing.T) {
	server := &recordingServer{failures: 1000}
	ts := httptest.NewServer(server.handler(t))
	defer ts.Close()

	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "nomadus", URL: ts.URL, Events: []string{"transaction.*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)
	w.maxAge = -time.Second // every delivery is immediately past its deadline
	enqueue(t, w, 2)

	w.deliverSubscriber(context.Background(), db.subscribers[0])

	outbox := db.deliveries()
	for _, delivery := range outbox {
		if delivery.Status != entity.WebhookFailed {
			t.Errorf("delivery %d status = %q, want failed", delivery.Sequence, delivery.Status)
		}
		if delivery.LastError == "" {
			t.Errorf("delivery %d has no last_error", delivery.Sequence)
		}
	}
	if got := server.requestCount(); got != 2 {
		t.Errorf("expected the queue to move on after a terminal failure, got %d requests", got)
	}
}

func TestDeliverAllIsolatesSubscribers(t *testing.T) {
	healthy := &recordingServer{}
	tsHealthy := httptest.NewServer(healthy.handler(t))
	defer tsHealthy.Close()
	broken := &recordingServer{failures: 1000}
	tsBroken := httptest.NewServer(broken.handler(t))
	defer tsBroken.Close()

	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "broken", URL: tsBroken.URL, Events: []string{"*"}, IsEnabled: true},
		{Name: "healthy", URL: tsHealthy.URL, Events: []string{"*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)
	w.schedule = []time.Duration{time.Hour}
	enqueue(t, w, 2)

	w.deliverAll(context.Background())
	deadline := time.Now().Add(2 * time.Second)
	for len(healthy.received()) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if got := len(healthy.received()); got != 2 {
		t.Errorf("healthy subscriber received %d events, want 2", got)
	}
	if got := broken.requestCount(); got != 1 {
		t.Errorf("broken subscriber saw %d requests, want 1 (blocked head)", got)
	}
}

func TestStartDeliversEndToEnd(t *testing.T) {
	server := &recordingServer{}
	ts := httptest.NewServer(server.handler(t))
	defer ts.Close()

	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "nomadus", URL: ts.URL, Events: []string{"transaction.*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)
	if err := w.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer w.Stop()

	w.OnTransactionStop(&internal.EventMessage{TransactionId: 5, Consumed: 1200})

	deadline := time.Now().Add(2 * time.Second)
	for len(server.received()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	envelopes := server.received()
	if len(envelopes) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(envelopes))
	}
	if envelopes[0].Type != TypeTransactionStop {
		t.Errorf("type = %q, want %q", envelopes[0].Type, TypeTransactionStop)
	}
	if envelopes[0].Data.Consumed != 1200 {
		t.Errorf("data.consumed = %d, want 1200", envelopes[0].Data.Consumed)
	}
}
