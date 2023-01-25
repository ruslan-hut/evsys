package ocpp

import (
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
	//status             core.ChargePointStatus
	currentTransaction int
}

type ChargePointState struct {
	//status            core.ChargePointStatus
	//diagnosticsStatus firmware.DiagnosticsStatus
	//firmwareStatus    firmware.FirmwareStatus
	connectors   map[int]*ConnectorInfo // No assumptions about the # of connectors
	transactions map[int]*TransactionInfo
	//errorCode         core.ChargePointErrorCode
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
	log.Printf("boot confirmed: ID %s; Serial number: %s", chargePointId, request.ChargePointSerialNumber)
	return NewBootNotificationResponse(types.NewDateTime(time.Now()), defaultHeartbeatInterval, RegistrationStatusAccepted), nil
}

func (h *SystemHandler) OnAuthorize(chargePointId string, request *AuthorizeRequest) (confirmation *AuthorizeResponse, err error) {
	_, ok := h.chargePoints[chargePointId]
	if !ok {
		h.addChargePoint(chargePointId)
	}
	log.Printf("authorization accepted: ID %s", chargePointId)
	return NewAuthorizationResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted)), nil
}

func (h *SystemHandler) OnHeartbeat(chargePointId string, request *HeartbeatRequest) (confirmation *HeartbeatResponse, err error) {
	log.Printf("received heartbeat: ID %s", chargePointId)
	return NewHeartbeatResponse(types.NewDateTime(time.Now())), nil
}

func (h *SystemHandler) OnStartTransaction(chargePointId string, request *StartTransactionRequest) (confirmation *StartTransactionResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charging point %s", chargePointId)
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

	log.Printf("ID %s started transaction #%v for connector %v", chargePointId, transaction.id, transaction.connectorId)
	return NewStartTransactionResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted), transaction.id), nil
}

func (h *SystemHandler) OnStopTransaction(chargePointId string, request *StopTransactionRequest) (confirmation *StopTransactionResponse, err error) {
	state, ok := h.chargePoints[chargePointId]
	if !ok {
		return nil, fmt.Errorf("unknown charging point %s", chargePointId)
	}
	transaction, ok := state.transactions[request.TransactionId]
	if ok {
		connector := state.getConnector(transaction.connectorId)
		connector.currentTransaction = -1
		transaction.endTime = request.Timestamp
		transaction.endMeter = request.MeterStop
		//TODO: bill clients
	}
	log.Printf("ID %s stopped transaction %v - %v", chargePointId, request.TransactionId, request.Reason)
	for _, mv := range request.TransactionData {
		log.Printf("%v", mv)
	}
	return NewStopTransactionResponse(), nil
}
