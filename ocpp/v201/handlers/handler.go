package handlers

import (
	"encoding/json"
	"evsys/ocpp/common"
	"evsys/ocpp/v201/authorization"
	"evsys/ocpp/v201/availability"
	"evsys/ocpp/v201/provisioning"
	"evsys/ocpp/v201/remotecontrol"
	"evsys/ocpp/v201/transactions"
	"fmt"
	"reflect"
)

// ============================================================================
// Handler201 - OCPP 2.0.1 Message Handler
// ============================================================================
// This is the main handler for OCPP 2.0.1 protocol. It implements the
// common.MessageHandler interface and routes incoming messages to the
// appropriate handler based on the feature name.
// ============================================================================

// Handler201 implements the MessageHandler interface for OCPP 2.0.1 protocol
type Handler201 struct {
	featureRegistry        common.FeatureRegistry
	provisioningHandler    provisioning.Handler
	authorizationHandler   authorization.Handler
	transactionsHandler    transactions.Handler
	availabilityHandler    availability.Handler
	remoteControlHandler   remotecontrol.Handler
	provisioningCmdHandler provisioning.CommandHandler
}

// Handler201Config holds the configuration for Handler201
type Handler201Config struct {
	ProvisioningHandler    provisioning.Handler
	AuthorizationHandler   authorization.Handler
	TransactionsHandler    transactions.Handler
	AvailabilityHandler    availability.Handler
	RemoteControlHandler   remotecontrol.Handler
	ProvisioningCmdHandler provisioning.CommandHandler
}

// NewHandler201 creates a new OCPP 2.0.1 message handler
func NewHandler201(config Handler201Config) *Handler201 {
	h := &Handler201{
		featureRegistry:        common.GetGlobalRegistry(),
		provisioningHandler:    config.ProvisioningHandler,
		authorizationHandler:   config.AuthorizationHandler,
		transactionsHandler:    config.TransactionsHandler,
		availabilityHandler:    config.AvailabilityHandler,
		remoteControlHandler:   config.RemoteControlHandler,
		provisioningCmdHandler: config.ProvisioningCmdHandler,
	}
	h.registerFeatures()
	return h
}

// registerFeatures registers all OCPP 2.0.1 features with the global registry
func (h *Handler201) registerFeatures() {
	version := common.OCPP201

	// ========================================================================
	// PROVISIONING FEATURES (Charging Station → CSMS)
	// ========================================================================

	common.RegisterFeature(version, provisioning.BootNotificationFeatureName,
		reflect.TypeOf(provisioning.BootNotificationRequest{}),
		reflect.TypeOf(provisioning.BootNotificationResponse{}))

	common.RegisterFeature(version, provisioning.HeartbeatFeatureName,
		reflect.TypeOf(provisioning.HeartbeatRequest{}),
		reflect.TypeOf(provisioning.HeartbeatResponse{}))

	common.RegisterFeature(version, provisioning.NotifyReportFeatureName,
		reflect.TypeOf(provisioning.NotifyReportRequest{}),
		reflect.TypeOf(provisioning.NotifyReportResponse{}))

	// Provisioning Commands (CSMS → Charging Station)
	common.RegisterFeature(version, provisioning.GetBaseReportFeatureName,
		reflect.TypeOf(provisioning.GetBaseReportRequest{}),
		reflect.TypeOf(provisioning.GetBaseReportResponse{}))

	common.RegisterFeature(version, provisioning.GetVariablesFeatureName,
		reflect.TypeOf(provisioning.GetVariablesRequest{}),
		reflect.TypeOf(provisioning.GetVariablesResponse{}))

	common.RegisterFeature(version, provisioning.SetVariablesFeatureName,
		reflect.TypeOf(provisioning.SetVariablesRequest{}),
		reflect.TypeOf(provisioning.SetVariablesResponse{}))

	common.RegisterFeature(version, provisioning.ResetFeatureName,
		reflect.TypeOf(provisioning.ResetRequest{}),
		reflect.TypeOf(provisioning.ResetResponse{}))

	// ========================================================================
	// AUTHORIZATION FEATURES
	// ========================================================================

	common.RegisterFeature(version, authorization.AuthorizeFeatureName,
		reflect.TypeOf(authorization.AuthorizeRequest{}),
		reflect.TypeOf(authorization.AuthorizeResponse{}))

	common.RegisterFeature(version, authorization.ClearedChargingLimitFeatureName,
		reflect.TypeOf(authorization.ClearedChargingLimitRequest{}),
		reflect.TypeOf(authorization.ClearedChargingLimitResponse{}))

	// ========================================================================
	// REMOTE CONTROL FEATURES (CSMS → Charging Station)
	// ========================================================================

	common.RegisterFeature(version, remotecontrol.RequestStartTransactionFeatureName,
		reflect.TypeOf(remotecontrol.RequestStartTransactionRequest{}),
		reflect.TypeOf(remotecontrol.RequestStartTransactionResponse{}))

	common.RegisterFeature(version, remotecontrol.RequestStopTransactionFeatureName,
		reflect.TypeOf(remotecontrol.RequestStopTransactionRequest{}),
		reflect.TypeOf(remotecontrol.RequestStopTransactionResponse{}))

	// ========================================================================
	// TRANSACTION FEATURES
	// ========================================================================

	common.RegisterFeature(version, transactions.TransactionEventFeatureName,
		reflect.TypeOf(transactions.TransactionEventRequest{}),
		reflect.TypeOf(transactions.TransactionEventResponse{}))

	// ========================================================================
	// AVAILABILITY FEATURES
	// ========================================================================

	common.RegisterFeature(version, availability.StatusNotificationFeatureName,
		reflect.TypeOf(availability.StatusNotificationRequest{}),
		reflect.TypeOf(availability.StatusNotificationResponse{}))
}

// HandleRequest processes incoming requests from charge points
// This implements the common.MessageHandler interface
func (h *Handler201) HandleRequest(ws common.VersionedWebSocket, action string, payload []byte) (common.Response, error) {
	chargePointId := ws.ID()

	// Get the request type from the registry
	reqType, _, err := h.featureRegistry.GetTypes(common.OCPP201, action)
	if err != nil {
		return nil, fmt.Errorf("unsupported action: %s - %w", action, err)
	}

	// Create a new instance of the request type
	request := reflect.New(reqType).Interface()

	// Unmarshal the payload into the request
	if err := json.Unmarshal(payload, request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request for %s: %w", action, err)
	}

	// Validate the request if it implements the Validate method
	if validator, ok := request.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for %s: %w", action, err)
		}
	}

	// Route to the appropriate handler based on action
	switch action {
	// ========================================================================
	// PROVISIONING FEATURES
	// ========================================================================
	case provisioning.BootNotificationFeatureName:
		if h.provisioningHandler == nil {
			return nil, fmt.Errorf("provisioning handler not configured")
		}
		req := request.(*provisioning.BootNotificationRequest)
		return h.provisioningHandler.OnBootNotification(chargePointId, req)

	case provisioning.HeartbeatFeatureName:
		if h.provisioningHandler == nil {
			return nil, fmt.Errorf("provisioning handler not configured")
		}
		req := request.(*provisioning.HeartbeatRequest)
		return h.provisioningHandler.OnHeartbeat(chargePointId, req)

	case provisioning.NotifyReportFeatureName:
		if h.provisioningHandler == nil {
			return nil, fmt.Errorf("provisioning handler not configured")
		}
		req := request.(*provisioning.NotifyReportRequest)
		return h.provisioningHandler.OnNotifyReport(chargePointId, req)

	// ========================================================================
	// AUTHORIZATION FEATURES
	// ========================================================================
	case authorization.AuthorizeFeatureName:
		if h.authorizationHandler == nil {
			return nil, fmt.Errorf("authorization handler not configured")
		}
		req := request.(*authorization.AuthorizeRequest)
		return h.authorizationHandler.OnAuthorize(chargePointId, req)

	case authorization.ClearedChargingLimitFeatureName:
		if h.authorizationHandler == nil {
			return nil, fmt.Errorf("authorization handler not configured")
		}
		req := request.(*authorization.ClearedChargingLimitRequest)
		return h.authorizationHandler.OnClearedChargingLimit(chargePointId, req)

	// ========================================================================
	// TRANSACTION FEATURES
	// ========================================================================
	case transactions.TransactionEventFeatureName:
		if h.transactionsHandler == nil {
			return nil, fmt.Errorf("transactions handler not configured")
		}
		req := request.(*transactions.TransactionEventRequest)
		return h.transactionsHandler.OnTransactionEvent(chargePointId, req)

	// ========================================================================
	// AVAILABILITY FEATURES
	// ========================================================================
	case availability.StatusNotificationFeatureName:
		if h.availabilityHandler == nil {
			return nil, fmt.Errorf("availability handler not configured")
		}
		req := request.(*availability.StatusNotificationRequest)
		return h.availabilityHandler.OnStatusNotification(chargePointId, req)

	default:
		return nil, fmt.Errorf("no handler configured for action: %s", action)
	}
}

// CreateRequest creates outgoing requests to charge points
// This implements the common.MessageHandler interface
func (h *Handler201) CreateRequest(action string, payload interface{}) (common.Request, error) {
	// Validate that we support this action
	if !h.featureRegistry.IsSupported(common.OCPP201, action) {
		return nil, fmt.Errorf("unsupported action: %s", action)
	}

	// Cast the payload to a Request
	request, ok := payload.(common.Request)
	if !ok {
		return nil, fmt.Errorf("payload does not implement Request interface")
	}

	// Validate the request if it implements Validate
	if validator, ok := request.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return nil, fmt.Errorf("validation failed for %s: %w", action, err)
		}
	}

	return request, nil
}

// GetVersion returns the protocol version this handler supports
func (h *Handler201) GetVersion() common.ProtocolVersion {
	return common.OCPP201
}

// SupportsFeature checks if a specific feature is supported by this handler
func (h *Handler201) SupportsFeature(action string) bool {
	return h.featureRegistry.IsSupported(common.OCPP201, action)
}

// GetSupportedFeatures returns all features supported by this handler
func (h *Handler201) GetSupportedFeatures() []string {
	return h.featureRegistry.GetFeatures(common.OCPP201)
}

// GetFeatureCount returns the number of features registered for this handler
func (h *Handler201) GetFeatureCount() int {
	return h.featureRegistry.GetFeatureCount(common.OCPP201)
}
