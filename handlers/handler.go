package handlers

import "evsys/ocpp"

type SystemHandler interface {
	OnBootNotification(chargePointId string, request *ocpp.BootNotificationRequest) (confirmation *ocpp.BootNotificationResponse, err error)
	OnAuthorize(chargePointId string, request *ocpp.AuthorizeRequest) (confirmation *ocpp.AuthorizeResponse, err error)
	OnHeartbeat(chargePointId string, request *ocpp.HeartbeatRequest) (confirmation *ocpp.HeartbeatResponse, err error)
	OnStartTransaction(chargePointId string, request *ocpp.StartTransactionRequest) (confirmation *ocpp.StartTransactionResponse, err error)
	OnStopTransaction(chargePointId string, request *ocpp.StopTransactionRequest) (confirmation *ocpp.StopTransactionResponse, err error)
}
