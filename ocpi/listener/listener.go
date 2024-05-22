package listener

import (
	"evsys/ocpi/client"
)

const (
	startEndpoint  = "/transactions/start"
	stopEndpoint   = "/transactions/stop"
	statusEndpoint = "/status"
)

type Listener struct {
	client *client.Client
}

func New(client *client.Client) *Listener {
	return &Listener{
		client: client,
	}
}

func callback(_ []byte, _ error) {
}

func (l *Listener) StatusNotification(event interface{}) {
	l.client.POST(statusEndpoint, event, callback)
}

func (l *Listener) TransactionStart(event interface{}) {
	l.client.POST(startEndpoint, event, callback)
}

func (l *Listener) TransactionStop(event interface{}) {
	l.client.POST(stopEndpoint, event, callback)
}
