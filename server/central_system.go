package server

import (
	"evsys/billing"
	"evsys/internal"
	"evsys/internal/config"
	"evsys/internal/errorlistener"
	"evsys/ocpi"
	"evsys/ocpp"
	"evsys/ocpp/common"
	"evsys/ocpp/v16"
	"evsys/ocpp/v16/core"
	"evsys/ocpp/v16/firmware"
	"evsys/ocpp/v16/localauth"
	"evsys/ocpp/v16/remotetrigger"
	"evsys/ocpp/v16/smartcharging"
	"evsys/power"
	"evsys/telegram"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"log"
	"net/http"
	"sync"
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
	powerManager      PowerManager
	location          *time.Location
	supportedProtocol []string
	pendingRequests   map[string]chan string
	connections       sync.Map               // chargePointId → common.ProtocolVersion
	featureRegistry   common.FeatureRegistry // Registry for all OCPP features
	routingEnabled    bool                   // Flag to enable new routing (default: false for backward compatibility)
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

// GetProtocolVersion returns the protocol version for a connected charge point
// Returns UnknownVersion if the charge point is not connected or version is not tracked
func (cs *CentralSystem) GetProtocolVersion(chargePointId string) common.ProtocolVersion {
	if value, ok := cs.connections.Load(chargePointId); ok {
		if protocol, ok := value.(common.ProtocolVersion); ok {
			return protocol
		}
	}
	return common.UnknownVersion
}

// RemoveConnection removes the protocol version tracking for a disconnected charge point
func (cs *CentralSystem) RemoveConnection(chargePointId string) {
	cs.connections.Delete(chargePointId)
}

func (cs *CentralSystem) handleIncomingMessage(ws ocpp.WebSocket, data []byte) error {
	chargePointId := ws.ID()

	// Track protocol version for this connection
	protocol := ws.GetProtocol()
	cs.connections.Store(chargePointId, protocol)

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

	// Version-aware message parsing
	if cs.routingEnabled && cs.featureRegistry != nil {
		return cs.handleIncomingMessageVersionAware(ws, message, protocol, chargePointId)
	}

	// Legacy routing (backward compatibility)
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

	// call the power manager to check the power limit
	switch action {
	case core.StartTransactionFeatureName:
		go cs.powerManager.CheckPowerLimit(chargePointId)
	case core.StopTransactionFeatureName:
		go cs.powerManager.CheckPowerLimit(chargePointId)
	case core.BootNotificationFeatureName:
		cs.powerManager.OnChargePointBoot(chargePointId)
	}

	return err
}

// handleIncomingMessageVersionAware handles incoming messages using the version-aware registry-based routing
func (cs *CentralSystem) handleIncomingMessageVersionAware(ws ocpp.WebSocket, message []interface{}, protocol common.ProtocolVersion, chargePointId string) error {
	// Parse the call request structure
	callRequest, err := ParseRequestVersionAware(message, protocol, cs.featureRegistry)
	if err != nil {
		return fmt.Errorf("failed to parse request: %w", err)
	}
	ws.SetUniqueId(callRequest.UniqueId)

	request := callRequest.Payload
	action := request.GetFeatureName()

	cs.logger.Debug(fmt.Sprintf("handling %s from %s (protocol: %s)", action, chargePointId, protocol))

	// Route to appropriate handler based on protocol version
	var confirmation ocpp.Response
	switch protocol {
	case common.OCPP16:
		confirmation, err = cs.routeOCPP16Request(chargePointId, action, request)
	case common.OCPP201:
		// TODO: Implement OCPP 2.0.1 routing in Phase 2
		return fmt.Errorf("OCPP 2.0.1 not yet implemented")
	case common.OCPP21:
		// TODO: Implement OCPP 2.1 routing in Phase 4
		return fmt.Errorf("OCPP 2.1 not yet implemented")
	default:
		return fmt.Errorf("unsupported protocol version: %s", protocol)
	}

	if err != nil {
		return err
	}

	if ws.IsClosed() {
		cs.logger.FeatureEvent(action, chargePointId, "websocket closed, response not sent")
		return nil
	}

	err = cs.server.SendResponse(ws, confirmation)

	// Call the power manager to check the power limit (version-agnostic)
	switch action {
	case core.StartTransactionFeatureName:
		go cs.powerManager.CheckPowerLimit(chargePointId)
	case core.StopTransactionFeatureName:
		go cs.powerManager.CheckPowerLimit(chargePointId)
	case core.BootNotificationFeatureName:
		cs.powerManager.OnChargePointBoot(chargePointId)
	}

	return err
}

// routeOCPP16Request routes OCPP 1.6J requests to the appropriate handler
func (cs *CentralSystem) routeOCPP16Request(chargePointId string, action string, request ocpp.Request) (ocpp.Response, error) {
	switch action {
	case core.BootNotificationFeatureName:
		return cs.coreHandler.OnBootNotification(chargePointId, request.(*core.BootNotificationRequest))
	case core.AuthorizeFeatureName:
		return cs.coreHandler.OnAuthorize(chargePointId, request.(*core.AuthorizeRequest))
	case core.HeartbeatFeatureName:
		return cs.coreHandler.OnHeartbeat(chargePointId, request.(*core.HeartbeatRequest))
	case core.StartTransactionFeatureName:
		return cs.coreHandler.OnStartTransaction(chargePointId, request.(*core.StartTransactionRequest))
	case core.StopTransactionFeatureName:
		return cs.coreHandler.OnStopTransaction(chargePointId, request.(*core.StopTransactionRequest))
	case core.MeterValuesFeatureName:
		return cs.coreHandler.OnMeterValues(chargePointId, request.(*core.MeterValuesRequest))
	case core.StatusNotificationFeatureName:
		return cs.coreHandler.OnStatusNotification(chargePointId, request.(*core.StatusNotificationRequest))
	case core.DataTransferFeatureName:
		return cs.coreHandler.OnDataTransfer(chargePointId, request.(*core.DataTransferRequest))
	case firmware.DiagnosticsStatusNotificationFeatureName:
		return cs.firmwareHandler.OnDiagnosticsStatusNotification(chargePointId, request.(*firmware.DiagnosticsStatusNotificationRequest))
	case firmware.StatusNotificationFeatureName:
		return cs.firmwareHandler.OnFirmwareStatusNotification(chargePointId, request.(*firmware.StatusNotificationRequest))
	default:
		return nil, fmt.Errorf("feature not supported for OCPP 1.6: %s", action)
	}
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
	case smartcharging.ClearChargingProfileFeatureName:
		request, err = cs.coreHandler.OnClearChargingProfile(command.ChargePointId, command.Payload)
	case firmware.GetDiagnosticsFeatureName:
		request, err = cs.coreHandler.OnGetDiagnostics(command.ChargePointId, command.Payload)
	case "GetServerStatus":
		_, err = w.Write(cs.server.GetStatus())
		return err
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
			_, err = w.Write([]byte(payload))
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

// EnableVersionAwareRouting enables the new registry-based routing system
// This should be called after initialization but before Start() to use the new routing
func (cs *CentralSystem) EnableVersionAwareRouting() {
	cs.routingEnabled = true
	// Initialize OCPP 1.6 handler to register all features
	_ = v16.NewHandler16()
	log.Println("version-aware routing enabled - using feature registry")
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

	go cs.powerManager.OnSystemStart()

	select {}
}

func NewCentralSystem(conf *config.Config) (*CentralSystem, error) {
	cs := &CentralSystem{}
	cs.pendingRequests = make(map[string]chan string)
	cs.featureRegistry = common.GetGlobalRegistry()
	cs.routingEnabled = false // Default: use legacy routing for backward compatibility

	log.Println("set time zone to " + conf.TimeZone)
	location, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		return cs, fmt.Errorf("time zone initialization failed: %s", err)
	}
	cs.location = location
	var database *internal.MongoDB

	if conf.Mongo.Enabled {
		database, err = internal.NewMongoClient(conf)
		if err != nil {
			return cs, fmt.Errorf("mongodb setup failed: %s", err)
		}
		if database != nil {
			log.Println("mongodb is configured and enabled")
		}
	} else {
		database = nil
		log.Println("database is disabled")
	}

	// logger with database and push service for the message handling
	logService := internal.NewLogger(location)
	logService.SetDebugMode(conf.IsDebug)
	if database != nil {
		logService.SetDatabase(database)
	}

	cs.logger = logService

	// billing
	affleck := billing.NewAffleck()
	affleck.SetLogger(logService)
	if database != nil {
		affleck.SetDatabase(database)
	}

	// system events handler
	systemHandler := NewSystemHandler(location)
	if database != nil {
		systemHandler.SetDatabase(database)
	}
	systemHandler.SetBillingService(affleck)
	systemHandler.SetLogger(logService)
	systemHandler.SetParameters(conf.IsDebug, conf.AcceptUnknownTag, conf.AcceptUnknownChp)

	// payment service
	if conf.Payment.Enabled {
		payment := billing.NewPaymentService(conf)
		payment.SetLogger(logService)
		if database != nil {
			payment.SetDatabase(database)
		}
		systemHandler.SetPaymentService(payment)
	}

	if conf.Telegram.Enabled {
		telegramBot, e := telegram.NewBot(conf.Telegram.ApiKey)
		if e != nil {
			return cs, fmt.Errorf("telegram bot setup failed: %s", e)
		} else {
			if database != nil {
				telegramBot.SetDatabase(database)
			}
			telegramBot.Start()
			systemHandler.AddEventListener(telegramBot)
			log.Println("telegram bot is configured and enabled")
		}
	}

	if conf.Ocpi.Enabled {
		ocpiClient := ocpi.New(conf.Ocpi.Url, conf.Ocpi.Token)
		systemHandler.AddEventListener(ocpiClient)
		systemHandler.SetAuthService(ocpiClient)
		log.Println("ocpi client is configured and enabled")
	}

	// error listener for system handler
	if database != nil {
		errorListener := errorlistener.NewErrorListener(database, logService)
		systemHandler.SetErrorListener(errorListener)
	}

	// websocket listener
	wsServer := NewServer(conf, logService)
	wsServer.AddSupportedSupProtocol(types.SubProtocol16)
	wsServer.SetMessageHandler(cs.handleIncomingMessage)
	wsServer.SetWatchdog(systemHandler)

	cs.server = wsServer

	// power manager
	cs.powerManager = power.NewLoadBalancer(database, wsServer, logService)

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
