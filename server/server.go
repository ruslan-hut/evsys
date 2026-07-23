package server

import (
	"context"
	"encoding/json"
	"errors"
	"evsys/internal"
	"evsys/internal/config"
	"evsys/ocpp"
	"evsys/ocpp/common"
	"evsys/utility"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

const (
	wsEndpoint           = "/ws/:id"
	featureNameWebSocket = "WebSocket"
)

// ErrResponseTimeout is returned when a charge point accepted a request but did
// not answer it within the caller's deadline.
var ErrResponseTimeout = errors.New("timeout waiting for response")

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
	// pending maps a request's unique id to the caller waiting for its
	// CallResult. Guarded by pendingMutex: entries are created on the caller's
	// goroutine and resolved on the connection's read pump.
	pending map[string]chan string
	// fireAndForget holds unique ids of requests sent without a waiting caller
	// (via SendRequest). Their CallResult is expected but discarded, so it must
	// not be logged as unmatched. Guarded by pendingMutex.
	fireAndForget map[string]struct{}
	pendingMutex  sync.Mutex
}

// maxFireAndForget bounds the fire-and-forget set so a charge point that drops
// off between the request and its answer cannot leak entries indefinitely.
const maxFireAndForget = 1024

type WebSocket struct {
	conn           *websocket.Conn
	send           chan []byte
	pool           *Pool
	id             string
	uniqueId       string
	protocol       common.ProtocolVersion
	messageHandler func(ws ocpp.WebSocket, data []byte) error
	logger         internal.LogHandler
	isClosed       bool
	watchdog       internal.StatusHandler
	mutex          sync.Mutex
}

type Pool struct {
	register   chan *WebSocket
	unregister chan *WebSocket
	clients    map[string]*WebSocket
	send       chan *envelope
	logger     internal.LogHandler
	mutex      sync.Mutex
	stop       chan struct{}
}

func NewPool(logger internal.LogHandler) *Pool {
	return &Pool{
		register:   make(chan *WebSocket),
		unregister: make(chan *WebSocket),
		clients:    make(map[string]*WebSocket),
		send:       make(chan *envelope, 256),
		logger:     logger,
		mutex:      sync.Mutex{},
		stop:       make(chan struct{}),
	}
}

func (pool *Pool) Start() {
	for {
		select {
		case <-pool.stop:
			pool.closeAllClients()
			return
		case client := <-pool.register:
			pool.checkAddClient(client)
		case client := <-pool.unregister:
			pool.deleteClient(client)
		case env := <-pool.send:
			if client, ok := pool.clients[env.recipient]; ok {
				data, err := env.getMessageData()
				if err != nil {
					pool.logger.Error("encode request:", err)
					break
				}
				select {
				case client.send <- data:
				default:
					close(client.send)
					delete(pool.clients, client.id)
				}
				break
			}
		}
	}
}

func (pool *Pool) Stop() {
	close(pool.stop)
}

func (pool *Pool) closeAllClients() {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	for _, client := range pool.clients {
		close(client.send)
	}
	pool.clients = make(map[string]*WebSocket)
	pool.logger.Debug("all websocket connections closed")
}

func (pool *Pool) checkAddClient(client *WebSocket) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	if !pool.recipientAvailable(client.id) {
		pool.clients[client.id] = client
		pool.logger.FeatureEvent(featureNameWebSocket, client.id, fmt.Sprintf("registered new connection: total connections %v", len(pool.clients)))
	}
	go client.watchdog.OnOnlineStatusChanged(client.id, true)
}

func (pool *Pool) recipientAvailable(clientId string) bool {
	for _, client := range pool.clients {
		if client.id == clientId {
			return true
		}
	}
	return false
}

// delete client from pool
func (pool *Pool) deleteClient(client *WebSocket) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	if _, ok := pool.clients[client.id]; ok {
		delete(pool.clients, client.id)
		pool.logger.FeatureEvent(featureNameWebSocket, client.id, fmt.Sprintf("unregistered: total connections %v", len(pool.clients)))
	}
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

func (ws *WebSocket) GetProtocol() common.ProtocolVersion {
	return ws.protocol
}

func (ws *WebSocket) SetProtocol(protocol common.ProtocolVersion) {
	ws.protocol = protocol
}

func NewServer(conf *config.Config, logger internal.LogHandler) *Server {
	// initialize and start the pool for sending and receiving messages
	pool := NewPool(logger)
	go pool.Start()

	server := Server{
		conf:          conf,
		upgrader:      websocket.Upgrader{Subprotocols: []string{}},
		pool:          pool,
		logger:        logger,
		pending:       make(map[string]chan string),
		fireAndForget: make(map[string]struct{}),
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
	for _, client := range s.pool.clients {
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

	// Parse the negotiated protocol version
	protocol := common.ParseProtocolVersion(requestedProto)
	if protocol == common.UnknownVersion {
		// Default to OCPP 1.6 for backward compatibility
		protocol = common.DefaultVersion()
		s.logger.Debug(fmt.Sprintf("unknown protocol '%s' for %s, defaulting to %s", requestedProto, id, protocol))
	}

	//s.logger.Debug(fmt.Sprintf("upgraded socket for %s and ready to receive data", id))
	ws := WebSocket{
		conn:           conn,
		id:             id,
		protocol:       protocol,
		pool:           s.pool,
		send:           make(chan []byte, 256),
		logger:         s.logger,
		messageHandler: s.messageHandler,
		isClosed:       false,
		watchdog:       s.watchdog,
		mutex:          sync.Mutex{},
	}
	s.pool.register <- &ws

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
		ws.pool.register <- ws
		ws.logger.RawDataEvent("IN", string(message))
		if ws.messageHandler != nil {
			err = ws.messageHandler(ws, message)
			if err != nil {
				ws.logger.Error(fmt.Sprintf("handling message from %s", ws.id), err)
				continue
			}
		}
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

// SendRequest send request to the websocket and return the unique id of the request.
// The charge point's answer is discarded; use SendRequestWithResponse or
// SendRequestSync when the answer matters.
func (s *Server) SendRequest(clientId string, request ocpp.Request) (string, error) {
	if !s.pool.recipientAvailable(clientId) {
		return "", fmt.Errorf("%s is not available", clientId)
	}
	callRequest, err := CreateCallRequest(request)
	if err != nil {
		return "", fmt.Errorf("creating call request: %s", err)
	}
	env := &envelope{
		recipient:   clientId,
		callRequest: &callRequest,
	}
	s.markFireAndForget(callRequest.UniqueId)
	s.pool.send <- env
	return callRequest.UniqueId, nil
}

// SendRequestWithResponse queues a request and returns the channel that will
// receive the charge point's raw CallResult payload. The error reports only
// whether the request could be queued, so a caller can distinguish an offline
// charge point from one that simply has not answered yet, and wait for the
// answer off its hot path.
//
// release must be called once the caller stops listening, otherwise the pending
// entry leaks.
func (s *Server) SendRequestWithResponse(clientId string, request ocpp.Request) (response <-chan string, release func(), err error) {
	if !s.pool.recipientAvailable(clientId) {
		return nil, nil, fmt.Errorf("%s is not available", clientId)
	}
	callRequest, err := CreateCallRequest(request)
	if err != nil {
		return nil, nil, fmt.Errorf("creating call request: %s", err)
	}
	// Registered before the request is queued: a charge point on a fast link can
	// answer before this function returns, and an answer that arrives with
	// nobody registered is dropped.
	channel := s.registerPending(callRequest.UniqueId)
	s.pool.send <- &envelope{
		recipient:   clientId,
		callRequest: &callRequest,
	}
	return channel, func() { s.releasePending(callRequest.UniqueId) }, nil
}

// SendRequestSync queues a request and blocks until the charge point answers,
// returning the raw CallResult payload. It reports ErrResponseTimeout if the
// answer does not arrive within timeout.
func (s *Server) SendRequestSync(clientId string, request ocpp.Request, timeout time.Duration) (string, error) {
	response, release, err := s.SendRequestWithResponse(clientId, request)
	if err != nil {
		return "", err
	}
	defer release()
	select {
	case payload := <-response:
		return payload, nil
	case <-time.After(timeout):
		return "", ErrResponseTimeout
	}
}

// ResolveResponse hands a CallResult payload to the caller waiting on it and
// reports whether anyone was waiting.
func (s *Server) ResolveResponse(uniqueId, payload string) bool {
	s.pendingMutex.Lock()
	channel, ok := s.pending[uniqueId]
	if !ok {
		// A fire-and-forget request expects a CallResult that nobody waits on;
		// report it as resolved so it is not logged as unmatched.
		if _, discard := s.fireAndForget[uniqueId]; discard {
			delete(s.fireAndForget, uniqueId)
			s.pendingMutex.Unlock()
			return true
		}
	}
	s.pendingMutex.Unlock()
	if !ok {
		return false
	}
	// The channel is buffered, so a caller that has already given up waiting
	// cannot wedge the connection's read pump here.
	channel <- payload
	return true
}

func (s *Server) registerPending(uniqueId string) chan string {
	channel := make(chan string, 1)
	s.pendingMutex.Lock()
	s.pending[uniqueId] = channel
	s.pendingMutex.Unlock()
	return channel
}

func (s *Server) releasePending(uniqueId string) {
	s.pendingMutex.Lock()
	delete(s.pending, uniqueId)
	s.pendingMutex.Unlock()
}

// markFireAndForget records a request whose CallResult is expected but has no
// waiting caller, so ResolveResponse can drop the answer silently instead of
// logging it as unmatched.
func (s *Server) markFireAndForget(uniqueId string) {
	s.pendingMutex.Lock()
	// A charge point that drops off before answering leaves its id behind;
	// evict an arbitrary stale entry rather than let the set grow without bound.
	if len(s.fireAndForget) >= maxFireAndForget {
		for id := range s.fireAndForget {
			delete(s.fireAndForget, id)
			break
		}
	}
	s.fireAndForget[uniqueId] = struct{}{}
	s.pendingMutex.Unlock()
}

type Status struct {
	ConnectedClients string `json:"connected_clients"`
	TotalClients     int    `json:"total_clients"`
}

func (s *Server) GetStatus() []byte {
	clientList := ""
	for _, client := range s.pool.clients {
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

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Debug("stopping websocket server...")
	s.pool.Stop()
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(shutdownCtx)
}
