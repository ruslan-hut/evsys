package v16

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v16/core"
	"evsys/ocpp/v16/firmware"
	"evsys/ocpp/v16/localauth"
	"evsys/ocpp/v16/remotetrigger"
	// "evsys/ocpp/v16/smartcharging" // TODO: uncomment when response types are added
	"fmt"
	"reflect"
)

// Handler16 implements the MessageHandler interface for OCPP 1.6J protocol
type Handler16 struct {
	featureRegistry common.FeatureRegistry
}

// NewHandler16 creates a new OCPP 1.6J message handler
func NewHandler16() *Handler16 {
	h := &Handler16{
		featureRegistry: common.GetGlobalRegistry(),
	}
	h.registerFeatures()
	return h
}

// registerFeatures registers all OCPP 1.6J features with the global registry
func (h *Handler16) registerFeatures() {
	version := common.OCPP16

	// Core Profile
	common.RegisterFeature(version, core.BootNotificationFeatureName,
		reflect.TypeOf(core.BootNotificationRequest{}),
		reflect.TypeOf(core.BootNotificationResponse{}))

	common.RegisterFeature(version, core.AuthorizeFeatureName,
		reflect.TypeOf(core.AuthorizeRequest{}),
		reflect.TypeOf(core.AuthorizeResponse{}))

	common.RegisterFeature(version, core.HeartbeatFeatureName,
		reflect.TypeOf(core.HeartbeatRequest{}),
		reflect.TypeOf(core.HeartbeatResponse{}))

	common.RegisterFeature(version, core.StartTransactionFeatureName,
		reflect.TypeOf(core.StartTransactionRequest{}),
		reflect.TypeOf(core.StartTransactionResponse{}))

	common.RegisterFeature(version, core.StopTransactionFeatureName,
		reflect.TypeOf(core.StopTransactionRequest{}),
		reflect.TypeOf(core.StopTransactionResponse{}))

	common.RegisterFeature(version, core.MeterValuesFeatureName,
		reflect.TypeOf(core.MeterValuesRequest{}),
		reflect.TypeOf(core.MeterValuesResponse{}))

	common.RegisterFeature(version, core.StatusNotificationFeatureName,
		reflect.TypeOf(core.StatusNotificationRequest{}),
		reflect.TypeOf(core.StatusNotificationResponse{}))

	common.RegisterFeature(version, core.DataTransferFeatureName,
		reflect.TypeOf(core.DataTransferRequest{}),
		reflect.TypeOf(core.DataTransferResponse{}))

	common.RegisterFeature(version, core.RemoteStartTransactionFeatureName,
		reflect.TypeOf(core.RemoteStartTransactionRequest{}),
		reflect.TypeOf(core.RemoteStartTransactionResponse{}))

	common.RegisterFeature(version, core.RemoteStopTransactionFeatureName,
		reflect.TypeOf(core.RemoteStopTransactionRequest{}),
		reflect.TypeOf(core.RemoteStopTransactionResponse{}))

	common.RegisterFeature(version, core.GetConfigurationFeatureName,
		reflect.TypeOf(core.GetConfigurationRequest{}),
		reflect.TypeOf(core.GetConfigurationResponse{}))

	common.RegisterFeature(version, core.ChangeConfigurationFeatureName,
		reflect.TypeOf(core.ChangeConfigurationRequest{}),
		reflect.TypeOf(core.ChangeConfigurationResponse{}))

	// Note: Reset feature has request but no response type defined in current code
	// common.RegisterFeature(version, core.ResetFeatureName, ...)

	// Firmware Management Profile
	common.RegisterFeature(version, firmware.DiagnosticsStatusNotificationFeatureName,
		reflect.TypeOf(firmware.DiagnosticsStatusNotificationRequest{}),
		reflect.TypeOf(firmware.DiagnosticsStatusNotificationResponse{}))

	common.RegisterFeature(version, firmware.StatusNotificationFeatureName,
		reflect.TypeOf(firmware.StatusNotificationRequest{}),
		reflect.TypeOf(firmware.StatusNotificationResponse{}))

	// Note: GetDiagnostics has request but no response type defined
	// common.RegisterFeature(version, firmware.GetDiagnosticsFeatureName, ...)

	// Smart Charging Profile
	// Note: Smart charging features have requests but no response types defined in current code
	// These will be registered when response types are added:
	// common.RegisterFeature(version, smartcharging.SetChargingProfileFeatureName, ...)
	// common.RegisterFeature(version, smartcharging.GetCompositeScheduleFeatureName, ...)
	// common.RegisterFeature(version, smartcharging.ClearChargingProfileFeatureName, ...)

	// Local Auth List Management Profile
	common.RegisterFeature(version, localauth.SendLocalListFeatureName,
		reflect.TypeOf(localauth.SendLocalListRequest{}),
		reflect.TypeOf(localauth.SendLocalListResponse{}))

	// Remote Trigger Profile
	common.RegisterFeature(version, remotetrigger.TriggerMessageFeatureName,
		reflect.TypeOf(remotetrigger.TriggerMessageRequest{}),
		reflect.TypeOf(remotetrigger.TriggerMessageResponse{}))
}

// HandleRequest processes incoming requests from charge points (not fully implemented - placeholder)
// This will be implemented in a later phase when we have the full handler infrastructure
func (h *Handler16) HandleRequest(ws common.VersionedWebSocket, action string, payload []byte) (common.Response, error) {
	// This is a placeholder implementation
	// The actual implementation will delegate to the SystemHandler in server package
	return nil, fmt.Errorf("HandleRequest not yet implemented for OCPP 1.6J handler")
}

// CreateRequest creates outgoing requests to charge points (not fully implemented - placeholder)
// This will be implemented in a later phase when we have the full handler infrastructure
func (h *Handler16) CreateRequest(action string, payload interface{}) (common.Request, error) {
	// This is a placeholder implementation
	// The actual implementation will create proper request objects
	return nil, fmt.Errorf("CreateRequest not yet implemented for OCPP 1.6J handler")
}

// GetVersion returns the protocol version this handler supports
func (h *Handler16) GetVersion() common.ProtocolVersion {
	return common.OCPP16
}

// SupportsFeature checks if a specific feature is supported by this handler
func (h *Handler16) SupportsFeature(action string) bool {
	return h.featureRegistry.IsSupported(common.OCPP16, action)
}

// GetSupportedFeatures returns all features supported by this handler
func (h *Handler16) GetSupportedFeatures() []string {
	return h.featureRegistry.GetFeatures(common.OCPP16)
}
