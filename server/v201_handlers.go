package server

import (
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp"
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
	"evsys/ocpp/v201/authorization"
	"evsys/ocpp/v201/availability"
	"evsys/ocpp/v201/provisioning"
	"evsys/ocpp/v201/remotecontrol"
	"evsys/ocpp/v201/transactions"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// OCPP 2.0.1 Business Logic Handlers
// ============================================================================
// These handlers implement the v201 handler interfaces and bridge between
// the OCPP 2.0.1 protocol and the existing business logic (SystemHandler).
// They use the ProtocolAdapter to convert between protocol versions.
// ============================================================================

// V201Handlers aggregates all OCPP 2.0.1 handler implementations
type V201Handlers struct {
	systemHandler   *SystemHandler
	protocolAdapter *ProtocolAdapter
	logger          internal.LogHandler
}

// NewV201Handlers creates a new set of OCPP 2.0.1 handlers
func NewV201Handlers(systemHandler *SystemHandler, logger internal.LogHandler) *V201Handlers {
	return &V201Handlers{
		systemHandler:   systemHandler,
		protocolAdapter: systemHandler.GetProtocolAdapter(),
		logger:          logger,
	}
}

// ============================================================================
// PROVISIONING HANDLER
// ============================================================================

// OnBootNotification handles OCPP 2.0.1 BootNotification requests
func (h *V201Handlers) OnBootNotification(chargePointId string, request *provisioning.BootNotificationRequest) (*provisioning.BootNotificationResponse, error) {
	h.logger.FeatureEvent("BootNotification", chargePointId, fmt.Sprintf("v2.0.1: %s %s (reason: %s)",
		request.ChargingStation.VendorName, request.ChargingStation.Model, request.Reason))

	// Get or create charge point state
	h.systemHandler.mux.Lock()
	state, ok := h.systemHandler.getChargePoint(chargePointId)
	if !ok {
		if !h.systemHandler.acceptPoints {
			h.systemHandler.mux.Unlock()
			return &provisioning.BootNotificationResponse{
				Status:      v201.RegistrationStatusRejected,
				CurrentTime: time.Now(),
				Interval:    0,
			}, nil
		}
		state = h.systemHandler.addChargePoint(chargePointId)
	}
	h.systemHandler.mux.Unlock()

	// Update charge point information
	if state.model != nil {
		state.model.Vendor = request.ChargingStation.VendorName
		state.model.Model = request.ChargingStation.Model
		state.model.SerialNumber = request.ChargingStation.SerialNumber
		state.model.FirmwareVersion = request.ChargingStation.FirmwareVersion
		state.model.ProtocolVersion = string(common.OCPP201)

		// Update in database
		if h.systemHandler.database != nil {
			_ = h.systemHandler.database.UpdateChargePoint(state.model)
		}
	}

	// Send heartbeat interval back
	response := &provisioning.BootNotificationResponse{
		Status:      v201.RegistrationStatusAccepted,
		CurrentTime: h.systemHandler.getTime(),
		Interval:    defaultHeartbeatInterval,
	}

	return response, nil
}

// OnHeartbeat handles OCPP 2.0.1 Heartbeat requests
func (h *V201Handlers) OnHeartbeat(chargePointId string, request *provisioning.HeartbeatRequest) (*provisioning.HeartbeatResponse, error) {
	h.logger.FeatureEvent("Heartbeat", chargePointId, "v2.0.1")

	response := &provisioning.HeartbeatResponse{
		CurrentTime: h.systemHandler.getTime(),
	}

	return response, nil
}

// OnNotifyReport handles OCPP 2.0.1 NotifyReport requests
func (h *V201Handlers) OnNotifyReport(chargePointId string, request *provisioning.NotifyReportRequest) (*provisioning.NotifyReportResponse, error) {
	h.logger.FeatureEvent("NotifyReport", chargePointId, fmt.Sprintf("v2.0.1: requestId=%d, seqNo=%d",
		request.RequestId, request.SeqNo))

	// Store device model data in charge point's DeviceModel field
	h.systemHandler.mux.Lock()
	state, ok := h.systemHandler.getChargePoint(chargePointId)
	if ok && state.model != nil {
		if state.model.DeviceModel == nil {
			state.model.DeviceModel = make(map[string]interface{})
		}
		// Store report data (simplified - in production you'd parse ReportData properly)
		state.model.DeviceModel["last_report"] = map[string]interface{}{
			"request_id":   request.RequestId,
			"seq_no":       request.SeqNo,
			"generated_at": request.GeneratedAt,
			"tbc":          request.Tbc,
		}

		if h.systemHandler.database != nil {
			_ = h.systemHandler.database.UpdateChargePoint(state.model)
		}
	}
	h.systemHandler.mux.Unlock()

	response := &provisioning.NotifyReportResponse{}
	return response, nil
}

// ============================================================================
// AUTHORIZATION HANDLER
// ============================================================================

// OnAuthorize handles OCPP 2.0.1 Authorize requests
func (h *V201Handlers) OnAuthorize(chargePointId string, request *authorization.AuthorizeRequest) (*authorization.AuthorizeResponse, error) {
	idTag := h.protocolAdapter.IdToken201ToIdTag(&request.IdToken)
	h.logger.FeatureEvent("Authorize", chargePointId, fmt.Sprintf("v2.0.1: idToken=%s (%s)", idTag, request.IdToken.Type))

	// Get user tag
	userTag := h.systemHandler.getUserTag(idTag)

	// Prepare response
	response := &authorization.AuthorizeResponse{}

	if !userTag.IsEnabled && !h.systemHandler.acceptTags {
		response.IdTokenInfo = v201.IdTokenInfo{
			Status: v201.AuthorizationStatusInvalid,
		}
		return response, nil
	}

	// Try OCPI authorization if available
	h.systemHandler.mux.Lock()
	state, ok := h.systemHandler.getChargePoint(chargePointId)
	h.systemHandler.mux.Unlock()

	locationId := ""
	evseId := ""
	if ok && state.model != nil {
		locationId = state.model.LocationId
		// Note: AuthorizeRequest doesn't include EVSE information in OCPP 2.0.1
		// EVSE context comes from TransactionEvent instead
	}

	authResult, err := h.systemHandler.authorize(locationId, evseId, idTag)
	if err == nil && authResult != nil {
		if authResult.blocked {
			response.IdTokenInfo = v201.IdTokenInfo{
				Status: v201.AuthorizationStatusBlocked,
			}
			return response, nil
		}
		if authResult.expired {
			response.IdTokenInfo = v201.IdTokenInfo{
				Status: v201.AuthorizationStatusExpired,
			}
			return response, nil
		}
		if !authResult.allowed {
			response.IdTokenInfo = v201.IdTokenInfo{
				Status: v201.AuthorizationStatusInvalid,
			}
			return response, nil
		}
	}

	// Authorization successful
	response.IdTokenInfo = v201.IdTokenInfo{
		Status: v201.AuthorizationStatusAccepted,
	}

	return response, nil
}

// OnClearedChargingLimit handles OCPP 2.0.1 ClearedChargingLimit requests
func (h *V201Handlers) OnClearedChargingLimit(chargePointId string, request *authorization.ClearedChargingLimitRequest) (*authorization.ClearedChargingLimitResponse, error) {
	h.logger.FeatureEvent("ClearedChargingLimit", chargePointId, fmt.Sprintf("v2.0.1: source=%s", request.ChargingLimitSource))

	// This is primarily informational - charging station notifies that a limit has been cleared
	response := &authorization.ClearedChargingLimitResponse{}
	return response, nil
}

// ============================================================================
// TRANSACTIONS HANDLER
// ============================================================================

// OnTransactionEvent handles OCPP 2.0.1 TransactionEvent requests
// This replaces StartTransaction, StopTransaction, and MeterValues from 1.6J
func (h *V201Handlers) OnTransactionEvent(chargePointId string, request *transactions.TransactionEventRequest) (*transactions.TransactionEventResponse, error) {
	h.logger.FeatureEvent("TransactionEvent", chargePointId, fmt.Sprintf("v2.0.1: type=%s, trigger=%s, txId=%s",
		request.EventType, request.TriggerReason, request.TransactionInfo.TransactionId))

	// Convert OCPP 2.0.1 TransactionEvent to internal Transaction using protocol adapter
	transaction, err := h.protocolAdapter.TransactionEventToEntity(
		request.EventType,
		request.TransactionInfo,
		request.IdToken,
		request.Evse,
		request.MeterValue,
		request.Timestamp,
		chargePointId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert transaction event: %w", err)
	}

	// Get connector
	h.systemHandler.mux.Lock()
	state, ok := h.systemHandler.getChargePoint(chargePointId)
	if !ok {
		h.systemHandler.mux.Unlock()
		return nil, fmt.Errorf("charge point not found: %s", chargePointId)
	}

	var evseId *int
	if request.Evse != nil {
		evseId = &request.Evse.Id
	}
	connector := h.systemHandler.getConnectorByEvseAndConnectorId(state, evseId, transaction.ConnectorId)
	if connector == nil {
		h.systemHandler.mux.Unlock()
		return nil, fmt.Errorf("connector %d not found on %s", transaction.ConnectorId, chargePointId)
	}

	// Update EVSE ID if provided
	if evseId != nil {
		_ = h.systemHandler.updateConnectorEvseId(connector, evseId)
	}
	h.systemHandler.mux.Unlock()

	response := &transactions.TransactionEventResponse{}

	// Handle based on event type
	switch request.EventType {
	case v201.TransactionEventStarted:
		// Transaction starting
		transaction.Init()
		h.systemHandler.setTransactionProtocolVersion(transaction, chargePointId)

		// Get user tag and username
		if transaction.IdTag != "" {
			userTag := h.systemHandler.getUserTag(transaction.IdTag)
			transaction.UserTag = userTag
			transaction.Username = userTag.Username
		}

		// Set transaction ID
		newTransactionId++
		transaction.Id = newTransactionId

		// Call billing service
		if h.systemHandler.billing != nil {
			_ = h.systemHandler.billing.OnTransactionStart(transaction)
		}

		// Save to database
		if h.systemHandler.database != nil {
			_ = h.systemHandler.database.AddTransaction(transaction)
		}

		// Update connector
		connector.Lock()
		connector.CurrentTransactionId = transaction.Id
		connector.Unlock()
		if h.systemHandler.database != nil {
			_ = h.systemHandler.database.UpdateConnector(connector)
		}

		// Register transaction
		state.registerTransaction(transaction.Id)
		h.systemHandler.updateActiveTransactionsCounter()

		// Notify event listeners
		go h.systemHandler.notifyEventListeners(internal.TransactionStart, &internal.EventMessage{
			ChargePointId: chargePointId,
			ConnectorId:   connector.Id,
			TransactionId: transaction.Id,
			Username:      transaction.Username,
			IdTag:         transaction.IdTag,
			Time:          transaction.TimeStart,
		})

		log.Printf("Transaction %d started on %s connector %d (OCPP 2.0.1)", transaction.Id, chargePointId, connector.Id)

	case v201.TransactionEventUpdated:
		// Transaction update (e.g., meter values)
		if h.systemHandler.database != nil {
			// Find existing transaction
			existingTx, err := h.systemHandler.database.GetTransaction(transaction.Id)
			if err == nil && existingTx != nil {
				// Process meter values
				for _, meterValue := range request.MeterValue {
					tm, err := h.protocolAdapter.MeterValue201ToTransactionMeter(meterValue, existingTx.Id)
					if err == nil {
						// Calculate price
						if h.systemHandler.billing != nil {
							_ = h.systemHandler.billing.OnMeterValue(existingTx, tm)
						}
						// Save meter value
						_ = h.systemHandler.database.AddTransactionMeterValue(tm)
					}
				}
			}
		}

	case v201.TransactionEventEnded:
		// Transaction ended
		if h.systemHandler.database != nil {
			existingTx, err := h.systemHandler.database.GetTransaction(transaction.Id)
			if err == nil && existingTx != nil {
				existingTx.Lock()
				existingTx.IsFinished = true
				existingTx.TimeStop = transaction.TimeStop
				existingTx.MeterStop = transaction.MeterStop
				existingTx.Reason = transaction.Reason

				// Call billing service
				if h.systemHandler.billing != nil {
					_ = h.systemHandler.billing.OnTransactionFinished(existingTx)
				}

				// Update in database
				_ = h.systemHandler.database.UpdateTransaction(existingTx)
				existingTx.Unlock()

				// Update connector
				connector.Lock()
				connector.CurrentTransactionId = -1
				connector.Unlock()
				if h.systemHandler.database != nil {
					_ = h.systemHandler.database.UpdateConnector(connector)
				}

				// Unregister transaction
				state.unregisterTransaction(existingTx.Id)
				h.systemHandler.updateActiveTransactionsCounter()

				// Notify event listeners
				go h.systemHandler.notifyEventListeners(internal.TransactionStop, &internal.EventMessage{
					ChargePointId: chargePointId,
					ConnectorId:   connector.Id,
					TransactionId: existingTx.Id,
					Username:      existingTx.Username,
					IdTag:         existingTx.IdTag,
					Time:          existingTx.TimeStop,
				})

				log.Printf("Transaction %d stopped on %s connector %d (OCPP 2.0.1)", existingTx.Id, chargePointId, connector.Id)
			}
		}
	}

	return response, nil
}

// ============================================================================
// AVAILABILITY HANDLER
// ============================================================================

// OnStatusNotification handles OCPP 2.0.1 StatusNotification requests
func (h *V201Handlers) OnStatusNotification(chargePointId string, request *availability.StatusNotificationRequest) (*availability.StatusNotificationResponse, error) {
	evseId := request.EvseId
	connectorId := request.ConnectorId

	h.logger.FeatureEvent("StatusNotification", chargePointId, fmt.Sprintf("v2.0.1: EVSE=%d, connector=%d, status=%s",
		evseId, connectorId, request.ConnectorStatus))

	// Get connector
	h.systemHandler.mux.Lock()
	defer h.systemHandler.mux.Unlock()

	state, ok := h.systemHandler.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found: %s", chargePointId)
	}

	connector := h.systemHandler.getConnectorByEvseAndConnectorId(state, &evseId, connectorId)
	if connector == nil {
		// Create connector if it doesn't exist
		connector = entity.NewConnector(connectorId, chargePointId)
		connector.EvseId = &evseId
		state.connectors[connectorId] = connector
		if h.systemHandler.database != nil {
			_ = h.systemHandler.database.AddConnector(connector)
		}
	}

	// Update connector status
	connector.Lock()
	connector.Status = string(request.ConnectorStatus)
	connector.StatusTime = request.Timestamp
	connector.Unlock()

	if h.systemHandler.database != nil {
		_ = h.systemHandler.database.UpdateConnector(connector)
	}

	// Notify event listeners
	go h.systemHandler.notifyEventListeners(internal.StatusNotification, &internal.EventMessage{
		LocationId: state.model.LocationId,
		Evse:       state.EvseId(connectorId),
		Status:     connector.Status,
	})

	response := &availability.StatusNotificationResponse{}
	return response, nil
}

// ============================================================================
// API COMMAND HANDLERS (CSMS → Charging Station)
// ============================================================================
// These handlers create outgoing requests to charge points via the API.
// ============================================================================

// OnRequestStartTransaction creates a RequestStartTransaction request for OCPP 2.0.1
func (h *V201Handlers) OnRequestStartTransaction(chargePointId string, connectorId int, payload string) (ocpp.Request, error) {
	h.logger.FeatureEvent("RequestStartTransaction", chargePointId, fmt.Sprintf("v2.0.1: connector=%d", connectorId))

	// Parse IdToken from payload (expected format: "userId" or JSON object)
	idToken := v201.IdToken{
		IdToken: payload,
		Type:    v201.IdTokenTypeLocal, // Default to Local type
	}

	// Try to parse as JSON if it looks like JSON
	if len(payload) > 0 && payload[0] == '{' {
		var tokenData struct {
			IdToken string `json:"idToken"`
			Type    string `json:"type"`
		}
		if err := json.Unmarshal([]byte(payload), &tokenData); err == nil {
			idToken.IdToken = tokenData.IdToken
			if tokenData.Type != "" {
				idToken.Type = v201.IdTokenType(tokenData.Type)
			}
		}
	}

	request := &remotecontrol.RequestStartTransactionRequest{
		IdToken:       idToken,
		RemoteStartId: int(time.Now().Unix() % 1000000), // Generate unique ID
	}

	// Set EVSE ID if connector is specified
	if connectorId > 0 {
		request.EvseId = &connectorId
	}

	return request, nil
}

// OnRequestStopTransaction creates a RequestStopTransaction request for OCPP 2.0.1
func (h *V201Handlers) OnRequestStopTransaction(chargePointId string, payload string) (ocpp.Request, error) {
	h.logger.FeatureEvent("RequestStopTransaction", chargePointId, fmt.Sprintf("v2.0.1: transactionId=%s", payload))

	// In OCPP 2.0.1, we need the transaction ID (string, not integer)
	transactionId := payload

	// If payload is empty, try to find active transaction
	if transactionId == "" {
		h.systemHandler.mux.Lock()
		state, ok := h.systemHandler.getChargePoint(chargePointId)
		if ok && len(state.transactions) > 0 {
			// Get first active transaction ID
			for txId := range state.transactions {
				transactionId = fmt.Sprintf("%d", txId)
				break
			}
		}
		h.systemHandler.mux.Unlock()
	}

	if transactionId == "" {
		return nil, fmt.Errorf("no transaction ID provided and no active transaction found")
	}

	request := &remotecontrol.RequestStopTransactionRequest{
		TransactionId: transactionId,
	}

	return request, nil
}

// OnReset creates a Reset request for OCPP 2.0.1
func (h *V201Handlers) OnReset(chargePointId string, payload string) (ocpp.Request, error) {
	h.logger.FeatureEvent("Reset", chargePointId, fmt.Sprintf("v2.0.1: type=%s", payload))

	resetType := provisioning.ResetTypeImmediate
	if payload == "Soft" || payload == "OnIdle" {
		resetType = provisioning.ResetTypeOnIdle
	}

	request := &provisioning.ResetRequest{
		Type: resetType,
	}

	return request, nil
}

// OnGetVariables creates a GetVariables request for OCPP 2.0.1
func (h *V201Handlers) OnGetVariables(chargePointId string, payload string) (ocpp.Request, error) {
	h.logger.FeatureEvent("GetVariables", chargePointId, fmt.Sprintf("v2.0.1: payload=%s", payload))

	// Parse payload as component.variable format or JSON
	var getVariableData []provisioning.GetVariableDataType

	// Try to parse as JSON array first
	if len(payload) > 0 && payload[0] == '[' {
		if err := json.Unmarshal([]byte(payload), &getVariableData); err != nil {
			return nil, fmt.Errorf("failed to parse GetVariables payload: %w", err)
		}
	} else {
		// Parse as simple "Component.Variable" format
		getVariableData = []provisioning.GetVariableDataType{
			{
				Component: v201.Component{Name: "OCPPCommCtrlr"},
				Variable:  v201.Variable{Name: payload},
			},
		}
	}

	request := &provisioning.GetVariablesRequest{
		GetVariableData: getVariableData,
	}

	return request, nil
}

// OnSetVariables creates a SetVariables request for OCPP 2.0.1
func (h *V201Handlers) OnSetVariables(chargePointId string, payload string) (ocpp.Request, error) {
	h.logger.FeatureEvent("SetVariables", chargePointId, fmt.Sprintf("v2.0.1: payload=%s", payload))

	// Parse payload as JSON array
	var setVariableData []provisioning.SetVariableDataType

	if err := json.Unmarshal([]byte(payload), &setVariableData); err != nil {
		return nil, fmt.Errorf("failed to parse SetVariables payload: %w", err)
	}

	request := &provisioning.SetVariablesRequest{
		SetVariableData: setVariableData,
	}

	return request, nil
}

// OnTriggerMessage creates a TriggerMessage request for OCPP 2.0.1
func (h *V201Handlers) OnTriggerMessage(chargePointId string, payload string) (ocpp.Request, error) {
	h.logger.FeatureEvent("TriggerMessage", chargePointId, fmt.Sprintf("v2.0.1: message=%s", payload))

	// TriggerMessage for OCPP 2.0.1 requires a different structure
	// For now, return an error as TriggerMessage is not yet fully implemented
	return nil, fmt.Errorf("TriggerMessage not yet implemented for OCPP 2.0.1")
}
