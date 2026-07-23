package power

import (
	"evsys/ocpp"
	"time"
)

type Handler interface {
	SendRequest(clientId string, request ocpp.Request) (string, error)
	// SendRequestSync queues a request and blocks until the charge point answers
	// with its raw CallResult payload, or timeout elapses.
	SendRequestSync(clientId string, request ocpp.Request, timeout time.Duration) (string, error)
	// SendRequestWithResponse queues a request and returns the channel carrying
	// the charge point's raw CallResult payload. The error reports only whether
	// the request could be queued, so the caller can tell an offline charge
	// point apart from one that has not answered yet. release must be called
	// once the caller stops listening.
	SendRequestWithResponse(clientId string, request ocpp.Request) (response <-chan string, release func(), err error)
}
