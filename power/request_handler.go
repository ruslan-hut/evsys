package power

import "evsys/ocpp"

type Handler interface {
	SendRequest(clientId string, request ocpp.Request) (string, error)
	// SendRequestWithResponse queues a request and returns the channel carrying
	// the charge point's raw CallResult payload. The error reports only whether
	// the request could be queued, so the caller can tell an offline charge
	// point apart from one that has not answered yet. release must be called
	// once the caller stops listening.
	SendRequestWithResponse(clientId string, request ocpp.Request) (response <-chan string, release func(), err error)
}
