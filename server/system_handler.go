package server

import (
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"evsys/metrics/counters"
	"evsys/ocpp/core"
	"evsys/ocpp/firmware"
	"evsys/ocpp/localauth"
	"evsys/ocpp/remotetrigger"
	"evsys/ocpp/smartcharging"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var newTransactionId = 0

const (
	defaultHeartbeatInterval = 600
	sourceOCPI               = "OCPI"
)

type BillingService interface {
	OnTransactionStart(transaction *entity.Transaction) error
	OnTransactionFinished(transaction *entity.Transaction) error
	OnMeterValue(transaction *entity.Transaction, transactionMeter *entity.TransactionMeter) error
}

type PaymentService interface {
	TransactionPayment(transaction *entity.Transaction)
}

type ErrorListener interface {
	OnError(data *entity.ErrorData)
}

type ChargePointState struct {
	status            core.ChargePointStatus
	diagnosticsStatus firmware.DiagnosticsStatus
	firmwareStatus    firmware.Status
	connectors        map[int]*entity.Connector // No assumptions about the # of connectors
	transactions      map[int]*int
	errorCode         core.ChargePointErrorCode
	model             *entity.ChargePoint
}

func newChargePointState(chp *entity.ChargePoint) *ChargePointState {
	return &ChargePointState{
		connectors:   make(map[int]*entity.Connector),
		transactions: make(map[int]*int),
		model:        chp,
	}
}

func (st *ChargePointState) registerTransaction(transactionId int) {
	st.transactions[transactionId] = &transactionId
}

func (st *ChargePointState) unregisterTransaction(transactionId int) {
	delete(st.transactions, transactionId)
}

func (st *ChargePointState) EvseId(connectorId int) string {
	if st.model == nil {
		return ""
	}
	return st.model.EvseId(connectorId)
}

type authResult struct {
	allowed bool
	expired bool
	blocked bool
	info    string
}

type AuthService interface {
	Authorize(locationId, evseId, idTag string) (bool, bool, bool, string)
}

type SystemHandler struct {
	chargePoints   map[string]*ChargePointState
	lastMeter      map[int]*entity.TransactionMeter
	database       internal.Database
	billing        BillingService
	payment        PaymentService
	auth           AuthService
	errorListener  ErrorListener
	logger         internal.LogHandler
	eventListeners []internal.EventHandler
	trigger        *Trigger
	debug          bool
	acceptTags     bool
	acceptPoints   bool
	location       *time.Location
	mux            sync.Mutex
}

func NewSystemHandler(location *time.Location) *SystemHandler {
	handler := &SystemHandler{
		chargePoints:   make(map[string]*ChargePointState),
		lastMeter:      make(map[int]*entity.TransactionMeter),
		eventListeners: make([]internal.EventHandler, 0),
		location:       location,
		mux:            sync.Mutex{},
	}
	return handler
}

func (h *SystemHandler) updateActiveTransactionsCounter() {
	// calculate transactions per locations
	locations := make(map[string]int)
	for _, cp := range h.chargePoints {
		if cp.model.LocationId != "" {
			locations[cp.model.LocationId] += len(cp.transactions)
		}
	}
	for location, count := range locations {
		counters.ObserveTransactions(location, count)
	}
}

func (h *SystemHandler) getTime() time.Time {
	t := time.Now().In(h.location)
	return t.Truncate(time.Second)
}

func (h *SystemHandler) SetDatabase(database internal.Database) {
	h.database = database
}

func (h *SystemHandler) SetBillingService(billing BillingService) {
	h.billing = billing
}

func (h *SystemHandler) SetPaymentService(payment PaymentService) {
	h.payment = payment
}

func (h *SystemHandler) SetAuthService(auth AuthService) {
	h.auth = auth
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

func (h *SystemHandler) SetErrorListener(listener ErrorListener) {
	h.errorListener = listener
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
	h.mux.Lock()
	defer h.mux.Unlock()

	totalPoints := 0
	totalConnectors := 0

	if h.database != nil {

		err := h.database.ResetOnlineStatus()
		if err != nil {
			h.logger.Error("reset online status", err)
		}

		// load charge points from database
		chargePoints, err := h.database.GetChargePoints()
		if err != nil {
			h.notifyEventListeners(internal.Information, &internal.EventMessage{
				Info: fmt.Sprintf("Start failed; load charge points from database: %s", err),
			})
			return fmt.Errorf("failed to load charge points from database: %s", err)
		}
		totalPoints = len(chargePoints)

		for _, cp := range chargePoints {
			h.initializeChargePointState(cp)
			totalConnectors += len(cp.Connectors)
		}
		h.logger.FeatureEvent("Start", "", fmt.Sprintf("loaded %d charge points, %d connectors from database", totalPoints, totalConnectors))

		// load transactions from database
		transaction, err := h.database.GetLastTransaction()
		if err != nil {
			h.logger.Error("failed to load last transaction from database", err)
		}
		if transaction != nil {
			newTransactionId = transaction.Id + 1
		}

		// load last meter values from database; used to calculate power rate
		meterValues, err := h.database.ReadLastMeterValues()
		if meterValues != nil {
			for _, mv := range meterValues {
				h.lastMeter[mv.Id] = mv
			}
		}

		// load firmware status from database
		// load diagnostics status from database
	}

	h.updateActiveTransactionsCounter()

	if h.trigger == nil {
		return fmt.Errorf("trigger is not set")
	}
	h.trigger.Start()
	// registering all connectors with active transactions
	for _, cp := range h.chargePoints {
		for _, c := range cp.connectors {
			h.checkListenTransaction(c, cp.model.IsOnline)
		}
	}

	go h.checkAndFinishTransactions()

	go h.notifyEventListeners(internal.Information, &internal.EventMessage{
		Info: fmt.Sprintf("Started with %d charge points, %d connectors", totalPoints, totalConnectors),
	})

	return nil
}

func (h *SystemHandler) initializeChargePointState(chp *entity.ChargePoint) *ChargePointState {
	state := newChargePointState(chp)
	state.status = core.GetStatus(chp.Status)
	state.errorCode = core.GetErrorCode(chp.ErrorCode)
	if !chp.IsEnabled {
		state.status = core.ChargePointStatusUnavailable
	}
	if chp.Connectors != nil {
		for _, c := range chp.Connectors {
			c.Init()
			state.connectors[c.Id] = c
			if c.CurrentTransactionId != -1 {
				state.registerTransaction(c.CurrentTransactionId)
			}
		}
	}
	h.chargePoints[chp.Id] = state
	return state
}

/**
 * Add a new charge point to the system and database
 */
func (h *SystemHandler) addChargePoint(chargePointId string) *ChargePointState {
	if chargePointId == "" {
		h.logger.Warn("invalid charge point id")
		return nil
	}
	cp := &entity.ChargePoint{
		Id:          chargePointId,
		Title:       fmt.Sprintf("(new) %s", chargePointId),
		IsEnabled:   true,
		Status:      string(core.ChargePointStatusAvailable),
		ErrorCode:   string(core.NoError),
		AccessType:  "private",
		AccessLevel: 10, // only users with equal or higher level can access this charge point
	}
	if h.database != nil {
		err := h.database.AddChargePoint(cp)
		if err != nil {
			h.logger.Error("failed to add charge point to database: %s", err)
		}
	}
	return h.initializeChargePointState(cp)
}

func (h *SystemHandler) getConnector(cps *ChargePointState, id int) *entity.Connector {
	connector, ok := cps.connectors[id]
	if ok {
		return connector
	}
	// check if connector is in database
	if h.database != nil {
		c, _ := h.database.GetConnector(id, cps.model.Id)
		if c != nil {
			c.Init()
			cps.connectors[id] = c
			return c
		}
	}
	// create new connector
	connector = entity.NewConnector(id, cps.model.Id)
	cps.connectors[id] = connector
	if h.database != nil {
		err := h.database.AddConnector(connector)
		if err != nil {
			h.logger.Error("failed to add connector to database", err)
		}
	}
	return connector
}

// select charge point
func (h *SystemHandler) getChargePoint(chargePointId string) (*ChargePointState, bool) {
	state, ok := h.chargePoints[chargePointId]
	if ok {
		return state, ok
	}

	if h.database != nil {
		chargePoint, _ := h.database.GetChargePoint(chargePointId)
		if chargePoint != nil {
			state = h.initializeChargePointState(chargePoint)
			return state, true
		}
	}

	h.logger.Warn(fmt.Sprintf("unknown charging point: %s", chargePointId))
	if h.acceptPoints {
		state = h.addChargePoint(chargePointId)
	}

	if state == nil {
		return nil, false
	}
	return state, true
}

func (h *SystemHandler) OnBootNotification(chargePointId string, request *core.BootNotificationRequest) (*core.BootNotificationResponse, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	regStatus := core.RegistrationStatusAccepted
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
		regStatus = core.RegistrationStatusRejected
		h.logger.Debug(fmt.Sprintf("charge point %s not registered", chargePointId))
	}

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, string(regStatus))
	return core.NewBootNotificationResponse(types.NewDateTime(h.getTime()), defaultHeartbeatInterval, regStatus), nil
}

func (h *SystemHandler) OnAuthorize(chargePointId string, request *core.AuthorizeRequest) (*core.AuthorizeResponse, error) {
	h.mux.Lock()
	defer h.mux.Unlock()
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("unknown chargepoint; authorization blocked; id:%s", request.IdTag))
		return core.NewAuthorizationResponse(types.NewIdTagInfo(types.AuthorizationStatusBlocked)), nil
	}
	if !state.model.IsEnabled {
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("chargepoint disabled; authorization blocked; id:%s", request.IdTag))
		return core.NewAuthorizationResponse(types.NewIdTagInfo(types.AuthorizationStatusBlocked)), nil
	}

	authStatus := types.AuthorizationStatusInvalid
	userTag := h.getUserTag(request.IdTag)
	if userTag.IsEnabled {
		authStatus = types.AuthorizationStatusAccepted
	}

	// invalid state indicated that user not listed in local database or not enabled locally
	if authStatus == types.AuthorizationStatusInvalid {
		// try to authorize with connected auth service; here goes the OCPI authorization
		// for EVSE id always use connector 1, because authorize request does not have connector id
		result, err := h.authorize(state.model.LocationId, state.model.EvseId(1), userTag.IdTag)
		if err == nil {
			if result.allowed {
				authStatus = types.AuthorizationStatusAccepted
			} else if result.expired {
				authStatus = types.AuthorizationStatusExpired
			} else if result.blocked {
				authStatus = types.AuthorizationStatusBlocked
			}
			// if status was changed, update user tag
			if authStatus != types.AuthorizationStatusInvalid {
				userTag.Source = sourceOCPI
				userTag.Note = result.info
				_ = h.database.UpdateTag(userTag)
			}
		}
	}

	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   0,
		Time:          h.getTime(),
		Username:      userTag.Username,
		IdTag:         fmt.Sprintf("%s %s", userTag.Source, userTag.IdTag),
		Status:        string(authStatus),
		Info:          userTag.Note,
		TransactionId: 0,
		Payload:       request,
	}
	go h.notifyEventListeners(internal.Authorize, eventMessage)

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("id tag: %s %s; status: %s", userTag.Source, userTag.IdTag, authStatus))
	return core.NewAuthorizationResponse(types.NewIdTagInfo(authStatus)), nil
}

func (h *SystemHandler) OnHeartbeat(chargePointId string, _ *core.HeartbeatRequest) (*core.HeartbeatResponse, error) {
	_, _ = h.getChargePoint(chargePointId)
	t := h.getTime()
	//h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%v", t))
	return core.NewHeartbeatResponse(types.NewDateTime(t)), nil
}

func (h *SystemHandler) OnStartTransaction(chargePointId string, request *core.StartTransactionRequest) (*core.StartTransactionResponse, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusBlocked), 0), nil
	}

	connector := h.getConnector(state, request.ConnectorId)
	connector.Lock()
	defer func() {
		connector.Unlock()
		h.checkListenTransaction(connector, state.model.IsOnline)
	}()

	if connector.CurrentTransactionId >= 0 && h.database != nil {

		transaction, _ := h.database.GetTransaction(connector.CurrentTransactionId)
		if transaction != nil {
			if !transaction.IsFinished && transaction.ConnectorId == connector.Id {
				h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("start transaction: connector %d is already started transaction %d", connector.Id, connector.CurrentTransactionId))
				return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted), connector.CurrentTransactionId), nil
			}

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

	}

	userTag := h.getUserTag(request.IdTag)

	transaction := &entity.Transaction{
		IdTag:         userTag.IdTag,
		IdTagNote:     userTag.Note,
		Username:      userTag.Username,
		IsFinished:    false,
		ConnectorId:   request.ConnectorId,
		ChargePointId: chargePointId,
		MeterStart:    request.MeterStart,
		TimeStart:     request.Timestamp.Time,
		ReservationId: request.ReservationId,
		Id:            newTransactionId,
		UserTag:       userTag,
	}
	newTransactionId += 1

	if userTag.Source == sourceOCPI {
		transaction.SessionId = userTag.IdTag
	}

	connector.CurrentTransactionId = transaction.Id
	connector.CurrentPowerLimit = 0
	state.registerTransaction(transaction.Id)
	h.updateActiveTransactionsCounter()

	if h.database != nil {

		_ = h.database.UpdateTagLastSeen(userTag)

		if h.billing != nil {
			err := h.billing.OnTransactionStart(transaction)
			if err != nil {
				h.logger.Warn(fmt.Sprintf("billing: %v", err))
				eventMessage := &internal.EventMessage{
					ChargePointId: chargePointId,
					ConnectorId:   transaction.ConnectorId,
					Username:      transaction.Username,
					IdTag:         transaction.IdTag,
					Info:          fmt.Sprintf("billing: %v", err),
					Payload:       request,
				}
				go h.notifyEventListeners(internal.Alert, eventMessage)
			}
		}

		// billing module may set payment plan for transaction,
		// but it does not store transaction in database

		err := h.database.AddTransaction(transaction)
		if err != nil {
			h.logger.Error("add transaction", err)
		}

		err = h.database.UpdateConnector(connector)
		if err != nil {
			h.logger.Error("update connector", err)
		}
	}

	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   transaction.ConnectorId,
		LocationId:    state.model.LocationId,
		Evse:          state.EvseId(transaction.ConnectorId),
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
	h.mux.Lock()
	defer h.mux.Unlock()

	meterValues, _ := h.database.ReadAllTransactionMeterValues(request.TransactionId)

	// save request data as is for debugging
	if request.TransactionData != nil && len(request.TransactionData) > 0 {
		_ = h.database.SaveStopTransactionRequest(request)
	}

	// removing all listeners and observers
	h.trigger.Unregister <- request.TransactionId
	delete(h.lastMeter, request.TransactionId)

	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewStopTransactionResponse(), nil
	}

	state.unregisterTransaction(request.TransactionId)
	h.updateActiveTransactionsCounter()

	if h.database == nil {
		return core.NewStopTransactionResponse(), nil
	}

	transaction, err := h.database.GetTransaction(request.TransactionId)
	if err != nil {
		h.logger.Error(fmt.Sprintf("on stop: transaction #%v not found", request.TransactionId), err)
		return core.NewStopTransactionResponse(), nil
	}

	transaction.Init()
	transaction.Lock()
	defer transaction.Unlock()

	if meterValues != nil {
		transaction.MeterValues = meterValues
	}

	connector := h.getConnector(state, transaction.ConnectorId)
	connector.Lock()
	defer func() {
		connector.Unlock()
	}()
	id := fmt.Sprintf("%d", transaction.ConnectorId)
	counters.ObservePowerRate(state.model.LocationId, chargePointId, id, 0)

	connector.CurrentTransactionId = -1
	connector.CurrentPowerLimit = 0
	err = h.database.UpdateConnector(connector)
	if err != nil {
		h.logger.Error("update connector", err)
	}

	if transaction.IsFinished && transaction.MeterStop >= request.MeterStop {
		h.logger.Warn(fmt.Sprintf("transaction #%d is already finished", request.TransactionId))
		eventMessage := &internal.EventMessage{
			ChargePointId: chargePointId,
			ConnectorId:   transaction.ConnectorId,
			LocationId:    state.model.LocationId,
			Evse:          state.EvseId(transaction.ConnectorId),
			TransactionId: request.TransactionId,
			Username:      transaction.Username,
			IdTag:         transaction.IdTag,
			Info:          "Transaction is already finished",
			Payload:       request,
		}
		go h.notifyEventListeners(internal.Alert, eventMessage)
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
			LocationId:    state.model.LocationId,
			Evse:          state.EvseId(transaction.ConnectorId),
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

	go func() {
		consumedPower := transaction.MeterStop - transaction.MeterStart
		if consumedPower < 0 {
			consumedPower = 0
		}
		counters.CountTransaction(state.model.LocationId, chargePointId)
		counters.CountConsumedPower(state.model.LocationId, chargePointId, float64(consumedPower))

		consumed := utility.IntToString(consumedPower)
		price := utility.IntAsPrice(transaction.PaymentAmount)
		eventMessage := &internal.EventMessage{
			ChargePointId: chargePointId,
			ConnectorId:   transaction.ConnectorId,
			LocationId:    state.model.LocationId,
			Evse:          state.EvseId(transaction.ConnectorId),
			Time:          transaction.TimeStart,
			Username:      transaction.Username,
			IdTag:         transaction.IdTag,
			Status:        connector.Status,
			TransactionId: transaction.Id,
			Consumed:      consumedPower,
			Info:          fmt.Sprintf("consumed %s kW; %s €", consumed, price),
			Payload:       request,
		}
		h.notifyEventListeners(internal.TransactionStop, eventMessage)
	}()

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("stopped transaction %d %s", request.TransactionId, request.Reason))
	return core.NewStopTransactionResponse(), nil
}

func (h *SystemHandler) OnMeterValues(chargePointId string, request *core.MeterValuesRequest) (*core.MeterValuesResponse, error) {
	chp, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewMeterValuesResponse(), nil
	}
	connector := h.getConnector(chp, request.ConnectorId)

	transactionId := request.TransactionId
	if transactionId == nil {
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%v", request.MeterValue))
		// check, if we received a triggered message, need to unregister connector
		for _, sampledValues := range request.MeterValue {
			for _, value := range sampledValues.SampledValue {
				if value.Context == types.ReadingContextTrigger {
					h.trigger.UnregisterConnector(connector)
				}
			}
		}
		return core.NewMeterValuesResponse(), nil
	}
	if h.database == nil {
		return core.NewMeterValuesResponse(), nil
	}

	transaction, err := h.database.GetTransaction(*transactionId)
	if transaction == nil {
		h.logger.Error("get transaction failed", err)
		return core.NewMeterValuesResponse(), nil
	}

	for _, sampledValue := range request.MeterValue {

		meter := entity.NewMeter(*transactionId, connector.Id, connector.Status, sampledValue.Timestamp.Time)

		for _, value := range sampledValue.SampledValue {

			if value.Context != types.ReadingContextTrigger {
				continue
			}

			// read value of active energy import register only for triggered messages
			if value.Measurand == types.MeasurandEnergyActiveImportRegister {

				meter.Value = utility.ToInt(value.Value)
				meter.Unit = string(value.Unit)
				meter.Measurand = string(value.Measurand)

				consumedTotal := meter.Value - transaction.MeterStart
				if consumedTotal > 0 {
					meter.ConsumedEnergy = consumedTotal
				}

				lastMeter, found := h.lastMeter[transaction.Id]
				if found {
					consumed := meter.Value - lastMeter.Value
					seconds := meter.Time.Sub(lastMeter.Time).Seconds()
					if consumed > 0 && seconds > 0.0 {
						meter.PowerRateWh = float64(consumed) * (3600 / 1000) / seconds //used in metrics
						meter.PowerRate = int(meter.PowerRateWh * 1000)
					}
				}
			}

			if value.Measurand == types.MeasurandSoC {
				meter.BatteryLevel = utility.ToInt(value.Value)
			}

			if value.Measurand == types.MeasurandPowerActiveImport {
				meter.PowerActive = utility.ToInt(value.Value)
			}

		}

		if meter.Value > 0 {
			h.lastMeter[transaction.Id] = meter

			// replace calculated values if received data from charger
			if meter.PowerActive > 0 {
				meter.PowerRate = meter.PowerActive
				meter.PowerRateWh = float64(meter.PowerActive) / 1000
			}

			counters.ObservePowerRate(chp.model.LocationId, chargePointId, connector.ID(), meter.PowerRateWh)

			// billing calculates charge price and must be called before meter value save
			err = h.billing.OnMeterValue(transaction, meter)
			if err != nil {
				h.logger.Error("billing on meter value", err)
			}

			err = h.database.AddTransactionMeterValue(meter)
			if err != nil {
				h.logger.Error("add transaction meter value", err)
			}
		}
	}

	return core.NewMeterValuesResponse(), nil
}

func (h *SystemHandler) OnStatusNotification(chargePointId string, request *core.StatusNotificationRequest) (*core.StatusNotificationResponse, error) {
	h.mux.Lock()
	defer h.mux.Unlock()

	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return core.NewStatusNotificationResponse(), nil
	}

	currentTransactionId := -1
	state.errorCode = request.ErrorCode

	if request.ConnectorId > 0 {

		connector := h.getConnector(state, request.ConnectorId)
		connector.Lock()
		defer func() {
			connector.Unlock()
			h.checkListenTransaction(connector, state.model.IsOnline)
		}()

		connector.Status = string(request.Status)
		connector.StatusTime = request.Timestamp.Time
		connector.State = h.stateFromStatus(request.Status)
		connector.Info = request.Info
		connector.VendorId = request.VendorId
		connector.ErrorCode = string(request.ErrorCode)

		// URBAN sends Available status while transaction is ongoing
		//if request.Status == core.ChargePointStatusAvailable {
		//	connector.CurrentTransactionId = -1
		//	connector.CurrentPowerLimit = 0
		//}

		if h.database != nil {
			err := h.database.UpdateConnector(connector)
			if err != nil {
				h.logger.Error("update status", err)
			}
		}
		currentTransactionId = connector.CurrentTransactionId

		if request.Status != core.ChargePointStatusCharging {
			counters.ObservePowerRate(state.model.LocationId, chargePointId, strconv.Itoa(connector.Id), 0)
		}

	} else {
		state.status = request.Status
		state.model.Status = string(request.Status)
		state.model.StatusTime = request.Timestamp.Time
		state.model.Info = request.Info
		if h.database != nil {
			err := h.database.UpdateChargePointStatus(state.model)
			if err != nil {
				h.logger.Error("update status", err)
			}
		}
	}

	connectorName := "main controller"
	if request.ConnectorId > 0 {
		connectorName = fmt.Sprintf("connector #%v", request.ConnectorId)
	}

	errorCode := ""
	if request.ErrorCode != core.NoError {
		errorCode = fmt.Sprintf(" (%v; %s)", request.ErrorCode, request.VendorErrorCode)
		counters.ObserveError(state.model.LocationId, chargePointId, request.VendorErrorCode)

		data := &entity.ErrorData{
			Location:        state.model.LocationId,
			ChargePointID:   chargePointId,
			ConnectorID:     request.ConnectorId,
			ErrorCode:       string(request.ErrorCode),
			Info:            request.Info,
			Status:          string(request.Status),
			Timestamp:       request.GetTimestamp(),
			VendorId:        request.VendorId,
			VendorErrorCode: request.VendorErrorCode,
		}
		if h.errorListener != nil {
			h.errorListener.OnError(data)
		}
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%s: %v%s", connectorName, request.Status, errorCode))

	eventMessage := &internal.EventMessage{
		ChargePointId: chargePointId,
		ConnectorId:   request.ConnectorId,
		LocationId:    state.model.LocationId,
		Evse:          state.EvseId(request.ConnectorId),
		Time:          h.getTime(),
		Status:        string(request.Status),
		TransactionId: currentTransactionId,
		Info:          fmt.Sprintf("%s%s", request.Info, errorCode),
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
	h.mux.Lock()
	defer h.mux.Unlock()
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
	h.mux.Lock()
	defer h.mux.Unlock()
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

func (h *SystemHandler) OnGetConfiguration(chargePointId string, key string) (*core.GetConfigurationRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	keys := make([]string, 0)
	if key != "" {
		keys = append(keys, key)
	}
	request := core.NewGetConfigurationRequest(keys)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("get configuration: %v", request.Key))
	return request, nil
}

func (h *SystemHandler) OnChangeConfiguration(chargePointId string, payload string) (*core.ChangeConfigurationRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	var request core.ChangeConfigurationRequest
	err := json.Unmarshal([]byte(payload), &request)
	if err != nil {
		return nil, fmt.Errorf("invalid payload")
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("change configuration: %v=%v", request.Key, request.Value))
	return &request, nil
}

func (h *SystemHandler) OnSetChargingProfile(chargePointId string, connectorId int, payload string) (*smartcharging.SetChargingProfileRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	var profile types.ChargingProfile
	err := json.Unmarshal([]byte(payload), &profile)
	if err != nil {
		return nil, fmt.Errorf("invalid payload")
	}
	request := smartcharging.NewSetChargingProfileRequest(connectorId, &profile)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("set charge profile: connector %d; %v", connectorId, request.ChargingProfile.ChargingProfilePurpose))
	return request, nil
}

func (h *SystemHandler) OnGetCompositeSchedule(chargePointId string, connectorId int, payload string) (*smartcharging.GetCompositeScheduleRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	duration, err := strconv.Atoi(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid payload")
	}
	request := smartcharging.NewGetCompositeScheduleRequest(connectorId, duration)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("get schedule: connector %d; duration %d", request.ConnectorId, request.Duration))
	return request, nil
}

func (h *SystemHandler) OnClearChargingProfile(chargePointId string, payload string) (*smartcharging.ClearChargingProfileRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	var request smartcharging.ClearChargingProfileRequest
	err := json.Unmarshal([]byte(payload), &request)
	if err != nil {
		return nil, fmt.Errorf("invalid payload")
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId,
		fmt.Sprintf("connector %d; purpose %v; stack level %d", request.ConnectorId, request.ChargingProfilePurpose, request.StackLevel))
	return &request, nil
}

func (h *SystemHandler) OnGetDiagnostics(chargePointId string, payload string) (*firmware.GetDiagnosticsRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	if payload == "" {
		return nil, fmt.Errorf("empty location")
	}
	request := firmware.NewGetDiagnosticsRequest(payload)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("location: %s***", payload[0:10]))
	return request, nil
}

func (h *SystemHandler) OnReset(chargePointId string, payload string) (*core.ResetRequest, error) {
	_, ok := h.getChargePoint(chargePointId)
	if !ok {
		return nil, fmt.Errorf("charge point not found")
	}
	request := core.NewResetRequest(core.ResetType(payload))
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("reset: %v", request.Type))
	return request, nil
}

func (h *SystemHandler) OnOnlineStatusChanged(id string, isOnline bool) {
	h.mux.Lock()
	defer h.mux.Unlock()

	state, ok := h.getChargePoint(id)
	if !ok {
		h.logger.Warn(fmt.Sprintf("online status: charge point %s not found", id))
		return
	}
	if state.model == nil {
		return
	}

	// don't send event and update database if status is not changed and charge point is offline
	if !isOnline && !state.model.IsOnline {
		return
	}

	if state.model.IsOnline != isOnline {
		info := fmt.Sprintf("comes online; was offline %v", utility.TimeAgo(state.model.EventTime))
		if !isOnline {
			info = "goes OFFLINE"
		}
		eventMessage := &internal.EventMessage{
			ChargePointId: id,
			Time:          h.getTime(),
			Info:          info,
		}
		go h.notifyEventListeners(internal.Alert, eventMessage)

		if state.connectors != nil {
			status := state.model.Status
			if !isOnline {
				status = "Offline"
			}
			for _, c := range state.connectors {
				// check active transactions only if online status is changed
				h.checkListenTransaction(c, isOnline)

				// for OCPI purpose, need to notify about every connector's state changes
				eventMessage = &internal.EventMessage{
					LocationId: state.model.LocationId,
					Evse:       state.EvseId(c.Id),
					Status:     status,
				}
				go h.notifyEventListeners(internal.StatusNotification, eventMessage)
			}
		}
	}

	state.model.IsOnline = isOnline
	state.model.EventTime = h.getTime()

	err := h.database.UpdateOnlineStatus(id, isOnline)
	if err != nil {
		h.logger.Error("update online status", err)
	}

	// observe online status per locations
	onlineCounter, err := h.database.OnlineCounter()
	if err != nil {
		h.logger.Error("online counter", err)
		return
	}
	if onlineCounter != nil {
		for location, count := range onlineCounter {
			counters.ObserveConnections(location, count)
		}
	}
}

func (h *SystemHandler) checkAndFinishTransactions() {
	if h.database == nil {
		return
	}

	transactions, err := h.database.GetUnfinishedTransactions()
	if err != nil {
		h.logger.Error("get unfinished transactions", err)
		return
	}
	for _, transaction := range transactions {
		h.logger.Warn(fmt.Sprintf("transaction #%v was not finished correctly", transaction.Id))
		h.trigger.Unregister <- transaction.Id

		transaction.Init()
		transaction.Lock()
		transaction.IsFinished = true
		transaction.TimeStop = h.getTime()
		transaction.Reason = "stopped by system"

		meterValue, _ := h.database.ReadTransactionMeterValue(transaction.Id)
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

		state, _ := h.getChargePoint(transaction.ChargePointId)
		if state != nil {
			state.unregisterTransaction(transaction.Id)
		}

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
	h.updateActiveTransactionsCounter()
}

func (h *SystemHandler) checkListenTransaction(connector *entity.Connector, isOnline bool) {
	if connector.CurrentTransactionId >= 0 {
		h.logger.FeatureEvent("CheckListenTransaction", connector.ChargePointId, fmt.Sprintf("connector %d; status %s; online %v; transaction: %d", connector.Id, connector.Status, isOnline, connector.CurrentTransactionId))
		if !isOnline {
			h.trigger.Unregister <- connector.CurrentTransactionId
		} else if len(connector.Status) != 0 && connector.Status != string(core.ChargePointStatusCharging) {
			h.trigger.Unregister <- connector.CurrentTransactionId
		} else {
			h.trigger.Register <- connector
		}
	} else {
		h.trigger.Unregister <- connector.CurrentTransactionId
	}
}

func (h *SystemHandler) getUserTag(idTag string) *entity.UserTag {
	userTag := entity.NewUserTag(idTag)

	if h.database == nil {
		userTag.IsEnabled = h.acceptTags
		return userTag
	}

	savedTag, _ := h.database.GetUserTag(userTag.IdTag)
	if savedTag != nil {
		userTag = savedTag
	} else {
		userTag.IsEnabled = h.acceptTags
		userTag.DateRegistered = time.Now()
		err := h.database.AddUserTag(userTag)
		if err != nil {
			h.logger.Error("add user tag to database", err)
		}
	}
	_ = h.database.UpdateTagLastSeen(userTag)

	return userTag
}

// authorize checks the id tag with connected authorization service;
// returns error if authorization service is not set
func (h *SystemHandler) authorize(locationId, evseId, idTag string) (*authResult, error) {
	if h.auth == nil {
		return nil, fmt.Errorf("authorization service is not set")
	}
	result := &authResult{}
	result.allowed, result.expired, result.blocked, result.info = h.auth.Authorize(locationId, evseId, idTag)
	return result, nil
}

func (h *SystemHandler) stateFromStatus(status core.ChargePointStatus) string {
	switch status {
	case core.ChargePointStatusAvailable:
		return "available"
	case core.ChargePointStatusPreparing:
		return "occupied"
	case core.ChargePointStatusCharging:
		return "occupied"
	case core.ChargePointStatusSuspendedEV:
		return "occupied"
	case core.ChargePointStatusSuspendedEVSE:
		return "occupied"
	case core.ChargePointStatusFinishing:
		return "occupied"
	case core.ChargePointStatusUnavailable:
		return "unavailable"
	case core.ChargePointStatusFaulted:
		return "unavailable"
	}
	return string(status)
}
