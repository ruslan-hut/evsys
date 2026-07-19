package transactions

// ============================================================================
// Transactions Handler Interface - OCPP 2.0.1
// ============================================================================
// This interface defines the methods that must be implemented to handle
// transaction-related messages from charging stations.
// ============================================================================

// Handler defines the interface for handling transaction messages
type Handler interface {
	// OnTransactionEvent handles incoming TransactionEvent requests
	// This is the unified message that replaces StartTransaction, StopTransaction, and MeterValues
	OnTransactionEvent(chargePointId string, request *TransactionEventRequest) (*TransactionEventResponse, error)
}
