package core

import (
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
}

func NewSystemHandler() *SystemHandler {
	handler := SystemHandler{
		chargePoints: make(map[string]*ChargePointState),
	}
	return &handler
}

func (h *SystemHandler) addChargePoint(chargePointId string) {
	h.chargePoints[chargePointId] = &ChargePointState{
		connectors:   make(map[int]*ConnectorInfo),
		transactions: make(map[int]*TransactionInfo),
	}
}

func (h *SystemHandler) OnBootNotification(chargePointId string, request *BootNotificationRequest) (confirmation *BootNotificationResponse, err error) {
	log.Printf("[%s] boot confirmed (serial number: %s)", chargePointId, request.ChargePointSerialNumber)
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
	log.Printf("[%s] authorization accepted for %s", chargePointId, id)
	return NewAuthorizationResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted)), nil
}

func (h *SystemHandler) OnHeartbeat(chargePointId string, request *HeartbeatRequest) (confirmation *HeartbeatResponse, err error) {
	log.Printf("[%s] %s", chargePointId, request.GetFeatureName())
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

	log.Printf("[%s] started transaction #%v for connector %v", chargePointId, transaction.id, transaction.connectorId)
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
	log.Printf("[%s] stopped transaction %v %v", chargePointId, request.TransactionId, request.Reason)
	for _, mv := range request.TransactionData {
		log.Printf("%v", mv)
	}
	return NewStopTransactionResponse(), nil
}

func (h *SystemHandler) OnMeterValues(chargePointId string, request *MeterValuesRequest) (confirmation *MeterValuesResponse, err error) {
	log.Printf("[%s] recieved meter values for connector #%v", chargePointId, request.ConnectorId)
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
		log.Printf("[%s] updated connector #%v status to %v", chargePointId, request.ConnectorId, request.Status)
	} else {
		state.status = request.Status
		log.Printf("[%s] updated main controller status to %v", chargePointId, request.Status)
	}
	return NewStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnDataTransfer(chargePointId string, request *DataTransferRequest) (confirmation *DataTransferResponse, err error) {
	log.Printf("[%s] recieved data #%v", chargePointId, request.Data)
	return NewDataTransferResponse(DataTransferStatusAccepted), nil
}

func (h *SystemHandler) OnDiagnosticsStatusNotification(chargePointId string, request *firmware.DiagnosticsStatusNotificationRequest) (confirmation *firmware.DiagnosticsStatusNotificationResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	state.diagnosticsStatus = request.Status
	log.Printf("[%s] updated diagnostic status to %v", chargePointId, request.Status)
	return firmware.NewDiagnosticsStatusNotificationResponse(), nil
}

func (h *SystemHandler) OnFirmwareStatusNotification(chargePointId string, request *firmware.StatusNotificationRequest) (confirmation *firmware.StatusNotificationResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("%v; unknown charging point: %s", request.GetFeatureName(), chargePointId)
	}
	state.firmwareStatus = request.Status
	log.Printf("[%s] updated firmware status to %v", chargePointId, request.Status)
	return firmware.NewStatusNotificationResponse(), nil
}
