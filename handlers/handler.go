package handlers

import "evsys/ocpp"

type SystemHandler interface {
	OnBootNotification(chargePointId string, request *ocpp.BootNotificationRequest) (confirmation *ocpp.BootNotificationResponse, err error)
	OnAuthorize(chargePointId string, request *ocpp.AuthorizeRequest) (confirmation *ocpp.AuthorizeResponse, err error)
	OnHeartbeat(chargePointId string, request *ocpp.HeartbeatRequest) (confirmation *ocpp.HeartbeatResponse, err error)
}
