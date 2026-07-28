package entity

import (
	"strings"
	"time"
)

// WebhookSubscriber is a webhook recipient stored in the webhook_subscribers collection.
// The bson tags are the contract with evsys-back, which will manage these documents.
type WebhookSubscriber struct {
	Name      string    `json:"name" bson:"name"`
	URL       string    `json:"url" bson:"url"`
	Token     string    `json:"token" bson:"token"`
	Events    []string  `json:"events" bson:"events"`
	IsEnabled bool      `json:"is_enabled" bson:"is_enabled"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// Matches reports whether the subscriber is interested in the given event type.
// An entry is an exact type ("transaction.start"), a prefix wildcard
// ("transaction.*") or the catch-all "*".
func (s *WebhookSubscriber) Matches(eventType string) bool {
	for _, e := range s.Events {
		if e == eventType || e == "*" {
			return true
		}
		if strings.HasSuffix(e, ".*") && strings.HasPrefix(eventType, e[:len(e)-1]) {
			return true
		}
	}
	return false
}
