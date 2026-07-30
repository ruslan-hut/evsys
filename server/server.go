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

	// wsWriteWait bounds a single write. Without it a charge point that has stopped reading - a
	// NAT mapping dropped mid-write, a wedged firmware - blocks the write pump forever, and with
	// it every later message queued for that charge point.
	wsWriteWait = 10 * time.Second
	// wsPingPeriod is how often an otherwise idle connection is pinged. Frequent enough to keep a
	// NAT mapping from expiring on the operator networks this fleet sits behind, where idle
	// connections were being dropped silently every few minutes.
	wsPingPeriod = 30 * time.Second
	// wsPongWait is how long a connection may stay silent before it is closed, once the charge
	// point has proved it answers pings. Four missed pings: long enough to ride out a hiccup on a
	// mobile link, short enough that is_online stops lying about a charger that is already gone.
	wsPongWait = 2 * time.Minute
	// wsSilenceWait is the same limit for a charge point that has never answered a ping. Not every
	// OCPP stack implements pongs, and for one that does not, silence only becomes evidence once it
	// outlasts the heartbeat interval we asked for in BootNotification - so this is derived from
	// that interval rather than chosen, and must stay comfortably above it.
	wsSilenceWait = 2 * defaultHeartbeatInterval * time.Second
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
	// keepalive windows handed to every connection this server accepts. They default to the
	// wsPingPeriod/wsPongWait/wsSilenceWait constants and are fields so a test can run the same
	// liveness logic on a timescale it can wait for.
	pingPeriod  time.Duration
	pongWait    time.Duration
	silenceWait time.Duration
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
	// pongSeen records that this charge point has answered a ping, which is what earns it the
	// short silence window. Read and written only on the read goroutine: the pong handler runs
	// inside ReadMessage, so no synchronisation is needed here.
	pongSeen bool
	// keepalive windows, copied from the server that accepted this connection
	pingPeriod  time.Duration
	pongWait    time.Duration
	silenceWait time.Duration
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
		pingPeriod:    wsPingPeriod,
		pongWait:      wsPongWait,
		silenceWait:   wsSilenceWait,
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
		pingPeriod:     s.pingPeriod,
		pongWait:       s.pongWait,
		silenceWait:    s.silenceWait,
	}
	s.pool.register <- &ws

	go ws.readPump()
	go ws.writePump()
}

// readWindow is how long the connection may stay silent before the read deadline closes it. Any
// frame pushes the deadline out again, data or pong alike, so this is the window for a charge point
// that has stopped talking altogether rather than one that is merely idle between messages.
func (ws *WebSocket) readWindow() time.Duration {
	if ws.pongSeen {
		return ws.pongWait
	}
	return ws.silenceWait
}

func (ws *WebSocket) readPump() {
	defer func() {
		ws.close()
	}()
	// Until this deadline exists, a connection whose peer vanishes without a FIN is only noticed
	// when the kernel gives up retransmitting, which took upwards of twenty minutes on this fleet -
	// all of it with the charge point recorded as online and its connectors offered to drivers.
	_ = ws.conn.SetReadDeadline(time.Now().Add(ws.readWindow()))
	ws.conn.SetPongHandler(func(string) error {
		ws.pongSeen = true
		return ws.conn.SetReadDeadline(time.Now().Add(ws.readWindow()))
	})
	for {
		_, message, err := ws.conn.ReadMessage()
		if err != nil {
			var netErr net.Error
			switch {
			case websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, 3001):
				//ws.logger.Debug(fmt.Sprintf("id %s leaving session", ws.id))
			case errors.As(err, &netErr) && netErr.Timeout():
				// worth telling apart from a transport failure: this one is our own deadline, so
				// the question it raises is whether the window is too tight, not what the network did
				ws.logger.FeatureEvent(featureNameWebSocket, ws.id, fmt.Sprintf(
					"silent for %s with no answer to keepalive pings: closing connection", ws.readWindow()))
			default:
				ws.logger.FeatureEvent(featureNameWebSocket, ws.id, fmt.Sprintf("read error: %s", err))
			}
			break
		}
		_ = ws.conn.SetReadDeadline(time.Now().Add(ws.readWindow()))
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
	// the ping half of the keepalive: it gives an idle connection something to fail on, which is
	// what turns the read deadline from a timer into a liveness check
	ticker := time.NewTicker(ws.pingPeriod)
	defer func() {
		ticker.Stop()
		// the pump reaching its end is the only evidence that it is not still spinning on a closed
		// send channel, which is what it used to do; cheap to log, and it bounds the goroutine
		ws.logger.Debug(fmt.Sprintf("%s: write pump stopped", ws.id))
		ws.close()
	}()
	for {
		// every exit here returns rather than breaking the select: the channel stays readable once
		// closed and a failing socket keeps failing, so breaking only the select spins the pump
		select {
		case message, ok := <-ws.send:
			if !ok {
				//ws.logger.Debug(fmt.Sprintf("id %s leaving session", ws.id))
				_ = ws.writeMessage(websocket.CloseMessage, []byte{})
				return
			}
			ws.logger.RawDataEvent("OUT", string(message))

			err := ws.writeMessage(websocket.TextMessage, message)

			if err != nil {
				ws.logger.Warn(fmt.Sprintf("socket %s: %s", ws.id, err))
				return
			}
		case <-ticker.C:
			if err := ws.writeMessage(websocket.PingMessage, nil); err != nil {
				// the write half noticing first is normal: a dead link fails a ping immediately,
				// while the read half has to wait out its whole window
				ws.logger.FeatureEvent(featureNameWebSocket, ws.id, fmt.Sprintf("keepalive ping failed: %s", err))
				return
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
	// gorilla permits one writer at a time, which the mutex above provides; the deadline is what
	// keeps a peer that has stopped reading from parking this goroutine indefinitely
	_ = ws.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
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
