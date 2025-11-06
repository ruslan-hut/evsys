package authorization

// ============================================================================
// Authorization Handler Interface - OCPP 2.0.1
// ============================================================================
// This interface defines the methods that must be implemented to handle
// authorization-related messages from charging stations.
// ============================================================================

// Handler defines the interface for handling authorization messages
type Handler interface {
	// OnAuthorize handles incoming Authorize requests
	// Called when a charging station needs to authorize an IdToken
	OnAuthorize(chargePointId string, request *AuthorizeRequest) (*AuthorizeResponse, error)

	// OnClearedChargingLimit handles incoming ClearedChargingLimit requests
	// Called when charging station clears a charging limit
	OnClearedChargingLimit(chargePointId string, request *ClearedChargingLimitRequest) (*ClearedChargingLimitResponse, error)
}
