package server

import (
	"evsys/internal"
	"evsys/internal/config"
	"evsys/logger"
	"evsys/mongodb"
	"evsys/ocpp/core"
	"evsys/ocpp/firmware"
	"evsys/ocpp/handlers"
	"evsys/pusher"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"log"
)

type CentralSystem struct {
	server            *Server
	database          internal.Database
	coreHandler       handlers.SystemHandler
	firmwareHandler   firmware.SystemHandler
	supportedProtocol []string
}

func (cs *CentralSystem) SetCoreHandler(handler handlers.SystemHandler) {
	cs.coreHandler = handler
}

func (cs *CentralSystem) SetFirmwareHandler(handler firmware.SystemHandler) {
	cs.firmwareHandler = handler
}

func (cs *CentralSystem) handleIncomingRequest(ws *WebSocket, data []byte) error {
	chargePointId := ws.ID()
	message, err := utility.ParseJson(data)
	if err != nil {
		return err
	}
	callRequest, err := ParseRequest(message)
	if err != nil {
		return err
	}
	ws.SetUniqueId(callRequest.UniqueId)

	request := callRequest.Payload
	action := request.GetFeatureName()
	var confirmation Response
	switch action {
	case core.BootNotificationFeatureName:
		confirmation, err = cs.coreHandler.OnBootNotification(chargePointId, request.(*core.BootNotificationRequest))
	case core.AuthorizeFeatureName:
		confirmation, err = cs.coreHandler.OnAuthorize(chargePointId, request.(*core.AuthorizeRequest))
	case core.HeartbeatFeatureName:
		confirmation, err = cs.coreHandler.OnHeartbeat(chargePointId, request.(*core.HeartbeatRequest))
	case core.StartTransactionFeatureName:
		confirmation, err = cs.coreHandler.OnStartTransaction(chargePointId, request.(*core.StartTransactionRequest))
	case core.StopTransactionFeatureName:
		confirmation, err = cs.coreHandler.OnStopTransaction(chargePointId, request.(*core.StopTransactionRequest))
	case core.MeterValuesFeatureName:
		confirmation, err = cs.coreHandler.OnMeterValues(chargePointId, request.(*core.MeterValuesRequest))
	case core.StatusNotificationFeatureName:
		confirmation, err = cs.coreHandler.OnStatusNotification(chargePointId, request.(*core.StatusNotificationRequest))
	case core.DataTransferFeatureName:
		confirmation, err = cs.coreHandler.OnDataTransfer(chargePointId, request.(*core.DataTransferRequest))
	case firmware.DiagnosticsStatusNotificationFeatureName:
		confirmation, err = cs.firmwareHandler.OnDiagnosticsStatusNotification(chargePointId, request.(*firmware.DiagnosticsStatusNotificationRequest))
	case firmware.StatusNotificationFeatureName:
		confirmation, err = cs.firmwareHandler.OnFirmwareStatusNotification(chargePointId, request.(*firmware.StatusNotificationRequest))
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

func NewCentralSystem() (CentralSystem, error) {
	cs := CentralSystem{}

	conf, err := config.GetConfig()
	if err != nil {
		return cs, utility.Err(fmt.Sprintf("loading configuration failed: %s", err))
	}

	database, err := mongodb.NewMongoClient(conf)
	if conf.Mongo.Enabled && err != nil {
		return cs, utility.Err(fmt.Sprintf("mongodb setup failed: %s", err))
	}
	if database != nil {
		log.Println("mongodb is configured and enabled")
	} else {
		log.Println("mongodb is disabled")
	}
	cs.database = database

	// websocket listener
	wsServer := NewServer(conf)
	wsServer.AddSupportedSupProtocol(types.SubProtocol16)
	wsServer.SetMessageHandler(cs.handleIncomingRequest)
	cs.server = wsServer

	// message handler
	systemHandler := core.NewSystemHandler()

	// logger with push service for the message handler
	logService := logger.NewLogger()
	logService.SetDatabase(database)
	pusherService, err := pusher.NewPusher(conf)
	if conf.Pusher.Enabled && err != nil {
		return cs, utility.Err(fmt.Sprintf("pusher setup failed: %s", err))
	}
	if pusherService != nil {
		log.Println("pusher service is configured and enabled")
	} else {
		log.Println("pusher service is disabled")
	}
	logService.SetMessageService(pusherService)
	systemHandler.SetLogger(logService)

	cs.SetCoreHandler(systemHandler)
	cs.SetFirmwareHandler(systemHandler)
	return cs, nil
}
