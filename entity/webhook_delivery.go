package entity

import "time"

const (
	WebhookPending   = "pending"
	WebhookDelivered = "delivered"
	WebhookFailed    = "failed"
)

// WebhookDelivery is one outbox document in webhook_outbox: a single event addressed
// to a single subscriber. Payload is the marshalled envelope, immutable once written,
// so a retry sends exactly the bytes the event produced.
type WebhookDelivery struct {
	EventId     string    `json:"event_id" bson:"event_id"`
	Subscriber  string    `json:"subscriber" bson:"subscriber"`
	Type        string    `json:"type" bson:"type"`
	Sequence    int64     `json:"sequence" bson:"sequence"`
	Payload     []byte    `json:"payload" bson:"payload"`
	Status      string    `json:"status" bson:"status"`
	Attempts    int       `json:"attempts" bson:"attempts"`
	NextAttempt time.Time `json:"next_attempt" bson:"next_attempt"`
	LastError   string    `json:"last_error,omitempty" bson:"last_error"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	DeliveredAt time.Time `json:"delivered_at,omitempty" bson:"delivered_at,omitempty"`
}
