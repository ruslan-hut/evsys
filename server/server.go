package server

import (
	"encoding/json"
	"evsys/internal"
	"evsys/internal/config"
	"evsys/ocpp"
	"evsys/utility"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"net"
	"net/http"
	"sync"
)

const (
	wsEndpoint           = "/ws/:id"
	featureNameWebSocket = "WebSocket"
)

type envelope struct {
	recipient   string
	callRequest *CallRequest
	callResult  *CallResult
}

func (e *envelope) getMessageData() ([]byte, error) {
	if e.callRequest != nil {
		return e.callRequest.MarshalJSON()
	}
	if e.callResult != nil {
		return e.callResult.MarshalJSON()
	}
	return nil, fmt.Errorf("envelope has no message data")
}

type Server struct {
	conf           *config.Config
	httpServer     *http.Server
	upgrader       websocket.Upgrader
	pool           *Pool
	messageHandler func(ws ocpp.WebSocket, data []byte) error
	logger         internal.LogHandler
	watchdog       internal.StatusHandler
}

type WebSocket struct {
	conn           *websocket.Conn
	send           chan []byte
	pool           *Pool
	id             string
	uniqueId       string
	messageHandler func(ws ocpp.WebSocket, data []byte) error
	logger         internal.LogHandler
	isClosed       bool
	watchdog       internal.StatusHandler
	mutex          *sync.Mutex
}

type Pool struct {
	register   chan *WebSocket
	unregister chan *WebSocket
	clients    map[*WebSocket]bool
	broadcast  chan []byte
	send       chan *envelope
	logger     internal.LogHandler
}

func NewPool(logger internal.LogHandler) *Pool {
	return &Pool{
		register:   make(chan *WebSocket),
		unregister: make(chan *WebSocket),
		clients:    make(map[*WebSocket]bool),
		send:       make(chan *envelope),
		broadcast:  make(chan []byte),
		logger:     logger,
	}
}

func (pool *Pool) Start() {
	for {
		select {
		case client := <-pool.register:
			pool.clients[client] = true
			pool.logger.FeatureEvent(featureNameWebSocket, client.id, fmt.Sprintf("registered new connection: total connections %v", len(pool.clients)))
		case client := <-pool.unregister:
			if _, ok := pool.clients[client]; ok {
				delete(pool.clients, client)
				close(client.send)
				pool.logger.FeatureEvent(featureNameWebSocket, client.id, fmt.Sprintf("unregistered: total connections %v", len(pool.clients)))
			}
		case message := <-pool.broadcast:
			for client := range pool.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(pool.clients, client)
				}
			}
		case env := <-pool.send:
			for client := range pool.clients {
				if client.id == env.recipient {
					data, err := env.getMessageData()
					if err != nil {
						pool.logger.Error("encode request:", err)
						break
					}
					select {
					case client.send <- data:
					default:
						close(client.send)
						delete(pool.clients, client)
					}
					break
				}
			}
		}
	}
}

func (pool *Pool) checkAddClient(client *WebSocket) {
	if !pool.recipientAvailable(client.id) {
		pool.register <- client
	}
	go client.watchdog.OnOnlineStatusChanged(client.id, true)
}

func (pool *Pool) recipientAvailable(clientId string) bool {
	for client := range pool.clients {
		if client.id == clientId {
			return true
		}
	}
	return false
}

func (ws *WebSocket) ID() string {
	return ws.id
}

func (ws *WebSocket) UniqueId() string {
	return ws.uniqueId
}

func (ws *WebSocket) SetUniqueId(uniqueId string) {
	ws.uniqueId = uniqueId
}

func (ws *WebSocket) IsClosed() bool {
	return ws.isClosed
}

func NewServer(conf *config.Config, logger internal.LogHandler) *Server {
	// initialize and start the pool for sending and receiving messages
	pool := NewPool(logger)
	go pool.Start()

	server := Server{
		conf:     conf,
		upgrader: websocket.Upgrader{Subprotocols: []string{}},
		pool:     pool,
		logger:   logger,
	}

	// register itself as a router for httpServer handler
	router := httprouter.New()
	server.Register(router)
	server.httpServer = &http.Server{
		Handler: router,
	}
	return &server
}

func (s *Server) AddSupportedSupProtocol(proto string) {
	for _, sub := range s.upgrader.Subprotocols {
		if sub == proto {
			return
		}
	}
	s.upgrader.Subprotocols = append(s.upgrader.Subprotocols, proto)
}

func (s *Server) SetMessageHandler(handler func(ws ocpp.WebSocket, data []byte) error) {
	s.messageHandler = handler
}

func (s *Server) SetWatchdog(handler internal.StatusHandler) {
	s.watchdog = handler
}

func (s *Server) Register(router *httprouter.Router) {
	router.GET(wsEndpoint, s.handleWsRequest)
}

func (s *Server) handleWsRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("id")
	//s.logger.Debug(fmt.Sprintf("connection initiated from remote %s", r.RemoteAddr))

	// check id above existed connections
	for client := range s.pool.clients {
		if client.id == id {
			s.logger.Debug(fmt.Sprintf("%s requested new connection", id))
			s.pool.unregister <- client
		}
	}

	s.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	clientSubProto := websocket.Subprotocols(r)
	requestedProto := ""
	for _, proto := range clientSubProto {
		if len(s.upgrader.Subprotocols) == 0 {
			// supporting all protocols
			requestedProto = proto
			break
		}
		if utility.Contains(s.upgrader.Subprotocols, proto) {
			requestedProto = proto
			break
		}
	}
	responseHeader := http.Header{}
	if requestedProto != "" {
		responseHeader.Add("Sec-WebSocket-Protocol", requestedProto)
	}

	conn, err := s.upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		s.logger.Error("upgrade failed: ", err)
		return
	}

	//s.logger.Debug(fmt.Sprintf("upgraded socket for %s and ready to receive data", id))
	ws := WebSocket{
		conn:           conn,
		id:             id,
		pool:           s.pool,
		send:           make(chan []byte, 256),
		logger:         s.logger,
		messageHandler: s.messageHandler,
		isClosed:       false,
		watchdog:       s.watchdog,
		mutex:          &sync.Mutex{},
	}
	s.pool.checkAddClient(&ws)

	go ws.readPump()
	go ws.writePump()
}

func (ws *WebSocket) readPump() {
	defer func() {
		ws.close()
	}()
	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, 3001) {
				//ws.logger.Debug(fmt.Sprintf("id %s leaving session", ws.id))
			} else {
				ws.logger.FeatureEvent(featureNameWebSocket, ws.id, fmt.Sprintf("read error: %s", err))
			}
			break
		}
		ws.logger.RawDataEvent("IN", string(message))
		if ws.messageHandler != nil {
			err = ws.messageHandler(ws, message)
			if err != nil {
				ws.logger.Error(fmt.Sprintf("handling message from %s", ws.id), err)
				continue
			}
		}
		ws.pool.checkAddClient(ws)
	}
}

func (ws *WebSocket) writePump() {
	defer func() {
		ws.close()
	}()
	for {
		select {
		case message, ok := <-ws.send:
			if !ok {
				//ws.logger.Debug(fmt.Sprintf("id %s leaving session", ws.id))
				_ = ws.writeMessage(websocket.CloseMessage, []byte{})
				break
			}
			ws.logger.RawDataEvent("OUT", string(message))

			err := ws.writeMessage(websocket.TextMessage, message)

			if err != nil {
				ws.logger.Warn(fmt.Sprintf("socket %s: %s", ws.id, err))
				break
			}
		}
	}
}

func (ws *WebSocket) writeMessage(messageType int, message []byte) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	if ws.isClosed {
		return fmt.Errorf("write cancelled, socket is closed")
	}
	return ws.conn.WriteMessage(messageType, message)
}

// close closing the websocket connection
func (ws *WebSocket) close() {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	go ws.watchdog.OnOnlineStatusChanged(ws.id, false)

	ws.pool.unregister <- ws
	if !ws.isClosed {
		ws.isClosed = true
		_ = ws.conn.Close()
	}
}

func (s *Server) Start() error {
	if s.conf == nil {
		return fmt.Errorf("configuration not loaded")
	}
	serverAddress := fmt.Sprintf("%s:%s", s.conf.Listen.BindIP, s.conf.Listen.Port)
	s.logger.Debug(fmt.Sprintf("starting server on %s", serverAddress))
	listener, err := net.Listen("tcp", serverAddress)
	if err != nil {
		return err
	}
	if s.conf.Listen.TLS {
		s.logger.Debug("starting https TLS server")
		err = s.httpServer.ServeTLS(listener, s.conf.Listen.CertFile, s.conf.Listen.KeyFile)
	} else {
		s.logger.Debug("starting http server")
		err = s.httpServer.Serve(listener)
	}
	return err
}

func (s *Server) SendResponse(ws ocpp.WebSocket, response ocpp.Response) error {
	callResult, _ := CreateCallResult(response, ws.UniqueId())
	env := &envelope{
		recipient:  ws.ID(),
		callResult: callResult,
	}
	s.pool.send <- env
	return nil
}

// SendRequest send request to the websocket and return the unique id of the request
func (s *Server) SendRequest(clientId string, request ocpp.Request) (string, error) {
	if !s.pool.recipientAvailable(clientId) {
		return "", fmt.Errorf("%s is not available", clientId)
	}
	callRequest, err := CreateCallRequest(request)
	if err != nil {
		return "", fmt.Errorf("error creating call request: %s", err)
	}
	env := &envelope{
		recipient:   clientId,
		callRequest: &callRequest,
	}
	s.pool.send <- env
	return callRequest.UniqueId, nil
}

type Status struct {
	ConnectedClients string `json:"connected_clients"`
	TotalClients     int    `json:"total_clients"`
}

func (s *Server) GetStatus() []byte {
	clientList := ""
	for client := range s.pool.clients {
		clientList += client.id + ","
	}
	// remove the last comma
	if len(clientList) > 0 {
		clientList = clientList[:len(clientList)-1]
	}
	status := &Status{
		ConnectedClients: clientList,
		TotalClients:     len(s.pool.clients),
	}
	data, err := json.Marshal(status)
	if err != nil {
		s.logger.Error("marshal status:", err)
		return []byte{}
	}
	return data
}
