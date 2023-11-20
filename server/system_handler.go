package server

import (
	"evsys/internal"
	"evsys/models"
	"evsys/ocpp/core"
	"evsys/ocpp/firmware"
	"evsys/ocpp/localauth"
	"evsys/ocpp/remotetrigger"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var newTransactionId = 0

const defaultHeartbeatInterval = 600

type ChargePointState struct {
	status            core.ChargePointStatus
	diagnosticsStatus firmware.DiagnosticsStatus
	firmwareStatus    firmware.Status
	connectors        map[int]*models.Connector // No assumptions about the # of connectors
	transactions      map[int]*models.Transaction
	errorCode         core.ChargePointErrorCode
	model             models.ChargePoint
}

type SystemHandler struct {
	chargePoints   map[string]ChargePointState
	database       internal.Database
	billing        internal.BillingService
	payment        internal.PaymentService
	logger         internal.LogHandler
	eventListeners []internal.EventHandler
	trigger        *Trigger
	debug          bool
	acceptTags     bool
	acceptPoints   bool
	location       *time.Location
	mux            *sync.Mutex
}

func NewSystemHandler(location *time.Location) *SystemHandler {
	handler := &SystemHandler{
		chargePoints:   make(map[string]ChargePointState),
		eventListeners: make([]internal.EventHandler, 0),
		location:       location,
		mux:            &sync.Mutex{},
	}
	return handler
}

func (h *SystemHandler) getTime() time.Time {
	t := time.Now().In(h.location)
	return t.Truncate(time.Second)
}

func (h *SystemHandler) SetDatabase(database internal.Database) {
	h.database = database
}

func (h *SystemHandler) SetBillingService(billing internal.BillingService) {
	h.billing = billing
}

func (h *SystemHandler) SetPaymentService(payment internal.PaymentService) {
	h.payment = payment
}

func (h *SystemHandler) SetParameters(debug bool, acceptTags bool, acceptPoints bool) {
	h.debug = debug
	h.acceptTags = acceptTags
	h.acceptPoints = acceptPoints
}

func (h *SystemHandler) SetLogger(logger internal.LogHandler) {
	h.logger = logger
}

func (h *SystemHandler) AddEventListener(eventListener internal.EventHandler) {
	h.eventListeners = append(h.eventListeners, eventListener)
}

func (h *SystemHandler) SetTrigger(trigger *Trigger) {
	h.trigger = trigger
}

// common function for event listeners
func (h *SystemHandler) notifyEventListeners(event internal.Event, eventData *internal.EventMessage) {
	for _, listener := range h.eventListeners {
		switch event {
		case internal.StatusNotification:
			listener.OnStatusNotification(eventData)
		case internal.TransactionStart:
			listener.OnTransactionStart(eventData)
		case internal.TransactionStop:
			listener.OnTransactionStop(eventData)
		case internal.Authorize:
			listener.OnAuthorize(eventData)
		case internal.TransactionEvent:
			listener.OnTransactionEvent(eventData)
		case internal.Alert:
			listener.OnAlert(eventData)
		case internal.Information:
			listener.OnInfo(eventData)
		}
	}
}

func (h *SystemHandler) OnStart() error {
	if h.database != nil {

		// load charge points from database
		chargePoints, err := h.database.GetChargePoints()
		if err != nil {
			h.notifyEventListeners(internal.Information, &internal.EventMessage{
				Info: fmt.Sprintf("Start failed; load charge points from database: %s", err),
			})
			return fmt.Errorf("failed to load charge points from database: %s", err)
		}

		// load connectors from database
		connectors, err := h.database.GetConnectors()
		if err != nil {
			h.notifyEventListeners(internal.Information, &internal.EventMessage{
				Info: fmt.Sprintf("Start failed; load connectors from database: %s", err),
			})
			return fmt.Errorf("failed to load connectors from database: %s", err)
		}

		for _, cp := range chargePoints {
			state := ChargePointState{
				connectors:   make(map[int]*models.Connector),
				transactions: make(map[int]*models.Transaction),
				model:        cp,
			}
			state.status = core.GetStatus(cp.Status)
			state.errorCode = core.GetErrorCode(cp.ErrorCode)
			if !cp.IsEnabled {
				state.status = core.ChargePointStatusUnavailable
			}
			for _, c := range connectors {
				if c.ChargePointId == cp.Id {
					c.Init()
					state.connectors[c.Id] = c
				}
			}
			h.chargePoints[cp.Id] = state
		}
		h.logger.Debug(fmt.Sprintf("loaded %d charge points, %d connectors from database", len(chargePoints), len(connectors)))

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

	if h.trigger == nil {
		return fmt.Errorf("trigger is not set")
	}
	h.trigger.Start()
	// registering all connectors with active transactions
	for _, cp := range h.chargePoints {
		for _, c := range cp.connectors {
			if c.CurrentTransactionId != -1 {
				h.trigger.Register <- c
			}
		}
	}

	go h.checkAndFinishTransactions()

	go h.notifyEventListeners(internal.Information, &internal.EventMessage{
		Info: "Central system started",
	})

	return nil
}

/**
 * Add a new charge point to the system and database
 */
func (h *SystemHandler) addChargePoint(chargePointId string) {
	if chargePointId == "" {
		h.logger.Warn("invalid charge point id")
		return
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	cp := models.ChargePoint{
		Id:        chargePointId,
		IsEnabled: true,
		Status:    string(core.ChargePointStatusAvailable),
		ErrorCode: string(core.NoError),
	}
	if h.database != nil {
		err := h.database.AddChargePoint(&cp)
		if err != nil {
			h.logger.Error("failed to add charge point to database: %s", err)
		}
	}
	h.chargePoints[chargePointId] = ChargePointState{
		connectors:   make(map[int]*models.Connector),
		transactions: make(map[int]*models.Transaction),
		model:        cp,
	}
}

func (h *SystemHandler) getConnector(cps *ChargePointState, id int) *models.Connector {
	connector, ok := cps.connectors[id]
	if !ok {
		h.mux.Lock()
		defer h.mux.Unlock()
		connector = models.NewConnector(id, cps.model.Id)
		cps.connectors[id] = connector
		if h.database != nil {
			err := h.database.AddConnector(connector)
			if err != nil {
				h.logger.Error("failed to add connector to database", err)
			}
		}
	}
	return connector
}

// select charge point
func (h *SystemHandler) getChargePoint(chargePointId string) (*ChargePointState, bool) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		h.logger.Warn(fmt.Sprintf("unknown charging point: %s", chargePointId))
		if h.acceptPoints {
			h.addChargePoint(chargePointId)
			state, ok = h.chargePoints[chargePointId]
		}
	}
	return &state, ok
}

func (h *SystemHandler) OnBootNotification(chargePointId string, request *core.BootNotificationRequest) (*core.BootNotificationResponse, error) {
	regStatus := core.RegistrationStatusAccepted
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		if h.database != nil {
			h.mux.Lock()
			defer h.mux.Unlock()
			if state.model.SerialNumber != request.ChargePointSerialNumber || state.model.FirmwareVersion != request.FirmwareVersion {
				state.model.SerialNumber = request.ChargePointSerialNumber
				state.model.FirmwareVersion = request.FirmwareVersion
				state.model.Model = request.ChargePointModel
				state.model.Vendor = request.ChargePointVendor
				err := h.database.UpdateChargePoint(&state.model)
				if err != nil {
					h.logger.Error("update charge point", err)
				}
			}
		}
	} else {
		regStatus = core.RegistrationStatusRejected
		h.logger.Debug(fmt.Sprintf("charge point %s not registered", chargePointId))
	}

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, string(regStatus))
	return core.NewBootNotificationResponse(types.NewDateTime(h.getTime()), defaultHeartbeatInterval, regStatus), nil
}

func (h *SystemHandler) OnAuthorize(chargePointId string, request *core.AuthorizeRequest) (*core.AuthorizeResponse, error) {
	authStatus := types.AuthorizationStatusAccepted
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		if !state.model.IsEnabled {
			authStatus = types.AuthorizationStatusBlocked
		}
	} else {
		authStatus = types.AuthorizationStatusBlocked
	}
	h.mux.Lock()
	defer h.mux.Unlock()
	username := ""
	info := ""
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
			// add user tag if not found, new tag is enabled if acceptTags mode is on
			if userTag == nil {
				userTag = &models.UserTag{
					IdTag:     id,
					IsEnabled: h.acceptTags,
					Note:      fmt.Sprintf("added at %s", time.Now().Format("2006-01-02 15:04:05")),
				}
				err = h.database.AddUserTag(userTag)
				if err != nil {
					h.logger.Error("failed to add user tag to database", err)
				}
			}
			if userTag.IsEnabled {
				authStatus = types.AuthorizationStatusAccepted
			}
			username = userTag.Username
			info = userTag.Note
		}
	}

	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   0,
		Time:          h.getTime(),
		Username:      username,
		IdTag:         id,
		Status:        string(authStatus),
		Info:          info,
		TransactionId: 0,
		Payload:       request,
	}
	go h.notifyEventListeners(internal.Authorize, eventMessage)

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("id tag: %s; authorization status: %s", id, authStatus))
	return core.NewAuthorizationResponse(types.NewIdTagInfo(authStatus)), nil
}

func (h *SystemHandler) OnHeartbeat(chargePointId string, _ *core.HeartbeatRequest) (*core.HeartbeatResponse, error) {
	_, _ = h.getChargePoint(chargePointId)
	t := h.getTime()
	//h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%v", t))
	return core.NewHeartbeatResponse(types.NewDateTime(t)), nil
}

func (h *SystemHandler) OnStartTransaction(chargePointId string, request *core.StartTransactionRequest) (*core.StartTransactionResponse, error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusBlocked), 0), nil
	}
	connector := h.getConnector(state, request.ConnectorId)
	connector.Lock()
	defer connector.Unlock()
	if connector.CurrentTransactionId >= 0 {
		h.logger.Error("connector is busy", fmt.Errorf("%s@%d is now busy with transaction %d", chargePointId, request.ConnectorId, connector.CurrentTransactionId))
		eventMessage := &internal.EventMessage{
			ChargePointId: chargePointId,
			ConnectorId:   connector.Id,
			Time:          time.Now(),
			Username:      "",
			IdTag:         request.IdTag,
			Status:        connector.Status,
			TransactionId: connector.CurrentTransactionId,
			Info:          "New transaction was requested, but connector is busy with another transaction.",
			Payload:       request,
		}
		go h.notifyEventListeners(internal.Alert, eventMessage)
		return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusConcurrentTx), connector.CurrentTransactionId), nil
	}

	transaction := &models.Transaction{
		IdTag:         request.IdTag,
		IsFinished:    false,
		ConnectorId:   request.ConnectorId,
		ChargePointId: chargePointId,
		MeterStart:    request.MeterStart,
		TimeStart:     request.Timestamp.Time,
		ReservationId: request.ReservationId,
		Id:            newTransactionId,
	}
	newTransactionId += 1

	connector.CurrentTransactionId = transaction.Id
	state.transactions[transaction.Id] = transaction

	if h.database != nil {
		err := h.database.UpdateConnector(connector)
		if err != nil {
			h.logger.Error("update connector", err)
		}
		idTag, err := h.database.GetUserTag(transaction.IdTag)
		if err != nil {
			h.logger.Error(fmt.Sprintf("get user tag %s", transaction.IdTag), err)
		} else {
			transaction.IdTagNote = idTag.Note
			transaction.Username = idTag.Username
		}
		if h.billing != nil {
			err = h.billing.OnTransactionStart(transaction)
			if err != nil {
				eventMessage := &internal.EventMessage{
					ChargePointId: chargePointId,
					ConnectorId:   transaction.ConnectorId,
					Username:      transaction.Username,
					IdTag:         transaction.IdTag,
					Info:          fmt.Sprintf("billing failed on transaction start; %v", err),
					Payload:       request,
				}
				go h.notifyEventListeners(internal.Alert, eventMessage)
			}
		}
		err = h.database.AddTransaction(transaction)
		if err != nil {
			h.logger.Error("add transaction", err)
		}
	}

	h.trigger.Register <- connector

	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   transaction.ConnectorId,
		Time:          transaction.TimeStart,
		Username:      transaction.Username,
		IdTag:         transaction.IdTag,
		Status:        connector.Status,
		TransactionId: transaction.Id,
		Info:          transaction.IdTagNote,
		Payload:       request,
	}
	go h.notifyEventListeners(internal.TransactionStart, eventMessage)

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("started transaction #%v for connector %v", transaction.Id, transaction.ConnectorId))
	return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted), transaction.Id), nil
}

func (h *SystemHandler) OnStopTransaction(chargePointId string, request *core.StopTransactionRequest) (*core.StopTransactionResponse, error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewStopTransactionResponse(), nil
	}

	// stop requests for meter values
	h.trigger.Unregister <- request.TransactionId

	var err error
	transaction, ok := state.transactions[request.TransactionId]
	if !ok && h.database != nil {
		transaction, err = h.database.GetTransaction(request.TransactionId)
		if err != nil {
			h.logger.Error("get transaction", err)
		}
		ok = err == nil
	}
	if !ok {
		h.logger.Warn(fmt.Sprintf("transaction #%v not found", request.TransactionId))
		return core.NewStopTransactionResponse(), nil
	}

	transaction.Init()
	transaction.Lock()
	defer transaction.Unlock()

	connector := h.getConnector(state, transaction.ConnectorId)
	connector.Lock()
	defer connector.Unlock()

	connector.CurrentTransactionId = -1
	err = h.database.UpdateConnector(connector)
	if err != nil {
		h.logger.Error("update connector", err)
	}
	if transaction.IsFinished {
		h.logger.Warn(fmt.Sprintf("transaction #%v is already finished", request.TransactionId))
		return core.NewStopTransactionResponse(), nil
	}

	transaction.IsFinished = true
	transaction.TimeStop = request.Timestamp.Time
	transaction.MeterStop = request.MeterStop
	transaction.Reason = string(request.Reason)

	// request data may contain meter values of begin and end of transaction
	if request.TransactionData != nil {
		for _, data := range request.TransactionData {
			if data.SampledValue != nil {
				for _, value := range data.SampledValue {
					if value.Context == types.ReadingContextTransactionBegin {
						transaction.MeterStart = utility.ToInt(value.Value)
						transaction.TimeStart = data.Timestamp.Time
					}
					if value.Context == types.ReadingContextTransactionEnd {
						transaction.MeterStop = utility.ToInt(value.Value)
						transaction.TimeStop = data.Timestamp.Time
					}
				}
			}
		}
	}

	err = h.billing.OnTransactionFinished(transaction)
	if err != nil {
		eventMessage := &internal.EventMessage{
			ChargePointId: chargePointId,
			ConnectorId:   transaction.ConnectorId,
			Username:      transaction.Username,
			IdTag:         transaction.IdTag,
			Info:          fmt.Sprintf("billing failed %v", err),
			Payload:       request,
		}
		go h.notifyEventListeners(internal.Alert, eventMessage)
	}

	if h.database != nil {
		err = h.database.UpdateTransaction(transaction)
		if err != nil {
			h.logger.Error("update transaction", err)
		} else {
			err = h.database.DeleteTransactionMeterValues(transaction.Id)
			if err != nil {
				h.logger.Error("delete transaction meter values", err)
			}
		}
	}

	if h.payment != nil {
		go h.payment.TransactionPayment(transaction)
	}

	consumed := utility.IntToString(transaction.MeterStop - transaction.MeterStart)
	price := utility.IntAsPrice(transaction.PaymentAmount)
	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   transaction.ConnectorId,
		Time:          transaction.TimeStart,
		Username:      transaction.Username,
		IdTag:         transaction.IdTag,
		Status:        connector.Status,
		TransactionId: transaction.Id,
		Info:          fmt.Sprintf("consumed %s kW; %v €", consumed, price),
		Payload:       request,
	}
	go h.notifyEventListeners(internal.TransactionStop, eventMessage)

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("stopped transaction %v %v", request.TransactionId, request.Reason))
	return core.NewStopTransactionResponse(), nil
}

func (h *SystemHandler) OnMeterValues(chargePointId string, request *core.MeterValuesRequest) (*core.MeterValuesResponse, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewMeterValuesResponse(), nil
	}

	transactionId := request.TransactionId
	if transactionId != nil && h.database != nil {

		transaction, err := h.database.GetTransaction(*transactionId)

		currentValue := 0
		if err != nil {
			h.logger.Error("get transaction failed", err)
		} else {

			for _, sampledValue := range request.MeterValue {
				for _, value := range sampledValue.SampledValue {
					// read value of active energy import register only for triggered messages
					if value.Context == types.ReadingContextTrigger && value.Measurand == types.MeasurandEnergyActiveImportRegister {

						currentValue = utility.ToInt(value.Value)

						transactionMeter := &models.TransactionMeter{
							Id:        transaction.Id,
							Value:     currentValue,
							Time:      time.Now(),
							Unit:      string(value.Unit),
							Measurand: string(value.Measurand),
						}

						err = h.billing.OnMeterValue(transaction, transactionMeter)
						if err != nil {
							eventMessage := &internal.EventMessage{
								ChargePointId: chargePointId,
								ConnectorId:   transaction.ConnectorId,
								Username:      transaction.Username,
								IdTag:         transaction.IdTag,
								Info:          fmt.Sprintf("billing failed %v", err),
								Payload:       request,
							}
							go h.notifyEventListeners(internal.Alert, eventMessage)
						}

						err = h.database.AddTransactionMeterValue(transactionMeter)
						if err != nil {
							h.logger.Error("add transaction meter value", err)
						}
					}
				}
			}
		}

	}
	return core.NewMeterValuesResponse(), nil
}

func (h *SystemHandler) OnStatusNotification(chargePointId string, request *core.StatusNotificationRequest) (*core.StatusNotificationResponse, error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewStatusNotificationResponse(), nil
	}
	currentTransactionId := 0
	state.errorCode = request.ErrorCode
	if request.ConnectorId > 0 {
		connector := h.getConnector(state, request.ConnectorId)
		connector.Lock()
		defer connector.Unlock()
		connector.Status = string(request.Status)
		connector.StatusTime = request.Timestamp.Time
		connector.Info = request.Info
		connector.VendorId = request.VendorId
		connector.ErrorCode = string(request.ErrorCode)
		if request.Status == core.ChargePointStatusAvailable {
			connector.CurrentTransactionId = -1
		}
		if h.database != nil {
			err := h.database.UpdateConnector(connector)
			if err != nil {
				h.logger.Error("update status", err)
			}
		}
		currentTransactionId = connector.CurrentTransactionId
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated connector #%v status to %v", request.ConnectorId, request.Status))
	} else {
		h.mux.Lock()
		defer h.mux.Unlock()
		state.status = request.Status
		state.model.Status = string(request.Status)
		state.model.StatusTime = request.Timestamp.Time
		state.model.Info = request.Info
		if h.database != nil {
			err := h.database.UpdateChargePointStatus(&state.model)
			if err != nil {
				h.logger.Error("update status", err)
			}
		}
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated main controller status to %v", request.Status))
	}

	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   request.ConnectorId,
		Time:          h.getTime(),
		Username:      "",
		IdTag:         "",
		Status:        string(request.Status),
		TransactionId: currentTransactionId,
		Info:          request.Info,
		Payload:       request,
	}
	go h.notifyEventListeners(internal.StatusNotification, eventMessage)

	if request.ConnectorId > 0 && request.Status == core.ChargePointStatusAvailable {
		go h.checkAndFinishTransactions()
	}

	return core.NewStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnDataTransfer(chargePointId string, request *core.DataTransferRequest) (*core.DataTransferResponse, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewDataTransferResponse(core.DataTransferStatusRejected), nil
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("recieved data #%v", request.Data))
	return core.NewDataTransferResponse(core.DataTransferStatusAccepted), nil
}

func (h *SystemHandler) OnDiagnosticsStatusNotification(chargePointId string, request *firmware.DiagnosticsStatusNotificationRequest) (*firmware.DiagnosticsStatusNotificationResponse, error) {
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		state.diagnosticsStatus = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated diagnostic status to %v", request.Status))
	}
	return firmware.NewDiagnosticsStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnFirmwareStatusNotification(chargePointId string, request *firmware.StatusNotificationRequest) (*firmware.StatusNotificationResponse, error) {
	state, ok := h.getChargePoint(chargePointId)
	if ok {
		state.firmwareStatus = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated firmware status to %v", request.Status))
	}
	return firmware.NewStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnTriggerMessage(chargePointId string, connectorId int, message string) (*remotetrigger.TriggerMessageRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	request := remotetrigger.NewTriggerMessageRequest(remotetrigger.MessageTrigger(message), connectorId)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("message: %v", message))
	return request, nil
}

func (h *SystemHandler) OnSendLocalList(chargePointId string) (*localauth.SendLocalListRequest, error) {
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point %s not found", chargePointId)
	}
	version := state.model.LocalAuthVersion + 1
	request := localauth.NewSendLocalListRequest(version, localauth.UpdateTypeFull)
	authList := make([]localauth.AuthorizationData, 0)
	if h.database != nil {
		ids, err := h.database.GetActiveUserTags(chargePointId, version)
		if err != nil {
			h.logger.Error("get active user tags", err)
		} else {
			for _, id := range ids {
				authList = append(authList, localauth.AuthorizationData{
					IdTag: id.IdTag,
					IdTagInfo: &types.IdTagInfo{
						//TODO: add expiry date
						Status: types.AuthorizationStatusAccepted,
					},
				})
			}
		}
	}
	request.LocalAuthorizationList = authList
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("sending local auth list #%v", version))
	return request, nil
}

func (h *SystemHandler) OnRemoteStartTransaction(chargePointId string, connectorId int, idTag string) (*core.RemoteStartTransactionRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	request := core.NewRemoteStartTransactionRequest(idTag)
	if connectorId > 0 {
		request.ConnectorId = &connectorId
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("remote start transaction on connector: %v; for id: %s", connectorId, idTag))
	return request, nil
}

func (h *SystemHandler) OnRemoteStopTransaction(chargePointId string, id string) (*core.RemoteStopTransactionRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	transactionId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction id")
	}
	request := core.NewRemoteStopTransactionRequest(int(transactionId))
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("remote stop transaction: %v", transactionId))
	return request, nil
}

func (h *SystemHandler) OnOnlineStatusChanged(id string, isOnline bool) {
	h.mux.Lock()
	defer h.mux.Unlock()
	chp, err := h.database.GetChargePoint(id)
	if chp != nil {
		// don't send event and update database if status is not changed and charge point is offline
		if !isOnline && !chp.IsOnline {
			return
		}
		if chp.IsOnline != isOnline {
			info := fmt.Sprintf("comes online; was offline %v", utility.TimeAgo(chp.EventTime))
			if !isOnline {
				info = "goes OFFLINE"
			}
			eventMessage := &internal.EventMessage{
				ChargePointId: id,
				ConnectorId:   0,
				Time:          h.getTime(),
				Info:          info,
			}
			go h.notifyEventListeners(internal.Alert, eventMessage)
		}
	}
	err = h.database.UpdateOnlineStatus(id, isOnline)
	if err != nil {
		h.logger.Error("update online status", err)
	}
}

func (h *SystemHandler) checkAndFinishTransactions() {
	if h.database == nil {
		return
	}
	//TODO add meter values for unfinished transactions

	transactions, err := h.database.GetUnfinishedTransactions()
	if err != nil {
		h.logger.Error("get unfinished transactions", err)
		return
	}
	for _, transaction := range transactions {
		h.logger.Warn(fmt.Sprintf("transaction #%v was not finished correctly", transaction.Id))
		h.trigger.Unregister <- transaction.ConnectorId

		transaction.Init()
		transaction.Lock()
		transaction.IsFinished = true
		transaction.TimeStop = h.getTime()
		transaction.Reason = "stopped by system"

		meterValue, err := h.database.ReadTransactionMeterValue(transaction.Id)
		if meterValue != nil {
			transaction.MeterStop = meterValue.Value
			transaction.TimeStop = meterValue.Time
		}

		err = h.billing.OnTransactionFinished(transaction)
		if err != nil {
			eventMessage := &internal.EventMessage{
				ChargePointId: transaction.ChargePointId,
				ConnectorId:   transaction.ConnectorId,
				TransactionId: transaction.Id,
				Username:      transaction.Username,
				IdTag:         transaction.IdTag,
				Info:          fmt.Sprintf("billing failed %v", err),
			}
			go h.notifyEventListeners(internal.Alert, eventMessage)
		}

		err = h.database.UpdateTransaction(transaction)
		if err != nil {
			h.logger.Error("update transaction", err)
		}
		transaction.Unlock()

		err = h.database.DeleteTransactionMeterValues(transaction.Id)
		if err != nil {
			h.logger.Error("delete transaction meter values", err)
		}

		if h.payment != nil {
			go h.payment.TransactionPayment(transaction)
		}

		eventMessage := &internal.EventMessage{
			ChargePointId: transaction.ChargePointId,
			ConnectorId:   transaction.ConnectorId,
			TransactionId: transaction.Id,
			Username:      transaction.Username,
			Time:          h.getTime(),
			Info:          "transaction was stopped by system",
		}
		go h.notifyEventListeners(internal.Alert, eventMessage)
	}
}
