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
	default:
		err = utility.Err(fmt.Sprintf("feature not supported: %s", action))
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
	cs.SetCoreHandler(&ocpp.MessageHandler{})
	return cs
}
