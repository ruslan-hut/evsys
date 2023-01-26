package core

import (
	"evsys/handlers"
	"evsys/ocpp"
	"evsys/types"
	"evsys/utility"
	"fmt"
)

type CentralSystem struct {
	server            *Server
	coreHandler       handlers.SystemHandler
	supportedProtocol []string
}

func (cs *CentralSystem) SetCoreHandler(handler handlers.SystemHandler) {
	cs.coreHandler = handler
}

func (cs *CentralSystem) handleIncomingRequest(ws *WebSocket, data []byte) error {
	chargePointId := ws.ID()
	message, err := utility.ParseJson(data)
	if err != nil {
		return err
	}
	request, err := ParseRequest(message)
	if err != nil {
		return err
	}
	callRequest := request.(*CallRequest)
	ws.SetUniqueId(callRequest.UniqueId)

	action := request.GetFeatureName()
	var confirmation Response
	switch action {
	case ocpp.BootNotificationFeatureName:
		bootRequest := callRequest.Payload.(*ocpp.BootNotificationRequest)
		confirmation, err = cs.coreHandler.OnBootNotification(chargePointId, bootRequest)
	case ocpp.AuthorizeFeatureName:
		authRequest := callRequest.Payload.(*ocpp.AuthorizeRequest)
		confirmation, err = cs.coreHandler.OnAuthorize(chargePointId, authRequest)
	case ocpp.HeartbeatFeatureName:
		heartbeatRequest := callRequest.Payload.(*ocpp.HeartbeatRequest)
		confirmation, err = cs.coreHandler.OnHeartbeat(chargePointId, heartbeatRequest)
	case ocpp.StartTransactionFeatureName:
		startRequest := callRequest.Payload.(*ocpp.StartTransactionRequest)
		confirmation, err = cs.coreHandler.OnStartTransaction(chargePointId, startRequest)
	case ocpp.StopTransactionFeatureName:
		stopRequest := callRequest.Payload.(*ocpp.StopTransactionRequest)
		confirmation, err = cs.coreHandler.OnStopTransaction(chargePointId, stopRequest)
	case ocpp.MeterValuesFeatureName:
		meterRequest := callRequest.Payload.(*ocpp.MeterValuesRequest)
		confirmation, err = cs.coreHandler.OnMeterValues(chargePointId, meterRequest)
	default:
		err = utility.Err(fmt.Sprintf("feature not supported: %s", action))
	}
	if err != nil {
		return err
	}

	err = cs.server.SendResponse(ws, &confirmation)
	return err
}

func (cs *CentralSystem) Start() error {
	err := cs.server.Start()

	return err
}

func NewCentralSystem() CentralSystem {
	cs := CentralSystem{}
	wsServer := NewServer()
	wsServer.AddSupportedSupProtocol(types.SubProtocol16)
	wsServer.SetMessageHandler(cs.handleIncomingRequest)
	cs.server = wsServer
	cs.SetCoreHandler(ocpp.NewSystemHandler())
	return cs
}
