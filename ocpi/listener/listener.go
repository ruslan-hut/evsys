package listener

import (
	"evsys/internal"
	"net/http"
)

type Listener struct {
	client *http.Client
	url    string
	token  string
}

func New(url, token string) *Listener {
	return &Listener{
		url:    url,
		token:  token,
		client: &http.Client{},
	}
}

func (l *Listener) OnStatusNotification(event *internal.EventMessage) {

}

func (l *Listener) OnTransactionStart(event *internal.EventMessage) {

}

func (l *Listener) OnTransactionStop(event *internal.EventMessage) {

}

func (l *Listener) OnAuthorize(event *internal.EventMessage) {

}

func (l *Listener) OnTransactionEvent(event *internal.EventMessage) {

}

func (l *Listener) OnAlert(event *internal.EventMessage) {

}

func (l *Listener) OnInfo(event *internal.EventMessage) {

}
