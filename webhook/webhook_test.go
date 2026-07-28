package webhook

import (
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"sort"
	"sync"
	"testing"
	"time"
)

// stubDatabase embeds the interface as a nil field, so any method the webhook
// service calls unexpectedly panics rather than silently returning a zero value.
type stubDatabase struct {
	internal.Database
	mu          sync.Mutex
	subscribers []entity.WebhookSubscriber
	outbox      []entity.WebhookDelivery
	maxSequence int64
}

func (s *stubDatabase) GetWebhookSubscribers() ([]entity.WebhookSubscriber, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]entity.WebhookSubscriber{}, s.subscribers...), nil
}

func (s *stubDatabase) AddWebhookDeliveries(deliveries []entity.WebhookDelivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outbox = append(s.outbox, deliveries...)
	return nil
}

func (s *stubDatabase) GetPendingWebhookDelivery(subscriber string, now time.Time) (*entity.WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	head := -1
	for i := range s.outbox {
		if s.outbox[i].Subscriber != subscriber || s.outbox[i].Status != entity.WebhookPending {
			continue
		}
		if head < 0 || s.outbox[i].Sequence < s.outbox[head].Sequence {
			head = i
		}
	}
	if head < 0 || s.outbox[head].NextAttempt.After(now) {
		return nil, nil
	}
	delivery := s.outbox[head]
	return &delivery, nil
}

func (s *stubDatabase) MarkWebhookDelivered(eventId, subscriber string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.outbox {
		if s.outbox[i].EventId == eventId && s.outbox[i].Subscriber == subscriber {
			s.outbox[i].Status = entity.WebhookDelivered
			s.outbox[i].DeliveredAt = time.Now()
		}
	}
	return nil
}

func (s *stubDatabase) MarkWebhookFailed(eventId, subscriber, lastError string, nextAttempt time.Time, terminal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.outbox {
		if s.outbox[i].EventId == eventId && s.outbox[i].Subscriber == subscriber {
			if terminal {
				s.outbox[i].Status = entity.WebhookFailed
			}
			s.outbox[i].LastError = lastError
			s.outbox[i].NextAttempt = nextAttempt
			s.outbox[i].Attempts++
		}
	}
	return nil
}

func (s *stubDatabase) GetMaxWebhookSequence() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSequence, nil
}

func (s *stubDatabase) deliveries() []entity.WebhookDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]entity.WebhookDelivery{}, s.outbox...)
}

type stubLogger struct{}

func (l *stubLogger) FeatureEvent(feature, id, text string) {}
func (l *stubLogger) RawDataEvent(direction, data string)   {}
func (l *stubLogger) Debug(text string)                     {}
func (l *stubLogger) Warn(text string)                      {}
func (l *stubLogger) Error(text string, err error)          {}

func newTestWebhook(db *stubDatabase) *Webhook {
	w := New(db, &stubLogger{})
	w.tick = 10 * time.Millisecond
	w.timeout = time.Second
	w.schedule = []time.Duration{0}
	return w
}

func TestSubscriberMatches(t *testing.T) {
	tests := []struct {
		name      string
		events    []string
		eventType string
		want      bool
	}{
		{"exact", []string{"transaction.start"}, "transaction.start", true},
		{"exact mismatch", []string{"transaction.start"}, "transaction.stop", false},
		{"wildcard prefix", []string{"transaction.*"}, "transaction.stop", true},
		{"wildcard prefix mismatch", []string{"transaction.*"}, "status", false},
		{"wildcard does not match bare prefix", []string{"transaction.*"}, "transaction", false},
		{"catch all", []string{"*"}, "alert", true},
		{"empty", nil, "transaction.start", false},
		{"second entry", []string{"status", "alert"}, "alert", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := entity.WebhookSubscriber{Events: tt.events}
			if got := s.Matches(tt.eventType); got != tt.want {
				t.Errorf("Matches(%q) with %v = %v, want %v", tt.eventType, tt.events, got, tt.want)
			}
		})
	}
}

func TestDispatchWritesDeliveriesPerMatchingSubscriber(t *testing.T) {
	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "gok-pi", URL: "http://a", Events: []string{"transaction.*"}, IsEnabled: true},
		{Name: "nomadus", URL: "http://b", Events: []string{"transaction.*", "status"}, IsEnabled: true},
		{Name: "alerts-only", URL: "http://c", Events: []string{"alert"}, IsEnabled: true},
	}}
	db.maxSequence = 41
	w := newTestWebhook(db)
	if err := w.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	w.Stop()

	message := &internal.EventMessage{ChargePointId: "cp1", ConnectorId: 2, TransactionId: 7}
	w.OnTransactionStart(message)

	outbox := db.deliveries()
	if len(outbox) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(outbox))
	}
	names := []string{outbox[0].Subscriber, outbox[1].Subscriber}
	sort.Strings(names)
	if names[0] != "gok-pi" || names[1] != "nomadus" {
		t.Errorf("unexpected subscribers %v", names)
	}

	var envelope Envelope
	if err := json.Unmarshal(outbox[0].Payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Type != TypeTransactionStart {
		t.Errorf("envelope type = %q, want %q", envelope.Type, TypeTransactionStart)
	}
	if envelope.Id == "" {
		t.Error("envelope id is empty")
	}
	if envelope.Sequence != 42 {
		t.Errorf("envelope sequence = %d, want 42 (seeded from outbox max 41)", envelope.Sequence)
	}
	if envelope.Source != "evsys" {
		t.Errorf("envelope source = %q, want evsys", envelope.Source)
	}
	if envelope.Data == nil || envelope.Data.TransactionId != 7 {
		t.Errorf("envelope data does not carry the event message: %+v", envelope.Data)
	}
	if string(outbox[0].Payload) != string(outbox[1].Payload) {
		t.Error("subscribers received different payload bytes for the same event")
	}
	if outbox[0].EventId != envelope.Id {
		t.Errorf("delivery event id %q does not match envelope id %q", outbox[0].EventId, envelope.Id)
	}
}

func TestDispatchNoMatchWritesNothing(t *testing.T) {
	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "alerts-only", URL: "http://c", Events: []string{"alert"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)
	w.OnTransactionStart(&internal.EventMessage{TransactionId: 1})
	if got := len(db.deliveries()); got != 0 {
		t.Fatalf("expected empty outbox, got %d deliveries", got)
	}
}

func TestBackoff(t *testing.T) {
	w := New(&stubDatabase{}, &stubLogger{})
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{0, 30 * time.Second},
		{1, time.Minute},
		{4, time.Hour},
		{5, time.Hour},
		{100, time.Hour},
	}
	for _, tt := range tests {
		if got := w.backoff(tt.attempts); got != tt.want {
			t.Errorf("backoff(%d) = %v, want %v", tt.attempts, got, tt.want)
		}
	}
}

// TestConcurrentDispatchKeepsSequenceOrder pins the outbox invariant the dispatcher
// relies on: events arrive on concurrent charge-point goroutines, but deliveries must
// land in the outbox in sequence order, or a stop could be delivered before its start.
func TestConcurrentDispatchKeepsSequenceOrder(t *testing.T) {
	db := &stubDatabase{subscribers: []entity.WebhookSubscriber{
		{Name: "nomadus", URL: "http://a", Events: []string{"*"}, IsEnabled: true},
	}}
	w := newTestWebhook(db)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w.dispatch(TypeTransactionStart, &internal.EventMessage{TransactionId: id})
		}(i)
	}
	wg.Wait()

	outbox := db.deliveries()
	if len(outbox) != 50 {
		t.Fatalf("expected 50 deliveries, got %d", len(outbox))
	}
	for i := 1; i < len(outbox); i++ {
		if outbox[i].Sequence <= outbox[i-1].Sequence {
			t.Fatalf("outbox insert order broken: sequence %d written after %d",
				outbox[i].Sequence, outbox[i-1].Sequence)
		}
	}
}

func TestClaimBlocksConcurrentDelivery(t *testing.T) {
	w := newTestWebhook(&stubDatabase{})
	if !w.claim("nomadus") {
		t.Fatal("first claim refused")
	}
	if w.claim("nomadus") {
		t.Error("second claim of the same subscriber succeeded")
	}
	if !w.claim("gok-pi") {
		t.Error("claim of a different subscriber refused")
	}
	w.release("nomadus")
	if !w.claim("nomadus") {
		t.Error("claim after release refused")
	}
}
