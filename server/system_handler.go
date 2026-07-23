package server

import (
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"evsys/metrics/counters"
	"evsys/ocpp"
	"evsys/ocpp/v16/core"
	"evsys/ocpp/v16/firmware"
	"evsys/ocpp/v16/localauth"
	"evsys/ocpp/v16/remotetrigger"
	"evsys/ocpp/v16/smartcharging"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"strconv"
	"sync"
	"time"
)

var newTransactionId = 1

const (
	defaultHeartbeatInterval = 600
	sourceOCPI               = "OCPI"

	// reasons the sweep writes when it closes a transaction the charger never stopped itself.
	// A later StopTransaction carrying one of these on the stored record means the sweep closed
	// a session that was still live.
	reasonStoppedBySystem = "stopped by system"
	reasonAbortedBySystem = "aborted by system"

	// transactionStaleAfter is how long a transaction may go without a meter value before the
	// sweeper treats it as abandoned. Meter values are triggered every 20s, so this leaves ample
	// room for a charge point that is merely slow or briefly offline.
	transactionStaleAfter = 20 * time.Minute
	// transactionSweepInterval is how often abandoned transactions are looked for.
	transactionSweepInterval = 5 * time.Minute
	// transactionReleaseGrace is how long a transaction whose connector has moved on is left
	// alone. OnStopTransaction finishes the transaction before it releases the connector, so the
	// two are never briefly inconsistent on the happy path - but if that first write fails the
	// connector is still released, and this keeps the sweeper off the result until the charge
	// point has had a chance to report again.
	transactionReleaseGrace = 2 * time.Minute
)

type BillingService interface {
	OnTransactionStart(transaction *entity.Transaction) error
	OnTransactionFinished(transaction *entity.Transaction) error
	OnMeterValue(transaction *entity.Transaction, transactionMeter *entity.TransactionMeter) error
}

type ErrorListener interface {
	OnError(data *entity.ErrorData)
}

// RequestSender sends a proactive OCPP request to a connected charge point.
type RequestSender interface {
	SendRequest(clientId string, request ocpp.Request) (string, error)
}

type ChargePointState struct {
	status            core.ChargePointStatus
	diagnosticsStatus firmware.DiagnosticsStatus
	firmwareStatus    firmware.Status
	connectors        map[int]*entity.Connector // No assumptions about the # of connectors
	transactions      map[int]*int
	errorCode         core.ChargePointErrorCode
	model             *entity.ChargePoint
	triggerMessage    bool
}

func newChargePointState(chp *entity.ChargePoint) *ChargePointState {
	return &ChargePointState{
		connectors:     make(map[int]*entity.Connector),
		transactions:   make(map[int]*int),
		model:          chp,
		triggerMessage: chp.TriggerMessage,
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
	chargePoints    map[string]*ChargePointState
	lastMeter       map[int]*entity.TransactionMeter
	database        internal.Database
	billing         BillingService
	auth            AuthService
	errorListener   ErrorListener
	logger          internal.LogHandler
	eventListeners  []internal.EventHandler
	trigger         *Trigger
	protocolAdapter *ProtocolAdapter // Adapter for converting between OCPP versions
	server          RequestSender    // used to push proactive requests to charge points
	debug           bool
	acceptTags      bool
	acceptPoints    bool
	// meterSampleInterval, in seconds, is pushed to a charge point on boot to re-assert periodic
	// metering; 0 disables the push
	meterSampleInterval int
	location            *time.Location
	mux                 sync.Mutex

	// consumedSeries remembers which label pairs the consumed power gauge currently holds, so a
	// group that drops out of the daily aggregation - yesterday's sessions after midnight - is
	// zeroed instead of staying stuck on its last value. Guarded by consumedMux, not h.mux: the
	// refresh runs a database aggregation and must not block the OCPP handlers.
	consumedSeries map[consumedSeriesKey]bool
	consumedMux    sync.Mutex
}

type consumedSeriesKey struct {
	location      string
	chargePointId string
}

func NewSystemHandler(location *time.Location) *SystemHandler {
	handler := &SystemHandler{
		chargePoints:    make(map[string]*ChargePointState),
		lastMeter:       make(map[int]*entity.TransactionMeter),
		eventListeners:  make([]internal.EventHandler, 0),
		protocolAdapter: NewProtocolAdapter(),
		location:        location,
		mux:             sync.Mutex{},
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

func (h *SystemHandler) SetServer(server RequestSender) {
	h.server = server
}

func (h *SystemHandler) SetMeterSampleInterval(seconds int) {
	h.meterSampleInterval = seconds
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
			h.logger.Warn(fmt.Sprintf("no transactions in the database; id will start with %d; %v", newTransactionId, err))
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

	go h.sweepTransactions()

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
			if c.CurrentTransactionId == -1 {
				continue
			}
			if h.releaseFinishedTransaction(c) {
				continue
			}
			state.registerTransaction(c.CurrentTransactionId)
		}
	}
	h.chargePoints[chp.Id] = state
	return state
}

/*
releaseFinishedTransaction clears a connector pointer that references an already finished
transaction, and reports whether it did.

Such a pointer is a dead end: OnStartTransaction refuses every new session on the connector with
ConcurrentTx, and nothing driven by the transactions collection can find it, because the sweeper
only looks at transactions that are still open.

The pointer is cleared only when the transaction is positively known to be finished. A lookup that
fails is left alone, since a transient database error is indistinguishable from a missing document
and a live session must not be released on a guess.
*/
func (h *SystemHandler) releaseFinishedTransaction(connector *entity.Connector) bool {
	if h.database == nil {
		return false
	}
	transaction, err := h.database.GetTransaction(connector.CurrentTransactionId)
	if err != nil || transaction == nil || !transaction.IsFinished {
		return false
	}

	h.logger.Warn(fmt.Sprintf("connector %d of %s points at finished transaction %d, releasing",
		connector.Id, connector.ChargePointId, connector.CurrentTransactionId))

	connector.Lock()
	defer connector.Unlock()

	connector.CurrentTransactionId = -1
	connector.CurrentPowerLimit = 0
	if err = h.database.UpdateConnector(connector); err != nil {
		h.logger.Error("update connector", err)
	}
	return true
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

	if regStatus == core.RegistrationStatusAccepted {
		go h.reconcileChargePointTransactions(chargePointId)
		go h.enforceMeterValueInterval(chargePointId, state.triggerMessage)
	}

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, string(regStatus))
	return core.NewBootNotificationResponse(types.NewDateTime(h.getTime()), defaultHeartbeatInterval, regStatus), nil
}

// enforceMeterValueInterval re-asserts periodic metering after a boot. A charge point that comes
// back with MeterValueSampleInterval reset to 0 stops reporting meter values on its own, and with
// triggering disabled nothing else refreshes the transaction, so the sweep then aborts live
// sessions. Pushing the configured interval on every boot keeps that from happening silently. It
// only applies to charge points with triggering off, which are the ones that depend on the charger
// reporting by itself; where triggering is on the server polls and the interval is irrelevant. It
// is a no-op unless meter_value_sample_interval is set in the config.
func (h *SystemHandler) enforceMeterValueInterval(chargePointId string, triggerMessage bool) {
	if triggerMessage || h.meterSampleInterval <= 0 || h.server == nil {
		return
	}
	request := &core.ChangeConfigurationRequest{
		Key:   "MeterValueSampleInterval",
		Value: strconv.Itoa(h.meterSampleInterval),
	}
	if _, err := h.server.SendRequest(chargePointId, request); err != nil {
		h.logger.Error(fmt.Sprintf("set meter value interval on %s", chargePointId), err)
		return
	}
	h.logger.FeatureEvent(core.ChangeConfigurationFeatureName, chargePointId,
		fmt.Sprintf("MeterValueSampleInterval=%d", h.meterSampleInterval))
}

func (h *SystemHandler) OnAuthorize(chargePointId string, request *core.AuthorizeRequest) (*core.AuthorizeResponse, error) {
	authStatus := h.authorizeIdTag(chargePointId, request.IdTag)
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("id tag: %s; status: %s", request.IdTag, authStatus))
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

		transaction, err := h.database.GetTransaction(connector.CurrentTransactionId)
		if err != nil && !internal.IsNotFound(err) {
			// the connector claims a transaction and the database cannot say what became of it.
			// Starting anyway would overwrite the pointer, and with it any record of a session
			// that is still running, so refuse until the state can be read again
			h.logger.Error("start transaction: cannot read the transaction held by the connector", err)
			return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusBlocked), 0), nil
		}
		if transaction != nil {
			if !transaction.IsFinished && transaction.ConnectorId == connector.Id {
				h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("start transaction: connector %d is already started transaction %d", connector.Id, connector.CurrentTransactionId))
				return core.NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted), connector.CurrentTransactionId), nil
			}

			if transaction.IsFinished {
				// pointer left behind by a stop that closed the transaction without releasing
				// the connector; without clearing it here every start is refused forever
				h.logger.Warn(fmt.Sprintf("connector %d still points at finished transaction %d, releasing", connector.Id, connector.CurrentTransactionId))
				state.unregisterTransaction(connector.CurrentTransactionId)
				connector.CurrentTransactionId = -1
				connector.CurrentPowerLimit = 0
				if err := h.database.UpdateConnector(connector); err != nil {
					h.logger.Error("update connector", err)
				}
			} else {
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

	}

	// TODO: check flow if the transaction is authorized on a charger itself (by credit card for example)
	//auth := h.authorizeIdTag(chargePointId, request.IdTag)
	//if auth != types.AuthorizationStatusAccepted {
	//	return core.NewStartTransactionResponse(types.NewIdTagInfo(auth), 0), nil
	//}

	userTag := h.getUserTag(request.IdTag)

	transaction := &entity.Transaction{
		IdTag:         userTag.IdTag,
		IdTagNote:     userTag.Note,
		Username:      userTag.Username,
		IsFinished:    false,
		ConnectorId:   request.ConnectorId,
		ChargePointId: chargePointId,
		MeterStart:    request.MeterStart,
		TimeStart:     request.GetTimestamp(),
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

	// a real StopTransaction arriving for a transaction the sweep already closed means that close
	// was premature - the session was still live on the charger. Trace it before the normal path
	// overwrites the system-generated figures with the charger's real ones.
	if transaction.IsFinished && isSystemClosedReason(transaction.Reason) {
		h.logger.Warn(fmt.Sprintf("late StopTransaction for transaction #%d on %s: system had closed it as %q at %s, charger now reports meter %d, reason %s",
			request.TransactionId, chargePointId, transaction.Reason, transaction.TimeStop.Format(time.RFC3339), request.MeterStop, request.Reason))
		eventMessage := &internal.EventMessage{
			ChargePointId: chargePointId,
			ConnectorId:   transaction.ConnectorId,
			LocationId:    state.model.LocationId,
			Evse:          state.EvseId(transaction.ConnectorId),
			TransactionId: request.TransactionId,
			Username:      transaction.Username,
			IdTag:         transaction.IdTag,
			Info:          fmt.Sprintf("late stop after the system closed the transaction as %q", transaction.Reason),
			Payload:       request,
		}
		go h.notifyEventListeners(internal.Alert, eventMessage)
	}

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

	// The connector is released only once the transaction is durably finished. Releasing first
	// leaves the database briefly showing an open transaction whose connector has moved on, which
	// is exactly the shape of an abandoned one, and the sweeper would claim a stop in progress.
	releaseConnector := func() {
		connector.CurrentTransactionId = -1
		connector.CurrentPowerLimit = 0
		if err = h.database.UpdateConnector(connector); err != nil {
			h.logger.Error("update connector", err)
		}
	}

	if transaction.IsFinished && transaction.MeterStop >= request.MeterStop {
		releaseConnector()
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
	transaction.TimeStop = request.GetTimestamp()
	transaction.MeterStop = request.MeterStop
	transaction.Reason = string(request.Reason)

	// request data may contain meter values of begin and end of transaction
	if request.TransactionData != nil {
		for _, data := range request.TransactionData {
			if data.SampledValue != nil {
				for _, value := range data.SampledValue {
					if value.Context == types.ReadingContextTransactionBegin {
						transaction.MeterStart = utility.ToInt(value.Value)
						transaction.TimeStart = data.GetTimestamp()
					}
					if value.Context == types.ReadingContextTransactionEnd {
						transaction.MeterStop = utility.ToInt(value.Value)
						transaction.TimeStop = data.GetTimestamp()
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

	// released even if the write above failed: the connector has to be usable again either way,
	// and the sweeper still reaches a transaction left open once its grace period elapses
	releaseConnector()

	go func() {
		consumedPower := transaction.MeterStop - transaction.MeterStart
		if consumedPower < 0 {
			consumedPower = 0
		}
		counters.CountConsumedPower(state.model.LocationId, chargePointId, float64(consumedPower))
		h.observeConsumedPower()

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
		// transactionId is optional in 1.6; a charger that reports periodically on
		// its own (typically with trigger_message disabled) omits it. Recover the
		// running transaction from the connector so the readings are still recorded
		// instead of dropped.
		if request.ConnectorId > 0 && connector.CurrentTransactionId >= 0 {
			tid := connector.CurrentTransactionId
			transactionId = &tid
		}
	}
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

		meter := entity.NewMeter(*transactionId, connector.Id, connector.Status, sampledValue.GetTimestamp())

		for _, value := range sampledValue.SampledValue {

			// Every reading is recorded regardless of context: a charge point
			// may report periodically on its own instead of only on request,
			// and triggerMessage decides whether we ask for readings, not
			// which of the ones that arrive are worth keeping.
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

			// Voltage and current say whether a session was held back by the
			// limit we set, by the charge point's own hardware or by the car -
			// which a power figure on its own cannot distinguish.
			if isVehicleSideReading(value) {
				switch value.Measurand {
				case types.MeasurandVoltage:
					meter.Voltage = keepHigher(meter.Voltage, value.Value)
				case types.MeasurandCurrentImport:
					meter.CurrentImport = keepHigher(meter.CurrentImport, value.Value)
				case types.MeasurandCurrentOffered:
					meter.CurrentOffered = keepHigher(meter.CurrentOffered, value.Value)
				}
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

// isVehicleSideReading reports whether a sample measures the side of the charge
// point the vehicle draws from. A DC charge point may also report its own grid
// connection, and an Inlet reading of 210A at 400V three-phase says nothing
// about what reached the car. An unset location means the charge point did not
// distinguish, which is the common case and the one we want.
func isVehicleSideReading(value types.SampledValue) bool {
	switch value.Location {
	case "", types.LocationOutlet, types.LocationCable, types.LocationEV:
		return true
	default:
		return false
	}
}

// keepHigher folds a sampled reading into the value kept so far. A charge point
// may report one sample per phase; the highest is the one that binds against a
// per-phase limit, and single-phase and DC readings are unaffected because there
// is only ever one sample.
func keepHigher(current float64, sampled string) float64 {
	value, err := strconv.ParseFloat(sampled, 64)
	if err != nil || value < current {
		return current
	}
	return value
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
		connector.StatusTime = request.GetTimestamp()
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
		state.model.StatusTime = request.GetTimestamp()
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

// sweepTransactions closes abandoned transactions on startup and then periodically, for the
// lifetime of the process. StatusNotification is not a reliable trigger on its own: a charge point
// that goes silent mid-transaction never sends one.
func (h *SystemHandler) sweepTransactions() {
	h.checkAndFinishTransactions()

	ticker := time.NewTicker(transactionSweepInterval)
	defer ticker.Stop()
	for range ticker.C {
		h.checkAndFinishTransactions()
	}
}

func (h *SystemHandler) checkAndFinishTransactions() {
	if h.database == nil {
		return
	}

	now := h.getTime()
	transactions, err := h.database.GetUnfinishedTransactions(
		now.Add(-transactionStaleAfter),
		now.Add(-transactionReleaseGrace),
	)
	if err != nil {
		h.logger.Error("get unfinished transactions", err)
		return
	}
	for _, swept := range transactions {
		idle := now.Sub(swept.LastActivity).Round(time.Second)
		h.logger.Warn(fmt.Sprintf("transaction #%v closed by sweep: %s (idle %s, last activity %s)",
			swept.Id, swept.Cause, idle, swept.LastActivity.Format(time.RFC3339)))
		h.finishAbandonedTransaction(&swept.Transaction)
	}
	h.mux.Lock()
	h.updateActiveTransactionsCounter()
	h.mux.Unlock()

	h.observeConsumedPower()
}

// observeConsumedPower refreshes the ocpp_consumed_power gauge with today's per-charge-point
// energy totals. Series absent from the aggregation result are set to zero, which is how the
// gauge comes back down after midnight; without that, yesterday's totals would stay on the graph
// until the process restarts.
func (h *SystemHandler) observeConsumedPower() {
	if h.database == nil {
		return
	}
	consumed, err := h.database.GetTodayConsumedEnergy()
	if err != nil {
		h.logger.Error("get today consumed energy", err)
		return
	}

	h.consumedMux.Lock()
	defer h.consumedMux.Unlock()

	current := make(map[consumedSeriesKey]bool, len(consumed))
	for _, c := range consumed {
		counters.ObserveConsumedPower(c.ID.Location, c.ID.ChargePointID, float64(c.Consumed))
		counters.ObserveTransactionCount(c.ID.Location, c.ID.ChargePointID, c.Count)
		current[consumedSeriesKey{c.ID.Location, c.ID.ChargePointID}] = true
	}
	for key := range h.consumedSeries {
		if !current[key] {
			counters.ObserveConsumedPower(key.location, key.chargePointId, 0)
			counters.ObserveTransactionCount(key.location, key.chargePointId, 0)
		}
	}
	h.consumedSeries = current
}

/*
reconcileChargePointTransactions closes transactions left open by a charge point that has just
rebooted, so their connectors are released without waiting out the sweep interval.

A transaction that is still reporting meter values is left alone. BootNotification does not always
mean the charge point restarted: some firmware sends it on a plain WebSocket reconnect, and this
fleet already has chargers that report Available mid-session. Closing a live transaction here would
be unrecoverable, because OnMeterValues keeps recording against the closed id, OnStopTransaction
drops the real stop as already finished, and the connector would be offered to a second driver
while the first car is still drawing power.

A StopTransaction queued while the charge point was offline may still arrive afterwards; it is
handled normally, since OnStopTransaction overwrites a transaction whose stored MeterStop is lower
than the reported one.
*/
func (h *SystemHandler) reconcileChargePointTransactions(chargePointId string) {
	if h.database == nil {
		return
	}

	transactions, err := h.database.GetUnfinishedTransactionsForChargePoint(chargePointId)
	if err != nil {
		h.logger.Error("get unfinished transactions for charge point", err)
		return
	}
	cutoff := h.getTime().Add(-transactionReleaseGrace)
	for _, transaction := range transactions {
		if meterValue, _ := h.database.ReadTransactionMeterValue(transaction.Id); meterValue != nil {
			if meterValue.Time.After(cutoff) {
				h.logger.Warn(fmt.Sprintf("transaction #%v is still reporting on %s, left open", transaction.Id, chargePointId))
				continue
			}
		}
		h.logger.Warn(fmt.Sprintf("transaction #%v was left open by a reboot of %s", transaction.Id, chargePointId))
		h.finishAbandonedTransaction(transaction)
	}
	if len(transactions) > 0 {
		h.mux.Lock()
		h.updateActiveTransactionsCounter()
		h.mux.Unlock()
		h.observeConsumedPower()
	}
}

// finishAbandonedTransaction closes a single transaction that will never receive its
// StopTransaction, bills it if any energy was measured, and frees the connector it was holding.
func (h *SystemHandler) finishAbandonedTransaction(transaction *entity.Transaction) {
	h.trigger.Unregister <- transaction.Id

	transaction.Init()
	transaction.Lock()
	transaction.IsFinished = true

	meterValue, _ := h.database.ReadTransactionMeterValue(transaction.Id)
	if meterValue != nil {
		transaction.MeterStop = meterValue.Value
		transaction.TimeStop = meterValue.Time
		transaction.Reason = reasonStoppedBySystem
		// the samples are deleted once the transaction is saved, so this is the only
		// chance to keep the consumption curve on the transaction, as a normal stop does
		if meterValues, _ := h.database.ReadAllTransactionMeterValues(transaction.Id); meterValues != nil {
			transaction.MeterValues = meterValues
		}
	} else {
		// no meter value ever arrived, so no energy was delivered and no time can be
		// attributed; stopping at the start time keeps the duration out of billing
		transaction.TimeStop = transaction.TimeStart
		transaction.Reason = reasonAbortedBySystem
	}

	if meterValue != nil {
		if err := h.billing.OnTransactionFinished(transaction); err != nil {
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
	}

	if err := h.database.UpdateTransaction(transaction); err != nil {
		h.logger.Error("update transaction", err)
	}
	transaction.Unlock()

	// getChargePoint and getConnector both mutate maps that every OCPP handler reads under
	// h.mux; this runs on the sweep goroutine, so it has to take the same lock or the two race
	// and crash the process on a concurrent map write
	h.mux.Lock()
	state, _ := h.getChargePoint(transaction.ChargePointId)
	abortedWhileActive := false
	var connectorStatus string
	if state != nil {
		state.unregisterTransaction(transaction.Id)

		// trace the dangerous case: the sweep is closing a transaction the charger may still
		// consider live. The connector's last reported status is the only in-band signal here,
		// and a charging status means an active session is being aborted and an occupied
		// connector freed - the car keeps drawing power with no record kept, and the connector
		// is offered to the next driver
		if connector := h.getConnector(state, transaction.ConnectorId); connector != nil {
			connector.Lock()
			connectorStatus = connector.Status
			connector.Unlock()
			abortedWhileActive = isActiveChargingStatus(connectorStatus)
		}

		h.releaseConnector(state, transaction)
		// a charge point that went silent sends neither StopTransaction nor a StatusNotification,
		// the two places that zero this gauge, so it would stay stuck on the last reading
		counters.ObservePowerRate(state.model.LocationId, transaction.ChargePointId, strconv.Itoa(transaction.ConnectorId), 0)
		// a session closed here never reaches OnStopTransaction, the only other place this
		// counter grows, so its energy would go unreported; a transaction with no meter value
		// delivered nothing and adds nothing
		if meterValue != nil {
			consumed := transaction.MeterStop - transaction.MeterStart
			if consumed < 0 {
				consumed = 0
			}
			counters.CountConsumedPower(state.model.LocationId, transaction.ChargePointId, float64(consumed))
		}
	}
	h.mux.Unlock()

	if err := h.database.DeleteTransactionMeterValues(transaction.Id); err != nil {
		h.logger.Error("delete transaction meter values", err)
	}

	info := fmt.Sprintf("transaction was %s", transaction.Reason)
	if abortedWhileActive {
		// distinct warning so this is greppable apart from the routine close of a dead session
		h.logger.Warn(fmt.Sprintf("transaction #%d on %s closed by system while connector %d still reports %s: session may be live on the charger",
			transaction.Id, transaction.ChargePointId, transaction.ConnectorId, connectorStatus))
		info = fmt.Sprintf("%s while connector still reports %s", info, connectorStatus)
	}

	eventMessage := &internal.EventMessage{
		ChargePointId: transaction.ChargePointId,
		ConnectorId:   transaction.ConnectorId,
		TransactionId: transaction.Id,
		Username:      transaction.Username,
		Status:        connectorStatus,
		Time:          h.getTime(),
		Info:          info,
	}
	go h.notifyEventListeners(internal.Alert, eventMessage)
}

// isActiveChargingStatus reports whether a connector status means an EV is still in a charging
// session - charging, or paused by either side. Preparing and Finishing are occupied but not
// actively charging, so they are not treated as an aborted-while-active session.
// isSystemClosedReason reports whether a stored transaction reason was written by the sweep rather
// than by the charge point, so a StopTransaction landing on it can be recognised as a late stop.
func isSystemClosedReason(reason string) bool {
	return reason == reasonStoppedBySystem || reason == reasonAbortedBySystem
}

func isActiveChargingStatus(status string) bool {
	switch status {
	case string(core.ChargePointStatusCharging),
		string(core.ChargePointStatusSuspendedEV),
		string(core.ChargePointStatusSuspendedEVSE):
		return true
	}
	return false
}

// releaseConnector clears the connector pointer left behind by a transaction the sweeper closed.
// Without this the connector stays pinned and OnStartTransaction keeps answering with the dead
// transaction id, making the connector unusable.
func (h *SystemHandler) releaseConnector(state *ChargePointState, transaction *entity.Transaction) {
	connector := h.getConnector(state, transaction.ConnectorId)
	if connector == nil {
		return
	}
	connector.Lock()
	defer connector.Unlock()

	if connector.CurrentTransactionId != transaction.Id {
		return
	}
	connector.CurrentTransactionId = -1
	connector.CurrentPowerLimit = 0
	if err := h.database.UpdateConnector(connector); err != nil {
		h.logger.Error("update connector", err)
	}
}

func (h *SystemHandler) checkListenTransaction(connector *entity.Connector, isOnline bool) {
	if chp, ok := h.getChargePoint(connector.ChargePointId); ok {
		if !chp.triggerMessage {
			h.logger.FeatureEvent("CheckListenTransaction", connector.ChargePointId, "trigger messages are disabled")
			return
		}
	}
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

// ============================================================================
// PROTOCOL ADAPTER HELPERS
// ============================================================================
// These methods provide version-agnostic interfaces for common operations
// They use the ProtocolAdapter to convert between OCPP versions

// GetProtocolAdapter returns the protocol adapter instance
func (h *SystemHandler) GetProtocolAdapter() *ProtocolAdapter {
	return h.protocolAdapter
}

// setTransactionProtocolVersion sets the protocol version for a transaction
// based on the charge point's protocol version
func (h *SystemHandler) setTransactionProtocolVersion(transaction *entity.Transaction, chargePointId string) {
	state, ok := h.getChargePoint(chargePointId)
	if ok && state.model != nil && state.model.ProtocolVersion != "" {
		transaction.ProtocolVersion = state.model.ProtocolVersion
	} else {
		// Default to OCPP 1.6J if not set
		transaction.ProtocolVersion = "ocpp1.6"
	}
}

// getConnectorByEvseAndConnectorId retrieves a connector using OCPP 2.0.1 EVSE structure
// Falls back to connector ID only for OCPP 1.6J compatibility
func (h *SystemHandler) getConnectorByEvseAndConnectorId(cps *ChargePointState, evseId *int, connectorId int) *entity.Connector {
	// For OCPP 2.0.1, we might need to look up by EVSE ID
	if evseId != nil {
		for _, connector := range cps.connectors {
			if connector.EvseId != nil && *connector.EvseId == *evseId && connector.Id == connectorId {
				return connector
			}
		}
	}

	// Fall back to connector ID lookup (OCPP 1.6J style)
	return h.getConnector(cps, connectorId)
}

// updateConnectorEvseId updates the EVSE ID for a connector (OCPP 2.0.1)
// This is called when we receive EVSE information from a 2.0.1 charge point
func (h *SystemHandler) updateConnectorEvseId(connector *entity.Connector, evseId *int) error {
	if connector.EvseId == nil && evseId != nil {
		connector.EvseId = evseId
		if h.database != nil {
			// Update database with EVSE ID
			return h.database.UpdateConnector(connector)
		}
	}
	return nil
}

func (h *SystemHandler) authorizeIdTag(chargePointId, idTag string) types.AuthorizationStatus {
	h.mux.Lock()
	defer h.mux.Unlock()
	state, ok := h.getChargePoint(chargePointId)
	if !ok {
		return types.AuthorizationStatusBlocked
	}
	if !state.model.IsEnabled {
		return types.AuthorizationStatusBlocked
	}

	authStatus := types.AuthorizationStatusInvalid
	userTag := h.getUserTag(idTag)
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
	}
	go h.notifyEventListeners(internal.Authorize, eventMessage)

	return authStatus
}
