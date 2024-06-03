package ocpi

import (
	"evsys/internal"
	"evsys/ocpi/authorize"
	"evsys/ocpi/client"
	"evsys/ocpi/listener"
)

type OCPI struct {
	listener *listener.Listener
	auth     *authorize.Authorize
}

func New(url, token string) *OCPI {
	cl := client.New(url, token)
	return &OCPI{
		listener: listener.New(cl),
		auth:     authorize.New(cl),
	}
}

func (o *OCPI) OnStatusNotification(event *internal.EventMessage) {
	o.listener.StatusNotification(event)
}

func (o *OCPI) OnTransactionStart(event *internal.EventMessage) {
	o.listener.TransactionStart(event)
}

func (o *OCPI) OnTransactionStop(event *internal.EventMessage) {
	o.listener.TransactionStop(event)
}

func (o *OCPI) OnAuthorize(_ *internal.EventMessage) {

}

func (o *OCPI) OnTransactionEvent(_ *internal.EventMessage) {

}

func (o *OCPI) OnAlert(_ *internal.EventMessage) {

}

func (o *OCPI) OnInfo(_ *internal.EventMessage) {

}

func (o *OCPI) Authorize(locationId, evseId, idTag string) (bool, bool, bool, string) {
	res := o.auth.Authorize(locationId, evseId, idTag)
	if res == nil {
		return false, false, false, ""
	}
	return res.Allowed, res.Expired, res.Blocked, res.Info
}
