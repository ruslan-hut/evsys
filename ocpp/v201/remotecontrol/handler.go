package remotecontrol

// ============================================================================
// Remote Control Handler Interface - OCPP 2.0.1
// ============================================================================
// This interface defines the methods that must be implemented to send
// remote control commands to charging stations.
// ============================================================================

// Handler defines the interface for sending remote control commands
type Handler interface {
	// SendRequestStartTransaction sends a RequestStartTransaction request to the charging station
	SendRequestStartTransaction(chargePointId string, request *RequestStartTransactionRequest) (*RequestStartTransactionResponse, error)

	// SendRequestStopTransaction sends a RequestStopTransaction request to the charging station
	SendRequestStopTransaction(chargePointId string, request *RequestStopTransactionRequest) (*RequestStopTransactionResponse, error)
}
