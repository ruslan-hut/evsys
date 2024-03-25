package power

import "evsys/ocpp"

type Handler interface {
	SendRequest(clientId string, request ocpp.Request) (string, error)
}
