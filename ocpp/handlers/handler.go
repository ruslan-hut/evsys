package handlers

import (
	"evsys/ocpp/core"
)

type SystemHandler interface {
	OnBootNotification(chargePointId string, request *core.BootNotificationRequest) (confirmation *core.BootNotificationResponse, err error)
	OnAuthorize(chargePointId string, request *core.AuthorizeRequest) (confirmation *core.AuthorizeResponse, err error)
	OnHeartbeat(chargePointId string, request *core.HeartbeatRequest) (confirmation *core.HeartbeatResponse, err error)
	OnStartTransaction(chargePointId string, request *core.StartTransactionRequest) (confirmation *core.StartTransactionResponse, err error)
	OnStopTransaction(chargePointId string, request *core.StopTransactionRequest) (confirmation *core.StopTransactionResponse, err error)
	OnMeterValues(chargePointId string, request *core.MeterValuesRequest) (confirmation *core.MeterValuesResponse, err error)
	OnStatusNotification(chargePointId string, request *core.StatusNotificationRequest) (confirmation *core.StatusNotificationResponse, err error)
	OnDataTransfer(chargePointId string, request *core.DataTransferRequest) (confirmation *core.DataTransferResponse, err error)
}
