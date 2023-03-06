package server

import (
	"evsys/api"
	"evsys/internal"
	"evsys/internal/config"
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
	if conf.IsDebug {
		log.Println("debug mode is enabled")
	}

	if conf.Mongo.Enabled {
		database, err := internal.NewMongoClient(conf)
		if err != nil {
			return cs, utility.Err(fmt.Sprintf("mongodb setup failed: %s", err))
		}
		if database != nil {
			log.Println("mongodb is configured and enabled")
		}
		cs.database = database
	} else {
		log.Println("database is disabled")
	}

	var messageService internal.MessageService
	if conf.Pusher.Enabled {
		messageService, err = pusher.NewPusher(conf)
		if conf.Pusher.Enabled && err != nil {
			return cs, utility.Err(fmt.Sprintf("pusher setup failed: %s", err))
		}
		if messageService != nil {
			log.Println("pusher service is configured and enabled")
		}
	} else {
		log.Println("message pushing service service is disabled")
	}

	// logger with database and push service for the message handling
	logService := internal.NewLogger()
	logService.SetDebugMode(conf.IsDebug)
	logService.SetDatabase(cs.database)
	logService.SetMessageService(messageService)

	// websocket listener
	wsServer := NewServer(conf)
	wsServer.AddSupportedSupProtocol(types.SubProtocol16)
	wsServer.SetMessageHandler(cs.handleIncomingRequest)
	wsServer.SetLogger(logService)

	// handler for api requests
	apiHandler := api.NewApiHandler()
	apiHandler.SetLogger(logService)
	apiHandler.SetDatabase(cs.database)
	wsServer.SetApiHandler(apiHandler.HandleApiCall)

	cs.server = wsServer

	// message handler
	systemHandler := core.NewSystemHandler()
	systemHandler.SetLogger(logService)
	systemHandler.SetDebugMode(conf.IsDebug)

	cs.SetCoreHandler(systemHandler)
	cs.SetFirmwareHandler(systemHandler)
	return cs, nil
}
