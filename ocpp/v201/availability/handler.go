package availability

// ============================================================================
// Availability Handler Interface - OCPP 2.0.1
// ============================================================================
// This interface defines the methods that must be implemented to handle
// availability-related messages from charging stations.
// ============================================================================

// Handler defines the interface for handling availability messages
type Handler interface {
	// OnStatusNotification handles incoming StatusNotification requests
	// Called when connector status changes
	OnStatusNotification(chargePointId string, request *StatusNotificationRequest) (*StatusNotificationResponse, error)
}
