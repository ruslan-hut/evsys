package core

import (
	"evsys/internal"
	"evsys/models"
	"evsys/ocpp/firmware"
	"evsys/types"
	"fmt"
	"time"
)

var newTransactionId = 0

const defaultHeartbeatInterval = 600

type TransactionInfo struct {
	id          int
	startTime   *types.DateTime
	endTime     *types.DateTime
	startMeter  int
	endMeter    int
	connectorId int
	idTag       string
}

type ConnectorInfo struct {
	status             ChargePointStatus
	currentTransaction int
	model              *models.Connector
}

type ChargePointState struct {
	status            ChargePointStatus
	diagnosticsStatus firmware.DiagnosticsStatus
	firmwareStatus    firmware.Status
	connectors        map[int]*ConnectorInfo // No assumptions about the # of connectors
	transactions      map[int]*models.Transaction
	errorCode         ChargePointErrorCode
	model             *models.ChargePoint
}

type SystemHandler struct {
	chargePoints map[string]*ChargePointState
	database     internal.Database
	logger       internal.LogHandler
	debug        bool
}

func NewSystemHandler() *SystemHandler {
	handler := SystemHandler{
		chargePoints: make(map[string]*ChargePointState),
	}
	return &handler
}

func (h *SystemHandler) SetDatabase(database internal.Database) {
	h.database = database
}

// SetDebugMode setting debug mode, used for registering unknown charge points
func (h *SystemHandler) SetDebugMode(debug bool) {
	h.debug = debug
}

func (h *SystemHandler) SetLogger(logger internal.LogHandler) {
	h.logger = logger
}

func (h *SystemHandler) OnStart() error {
	if h.database != nil {

		// load charge points from database
		chargePoints, err := h.database.GetChargePoints()
		if err != nil {
			return fmt.Errorf("failed to load charge points from database: %s", err)
		}

		// load connectors from database
		connectors, err := h.database.GetConnectors()
		if err != nil {
			return fmt.Errorf("failed to load connectors from database: %s", err)
		}

		for _, cp := range chargePoints {
			h.addChargePoint(cp.Id, &cp)
			state, _ := h.chargePoints[cp.Id]
			state.status = GetStatus(cp.Status)
			state.errorCode = GetErrorCode(cp.ErrorCode)
			if !cp.IsEnabled {
				state.status = ChargePointStatusUnavailable
			}
			for _, c := range connectors {
				if c.ChargePointId == cp.Id {
					ci := &ConnectorInfo{
						currentTransaction: -1,
						model:              &c,
						status:             GetStatus(c.Status),
					}
					state.connectors[c.Id] = ci
				}
			}
		}

		// load transactions from database
		transaction, err := h.database.GetLastTransaction()
		if err != nil {
			h.logger.Error("failed to load last transaction from database", err)
		}
		if transaction != nil {
			newTransactionId = transaction.Id + 1
		}

		// load firmware status from database
		// load diagnostics status from database
	}
	return nil
}

/**
 * Add a new charge point to the system and database
 */
func (h *SystemHandler) addChargePoint(chargePointId string, model *models.ChargePoint) {
	var cp *models.ChargePoint
	if model == nil {
		cp = &models.ChargePoint{
			Id:        chargePointId,
			IsEnabled: true,
			Status:    string(ChargePointStatusAvailable),
			ErrorCode: string(NoError),
		}
		if h.database != nil {
			err := h.database.AddChargePoint(cp)
			if err != nil {
				h.logger.Error("failed to add charge point to database: %s", err)
			}
		}
	} else {
		cp = model
	}
	h.chargePoints[chargePointId] = &ChargePointState{
		connectors:   make(map[int]*ConnectorInfo),
		transactions: make(map[int]*models.Transaction),
		model:        cp,
	}
}

func (h *SystemHandler) getConnector(cps *ChargePointState, id int) *ConnectorInfo {
	ci, ok := cps.connectors[id]
	if !ok {
		co := &models.Connector{
			Id:            id,
			ChargePointId: cps.model.Id,
			IsEnabled:     true,
		}
		ci = &ConnectorInfo{
			currentTransaction: -1,
			model:              co,
		}
		cps.connectors[id] = ci
		if h.database != nil {
			err := h.database.AddConnector(co)
			if err != nil {
				h.logger.Error("failed to add connector to database", err)
			}
		}
	}
	return ci
}

// select charge point
func (h *SystemHandler) getChargePoint(chargePointId string) (*ChargePointState, bool) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		h.logger.Warn(fmt.Sprintf("unknown charging point: %s", chargePointId))
		if h.debug {
			h.logger.Debug("registering new charge point in debug mode")
			h.addChargePoint(chargePointId, nil)
			state, ok = h.chargePoints[chargePointId]
		}
	}
	return state, ok
}

func (h *SystemHandler) OnBootNotification(chargePointId string, request *BootNotificationRequest) (confirmation *BootNotificationResponse, err error) {
	regStatus := RegistrationStatusAccepted

	state, ok := h.getChargePoint(chargePointId)
	if ok {
		if h.database != nil {
			if state.model.SerialNumber != request.ChargePointSerialNumber || state.model.FirmwareVersion != request.FirmwareVersion {
				state.model.SerialNumber = request.ChargePointSerialNumber
				state.model.FirmwareVersion = request.FirmwareVersion
				state.model.Model = request.ChargePointModel
				state.model.Vendor = request.ChargePointVendor
				err := h.database.UpdateChargePoint(state.model)
				if err != nil {
					h.logger.Error("update charge point", err)
				}
			}
		}
	} else {
		regStatus = RegistrationStatusRejected
		h.logger.Debug(fmt.Sprintf("charge point %s not registered", chargePointId))
	}

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, string(regStatus))
	return NewBootNotificationResponse(types.NewDateTime(time.Now()), defaultHeartbeatInterval, regStatus), nil
}

func (h *SystemHandler) OnAuthorize(chargePointId string, request *AuthorizeRequest) (confirmation *AuthorizeResponse, err error) {
	authStatus := types.AuthorizationStatusAccepted
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		if !state.model.IsEnabled {
			authStatus = types.AuthorizationStatusBlocked
		}
	} else {
		authStatus = types.AuthorizationStatusBlocked
	}
	id := request.IdTag
	if id == "" {
		authStatus = types.AuthorizationStatusInvalid
	} else {
		// auth logic with database
		if h.database != nil && authStatus == types.AuthorizationStatusAccepted {
			// status will be changed if user tag is found and enabled
			authStatus = types.AuthorizationStatusBlocked
			userTag, err := h.database.GetUserTag(id)
			if err != nil {
				h.logger.Error("failed to get user tag from database", err)
			}
			// add user tag if not found, new tag is enabled if debug mode is on
			if userTag == nil {
				userTag = &models.UserTag{
					IdTag:     id,
					IsEnabled: h.debug,
				}
				err = h.database.AddUserTag(userTag)
				if err != nil {
					h.logger.Error("failed to add user tag to database", err)
				}
			}
			if userTag.IsEnabled {
				authStatus = types.AuthorizationStatusAccepted
			}
		}
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("id tag: %s; authorization status: %s", id, authStatus))
	return NewAuthorizationResponse(types.NewIdTagInfo(authStatus)), nil
}

func (h *SystemHandler) OnHeartbeat(chargePointId string, request *HeartbeatRequest) (confirmation *HeartbeatResponse, err error) {
	_, _ = h.getChargePoint(chargePointId)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%v", time.Now()))
	return NewHeartbeatResponse(types.NewDateTime(time.Now())), nil
}

func (h *SystemHandler) OnStartTransaction(chargePointId string, request *StartTransactionRequest) (confirmation *StartTransactionResponse, err error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusBlocked), 0), nil
	}
	connector := h.getConnector(state, request.ConnectorId)
	if connector.currentTransaction >= 0 {
		h.logger.Warn(fmt.Sprintf("connector %v@%s is now busy with another transaction", request.ConnectorId, chargePointId))
		return NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusConcurrentTx), 0), nil
	}

	transaction := &models.Transaction{}
	transaction.IdTag = request.IdTag
	transaction.ConnectorId = request.ConnectorId
	transaction.ChargePointId = chargePointId
	transaction.MeterStart = request.MeterStart
	transaction.TimeStart = request.Timestamp.Time
	transaction.ReservationId = request.ReservationId
	transaction.Id = newTransactionId
	newTransactionId += 1

	connector.currentTransaction = transaction.Id
	state.transactions[transaction.Id] = transaction

	if h.database != nil {
		err := h.database.AddTransaction(transaction)
		if err != nil {
			h.logger.Error("add transaction", err)
		}
	}

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("started transaction #%v for connector %v", transaction.Id, transaction.ConnectorId))
	return NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted), transaction.Id), nil
}

func (h *SystemHandler) OnStopTransaction(chargePointId string, request *StopTransactionRequest) (confirmation *StopTransactionResponse, err error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return NewStopTransactionResponse(), nil
	}
	transaction, ok := state.transactions[request.TransactionId]
	if !ok && h.database != nil {
		transaction, err = h.database.GetTransaction(request.TransactionId)
		if err != nil {
			h.logger.Error("get transaction", err)
		}
		ok = transaction != nil
	}
	if ok {
		connector := h.getConnector(state, transaction.ConnectorId)
		connector.currentTransaction = -1
		transaction.ConnectorId = connector.model.Id
		transaction.IdTag = request.IdTag
		transaction.ChargePointId = chargePointId
		transaction.TimeStop = request.Timestamp.Time
		transaction.MeterStop = request.MeterStop
		transaction.Reason = string(request.Reason)
		//TODO: bill clients
		if h.database != nil {
			err := h.database.UpdateTransaction(transaction)
			if err != nil {
				h.logger.Error("update transaction", err)
			}
		}
	} else {
		h.logger.Warn(fmt.Sprintf("transaction #%v not found", request.TransactionId))
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("stopped transaction %v %v", request.TransactionId, request.Reason))
	return NewStopTransactionResponse(), nil
}

func (h *SystemHandler) OnMeterValues(chargePointId string, request *MeterValuesRequest) (confirmation *MeterValuesResponse, err error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return NewMeterValuesResponse(), nil
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("recieved meter values for connector #%v", request.ConnectorId))
	for _, value := range request.MeterValue {
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%v --> %v", request.ConnectorId, value))
	}
	return NewMeterValuesResponse(), nil
}

func (h *SystemHandler) OnStatusNotification(chargePointId string, request *StatusNotificationRequest) (confirmation *StatusNotificationResponse, err error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return NewStatusNotificationResponse(), nil
	}
	state.errorCode = request.ErrorCode
	if request.ConnectorId > 0 {
		connector := h.getConnector(state, request.ConnectorId)
		connector.status = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated connector #%v status to %v", request.ConnectorId, request.Status))
		connector.model.Status = string(request.Status)
		connector.model.Info = request.Info
		connector.model.VendorId = request.VendorId
		connector.model.ErrorCode = string(request.ErrorCode)
		if h.database != nil {
			err = h.database.UpdateConnector(connector.model)
			if err != nil {
				h.logger.Error("update status", err)
			}
		}
	} else {
		state.status = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated main controller status to %v", request.Status))
		state.model.Status = string(request.Status)
		state.model.Info = request.Info
		if h.database != nil {
			err = h.database.UpdateChargePoint(state.model)
			if err != nil {
				h.logger.Error("update status", err)
			}
		}
	}
	return NewStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnDataTransfer(chargePointId string, request *DataTransferRequest) (confirmation *DataTransferResponse, err error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return NewDataTransferResponse(DataTransferStatusRejected), nil
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("recieved data #%v", request.Data))
	return NewDataTransferResponse(DataTransferStatusAccepted), nil
}

func (h *SystemHandler) OnDiagnosticsStatusNotification(chargePointId string, request *firmware.DiagnosticsStatusNotificationRequest) (confirmation *firmware.DiagnosticsStatusNotificationResponse, err error) {
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		state.diagnosticsStatus = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated diagnostic status to %v", request.Status))
	}
	return firmware.NewDiagnosticsStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnFirmwareStatusNotification(chargePointId string, request *firmware.StatusNotificationRequest) (confirmation *firmware.StatusNotificationResponse, err error) {
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		state.firmwareStatus = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated firmware status to %v", request.Status))
	}
	return firmware.NewStatusNotificationResponse(), nil
}
