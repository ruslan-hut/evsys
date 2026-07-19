package provisioning

// ============================================================================
// Provisioning Handler Interface - OCPP 2.0.1
// ============================================================================
// This interface defines the methods that must be implemented to handle
// provisioning-related messages from charging stations.
// ============================================================================

// Handler defines the interface for handling provisioning messages
type Handler interface {
	// OnBootNotification handles incoming BootNotification requests
	// Called when a charging station boots or reboots
	OnBootNotification(chargePointId string, request *BootNotificationRequest) (*BootNotificationResponse, error)

	// OnHeartbeat handles incoming Heartbeat requests
	// Called periodically to maintain connection
	OnHeartbeat(chargePointId string, request *HeartbeatRequest) (*HeartbeatResponse, error)

	// OnNotifyReport handles incoming NotifyReport requests
	// Called when charging station reports its device model variables
	OnNotifyReport(chargePointId string, request *NotifyReportRequest) (*NotifyReportResponse, error)
}

// CommandHandler defines the interface for sending provisioning commands to charging stations
type CommandHandler interface {
	// SendGetBaseReport sends a GetBaseReport request to the charging station
	SendGetBaseReport(chargePointId string, request *GetBaseReportRequest) (*GetBaseReportResponse, error)

	// SendGetVariables sends a GetVariables request to the charging station
	SendGetVariables(chargePointId string, request *GetVariablesRequest) (*GetVariablesResponse, error)

	// SendSetVariables sends a SetVariables request to the charging station
	SendSetVariables(chargePointId string, request *SetVariablesRequest) (*SetVariablesResponse, error)

	// SendReset sends a Reset request to the charging station
	SendReset(chargePointId string, request *ResetRequest) (*ResetResponse, error)
}
