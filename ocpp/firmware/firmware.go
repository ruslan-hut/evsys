package firmware

type SystemHandler interface {
	OnDiagnosticsStatusNotification(chargePointId string, request *DiagnosticsStatusNotificationRequest) (confirmation *DiagnosticsStatusNotificationResponse, err error)
	//OnFirmwareStatusNotification(chargePointId string, request *FirmwareStatusNotificationRequest) (confirmation *FirmwareStatusNotificationConfirmation, err error)
}
