package webhook

import (
	"evsys/internal"
	"time"
)

// Event type names carried in the envelope. Dotted so subscribers can filter
// with a prefix wildcard ("transaction.*").
const (
	TypeStatus           = "status"
	TypeTransactionStart = "transaction.start"
	TypeTransactionStop  = "transaction.stop"
	TypeTransactionEvent = "transaction.event"
	TypeAuthorize        = "authorize"
	TypeAlert            = "alert"
	TypeInfo             = "info"
)

// Envelope is the wire format posted to subscribers. Id is the consumer's dedup key
// under at-least-once delivery; Sequence is monotonic across restarts, so consumers
// can apply last-writer-wins to state updates.
type Envelope struct {
	Id       string                 `json:"id"`
	Type     string                 `json:"type"`
	Source   string                 `json:"source"`
	Time     time.Time              `json:"time"`
	Sequence int64                  `json:"sequence"`
	Data     *internal.EventMessage `json:"data"`
}
