package server

import (
	"evsys/billing"
	"evsys/internal"
	"evsys/internal/config"
	"evsys/ocpp"
	"evsys/ocpp/core"
	"evsys/ocpp/firmware"
	"evsys/ocpp/localauth"
	"evsys/ocpp/remotetrigger"
	"evsys/ocpp/smartcharging"
	"evsys/pusher"
	"evsys/telegram"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"log"
	"net/http"
	"time"
)

type CentralSystem struct {
	server            *Server
	api               *Api
	logger            internal.LogHandler
	coreHandler       *SystemHandler
	firmwareHandler   firmware.SystemHandler
	remoteTrigger     remotetrigger.SystemHandler
	localAuth         localauth.SystemHandler
	location          *time.Location
	supportedProtocol []string
	pendingRequests   map[string]chan string
}

type CentralSystemCommand struct {
	ChargePointId string `json:"charge_point_id"`
	ConnectorId   int    `json:"connector_id"`
	FeatureName   string `json:"feature_name"`
	Payload       string `json:"payload"`
}

func (cs *CentralSystem) SetCoreHandler(handler *SystemHandler) {
	cs.coreHandler = handler
}

func (cs *CentralSystem) SetFirmwareHandler(handler firmware.SystemHandler) {
	cs.firmwareHandler = handler
}

func (cs *CentralSystem) SetRemoteTriggerHandler(handler remotetrigger.SystemHandler) {
	cs.remoteTrigger = handler
}

func (cs *CentralSystem) SetLocalAuthHandler(handler localauth.SystemHandler) {
	cs.localAuth = handler
}

func (cs *CentralSystem) handleIncomingMessage(ws ocpp.WebSocket, data []byte) error {
	chargePointId := ws.ID()
	message, err := utility.ParseJson(data)
	if err != nil {
		return err
	}
	callType, err := MessageType(message)
	if err != nil {
		return err
	}
	if callType == CallTypeError {
		cs.logger.Warn(fmt.Sprintf("error message received from charge point %s: %s", chargePointId, string(data)))
		return nil
	}
	if callType == CallTypeResult {
		result, err := ParseResultUnchecked(message)
		if err != nil {
			cs.logger.Warn(fmt.Sprintf("invalid message received from charge point %s: %s", chargePointId, string(data)))
			return nil
		}
		if responseChan, ok := cs.pendingRequests[result.UniqueId]; ok {
			responseChan <- result.Payload
		}
		return nil
	}
	callRequest, err := ParseRequest(message)
	if err != nil {
		return err
	}
	ws.SetUniqueId(callRequest.UniqueId)

	request := callRequest.Payload
	action := request.GetFeatureName()
	var confirmation ocpp.Response
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
		err = fmt.Errorf("feature not supported: %s", action)
	}
	if err != nil {
		return err
	}

	if ws.IsClosed() {
		cs.logger.FeatureEvent(action, chargePointId, "websocket closed, response not sent")
		return nil
	}
	err = cs.server.SendResponse(ws, confirmation)
	return err
}

func (cs *CentralSystem) handleApiRequest(w http.ResponseWriter, command CentralSystemCommand) error {
	if command.FeatureName == "" {
		return fmt.Errorf("feature name is empty")
	}
	var request ocpp.Request
	var err error
	switch command.FeatureName {
	case remotetrigger.TriggerMessageFeatureName:
		request, err = cs.remoteTrigger.OnTriggerMessage(command.ChargePointId, command.ConnectorId, command.Payload)
	case localauth.SendLocalListFeatureName:
		request, err = cs.localAuth.OnSendLocalList(command.ChargePointId)
	case core.RemoteStartTransactionFeatureName:
		request, err = cs.coreHandler.OnRemoteStartTransaction(command.ChargePointId, command.ConnectorId, command.Payload)
	case core.RemoteStopTransactionFeatureName:
		request, err = cs.coreHandler.OnRemoteStopTransaction(command.ChargePointId, command.Payload)
	case core.GetConfigurationFeatureName:
		request, err = cs.coreHandler.OnGetConfiguration(command.ChargePointId, command.Payload)
	case core.ChangeConfigurationFeatureName:
		request, err = cs.coreHandler.OnChangeConfiguration(command.ChargePointId, command.Payload)
	case core.ResetFeatureName:
		request, err = cs.coreHandler.OnReset(command.ChargePointId, command.Payload)
	case smartcharging.SetChargingProfileFeatureName:
		request, err = cs.coreHandler.OnSetChargingProfile(command.ChargePointId, command.ConnectorId, command.Payload)
	case smartcharging.GetCompositeScheduleFeatureName:
		request, err = cs.coreHandler.OnGetCompositeSchedule(command.ChargePointId, command.ConnectorId, command.Payload)
	case firmware.GetDiagnosticsFeatureName:
		request, err = cs.coreHandler.OnGetDiagnostics(command.ChargePointId, command.Payload)
	default:
		err = fmt.Errorf("feature not supported: %s", command.FeatureName)
	}
	if err != nil {
		return err
	}

	id, err := cs.server.SendRequest(command.ChargePointId, request)
	if err != nil {
		return err
	}
	response := make(chan string)
	cs.pendingRequests[id] = response

	select {
	case payload := <-response:
		if payload == "" {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.Header().Add("Content-Type", "application/json; charset=utf-8")
			_, err := w.Write([]byte(payload))
			if err != nil {
				cs.logger.Error("cs command send response", err)
			}
		}
	case <-time.After(10 * time.Second):
		cs.logger.Warn(fmt.Sprintf("timeout waiting for response from %s", command.ChargePointId))
		w.WriteHeader(http.StatusNoContent)
	}
	delete(cs.pendingRequests, id)

	return nil
}

func (cs *CentralSystem) Start() {

	go func() {
		if err := cs.server.Start(); err != nil {
			cs.logger.Error("websocket server failed", err)
		}
	}()

	go func() {
		if err := cs.api.Start(); err != nil {
			cs.logger.Error("api server failed", err)
		}
	}()

	select {}
}

func NewCentralSystem(conf *config.Config) (CentralSystem, error) {
	cs := CentralSystem{}
	cs.pendingRequests = make(map[string]chan string)

	log.Println("set time zone to " + conf.TimeZone)
	location, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		return cs, fmt.Errorf("time zone initialization failed: %s", err)
	}
	cs.location = location
	var database internal.Database

	if conf.Mongo.Enabled {
		database, err = internal.NewMongoClient(conf)
		if err != nil {
			return cs, fmt.Errorf("mongodb setup failed: %s", err)
		}
		if database != nil {
			log.Println("mongodb is configured and enabled")
		}
	} else {
		log.Println("database is disabled")
	}

	var messageService internal.MessageService
	if conf.Pusher.Enabled {
		messageService, err = pusher.NewPusher(conf)
		if conf.Pusher.Enabled && err != nil {
			return cs, fmt.Errorf("pusher setup failed: %s", err)
		}
		if messageService != nil {
			log.Println("pusher service is configured and enabled")
		}
	} else {
		log.Println("message pushing service service is disabled")
	}

	// logger with database and push service for the message handling
	logService := internal.NewLogger(location)
	logService.SetDebugMode(conf.IsDebug)
	logService.SetDatabase(database)
	logService.SetMessageService(messageService)

	cs.logger = logService

	// billing
	affleck := billing.NewAffleck()
	affleck.SetDatabase(database)
	affleck.SetLogger(logService)

	// system events handler
	systemHandler := NewSystemHandler(location)
	systemHandler.SetDatabase(database)
	systemHandler.SetBillingService(affleck)
	systemHandler.SetLogger(logService)
	systemHandler.SetParameters(conf.IsDebug, conf.AcceptUnknownTag, conf.AcceptUnknownChp)

	// payment service
	if conf.Payment.Enabled {
		payment := billing.NewPaymentService(conf)
		payment.SetDatabase(database)
		payment.SetLogger(logService)
		systemHandler.SetPaymentService(payment)
	}

	if conf.Telegram.Enabled {
		telegramBot, err := telegram.NewBot(conf.Telegram.ApiKey)
		if err != nil {
			return cs, fmt.Errorf("telegram bot setup failed: %s", err)
		} else {
			telegramBot.SetDatabase(database)
			telegramBot.Start()
			systemHandler.AddEventListener(telegramBot)
			log.Println("telegram bot is configured and enabled")
		}
	}

	// websocket listener
	wsServer := NewServer(conf, logService)
	wsServer.AddSupportedSupProtocol(types.SubProtocol16)
	wsServer.SetMessageHandler(cs.handleIncomingMessage)
	wsServer.SetWatchdog(systemHandler)

	cs.server = wsServer

	trigger := NewTrigger(wsServer, logService)
	systemHandler.SetTrigger(trigger)

	err = systemHandler.OnStart()
	if err != nil {
		return cs, err
	}

	cs.SetCoreHandler(systemHandler)
	cs.SetFirmwareHandler(systemHandler)
	cs.SetRemoteTriggerHandler(systemHandler)
	cs.SetLocalAuthHandler(systemHandler)

	// api server
	apiServer := NewServerApi(conf, logService)
	apiServer.SetRequestHandler(cs.handleApiRequest)
	cs.api = apiServer

	return cs, nil
}
