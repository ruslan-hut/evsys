package core

import (
	"evsys/handlers"
	"evsys/ocpp/firmware"
	"evsys/types"
	"fmt"
	"log"
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
}

type ChargePointState struct {
	status            ChargePointStatus
	diagnosticsStatus firmware.DiagnosticsStatus
	firmwareStatus    firmware.Status
	connectors        map[int]*ConnectorInfo // No assumptions about the # of connectors
	transactions      map[int]*TransactionInfo
	errorCode         ChargePointErrorCode
}

func (cps *ChargePointState) getConnector(id int) *ConnectorInfo {
	ci, ok := cps.connectors[id]
	if !ok {
		ci = &ConnectorInfo{currentTransaction: -1}
		cps.connectors[id] = ci
	}
	return ci
}

type SystemHandler struct {
	chargePoints map[string]*ChargePointState
	logger       handlers.LogHandler
}

func NewSystemHandler() *SystemHandler {
	handler := SystemHandler{
		chargePoints: make(map[string]*ChargePointState),
	}
	return &handler
}

func (h *SystemHandler) SetLogger(logger handlers.LogHandler) {
	h.logger = logger
}

func (h *SystemHandler) addChargePoint(chargePointId string) {
	h.chargePoints[chargePointId] = &ChargePointState{
		connectors:   make(map[int]*ConnectorInfo),
		transactions: make(map[int]*TransactionInfo),
	}
}

func (h *SystemHandler) OnBootNotification(chargePointId string, request *BootNotificationRequest) (confirmation *BootNotificationResponse, err error) {
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("boot confirmed (serial number: %s)", request.ChargePointSerialNumber))
	return NewBootNotificationResponse(types.NewDateTime(time.Now()), defaultHeartbeatInterval, RegistrationStatusAccepted), nil
}

func (h *SystemHandler) OnAuthorize(chargePointId string, request *AuthorizeRequest) (confirmation *AuthorizeResponse, err error) {
	_, ok := h.chargePoints[chargePointId]
	if !ok {
		h.addChargePoint(chargePointId)
	}
	id := request.IdTag
	if id == "" {
		return nil, fmt.Errorf("%s cannot authorize empty id %s", request.GetFeatureName(), id)
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("authorization accepted for %s", id))
	return NewAuthorizationResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted)), nil
}

func (h *SystemHandler) OnHeartbeat(chargePointId string, request *HeartbeatRequest) (confirmation *HeartbeatResponse, err error) {
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("%v", time.Now()))
	return NewHeartbeatResponse(types.NewDateTime(time.Now())), nil
}

func (h *SystemHandler) OnStartTransaction(chargePointId string, request *StartTransactionRequest) (confirmation *StartTransactionResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	connector := state.getConnector(request.ConnectorId)
	if connector.currentTransaction >= 0 {
		return nil, fmt.Errorf("connector %v is now busy with another transaction", request.ConnectorId)
	}

	transaction := &TransactionInfo{}
	transaction.idTag = request.IdTag
	transaction.connectorId = request.ConnectorId
	transaction.startMeter = request.MeterStart
	transaction.startTime = request.Timestamp
	transaction.id = newTransactionId
	newTransactionId += 1

	connector.currentTransaction = transaction.id

	state.transactions[transaction.id] = transaction

	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("started transaction #%v for connector %v", transaction.id, transaction.connectorId))
	return NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted), transaction.id), nil
}

func (h *SystemHandler) OnStopTransaction(chargePointId string, request *StopTransactionRequest) (confirmation *StopTransactionResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	transaction, ok := state.transactions[request.TransactionId]
	if ok {
		connector := state.getConnector(transaction.connectorId)
		connector.currentTransaction = -1
		transaction.endTime = request.Timestamp
		transaction.endMeter = request.MeterStop
		//TODO: bill clients
	}
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("stopped transaction %v %v", request.TransactionId, request.Reason))
	for _, mv := range request.TransactionData {
		log.Printf("%v", mv)
	}
	return NewStopTransactionResponse(), nil
}

func (h *SystemHandler) OnMeterValues(chargePointId string, request *MeterValuesRequest) (confirmation *MeterValuesResponse, err error) {
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("recieved meter values for connector #%v", request.ConnectorId))
	for _, value := range request.MeterValue {
		log.Printf("[%s] -- %v", chargePointId, value)
	}
	return NewMeterValuesResponse(), nil
}

func (h *SystemHandler) OnStatusNotification(chargePointId string, request *StatusNotificationRequest) (confirmation *StatusNotificationResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	state.errorCode = request.ErrorCode
	if request.ConnectorId > 0 {
		connector := state.getConnector(request.ConnectorId)
		connector.status = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated connector #%v status to %v", request.ConnectorId, request.Status))
	} else {
		state.status = request.Status
		h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated main controller status to %v", request.Status))
	}
	return NewStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnDataTransfer(chargePointId string, request *DataTransferRequest) (confirmation *DataTransferResponse, err error) {
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("recieved data #%v", request.Data))
	return NewDataTransferResponse(DataTransferStatusAccepted), nil
}

func (h *SystemHandler) OnDiagnosticsStatusNotification(chargePointId string, request *firmware.DiagnosticsStatusNotificationRequest) (confirmation *firmware.DiagnosticsStatusNotificationResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	state.diagnosticsStatus = request.Status
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated diagnostic status to %v", request.Status))
	return firmware.NewDiagnosticsStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnFirmwareStatusNotification(chargePointId string, request *firmware.StatusNotificationRequest) (confirmation *firmware.StatusNotificationResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	state.firmwareStatus = request.Status
	h.logger.FeatureEvent(request.GetFeatureName(), chargePointId, fmt.Sprintf("updated firmware status to %v", request.Status))
	return firmware.NewStatusNotificationResponse(), nil
}
