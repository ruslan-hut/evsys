package handlers

import "evsys/ocpp"

type SystemHandler interface {
	OnBootNotification(chargePointId string, request *ocpp.BootNotificationRequest) (confirmation *ocpp.BootNotificationResponse, err error)
}
