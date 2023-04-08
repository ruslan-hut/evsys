package core

type SystemHandler interface {
	OnBootNotification(chargePointId string, request *BootNotificationRequest) (confirmation *BootNotificationResponse, err error)
	OnAuthorize(chargePointId string, request *AuthorizeRequest) (confirmation *AuthorizeResponse, err error)
	OnHeartbeat(chargePointId string, request *HeartbeatRequest) (confirmation *HeartbeatResponse, err error)
	OnStartTransaction(chargePointId string, request *StartTransactionRequest) (confirmation *StartTransactionResponse, err error)
	OnStopTransaction(chargePointId string, request *StopTransactionRequest) (confirmation *StopTransactionResponse, err error)
	OnMeterValues(chargePointId string, request *MeterValuesRequest) (confirmation *MeterValuesResponse, err error)
	OnStatusNotification(chargePointId string, request *StatusNotificationRequest) (confirmation *StatusNotificationResponse, err error)
	OnDataTransfer(chargePointId string, request *DataTransferRequest) (confirmation *DataTransferResponse, err error)
}
