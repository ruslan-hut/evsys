package server

import (
	"evsys/internal"
	"evsys/internal/config"
	"evsys/ocpp"
	"evsys/utility"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"net"
	"net/http"
)

const (
	wsEndpoint           = "/ws/:id"
	featureNameWebSocket = "WebSocket"
)

type envelope struct {
	recipient string
	message   CallRequest
}

type Server struct {
	conf           *config.Config
	httpServer     *http.Server
	upgrader       websocket.Upgrader
	pool           *Pool
	messageHandler func(ws *WebSocket, data []byte) error
	logger         internal.LogHandler
}

type WebSocket struct {
	conn           *websocket.Conn
	send           chan []byte
	pool           *Pool
	id             string
	uniqueId       string
	messageHandler func(ws *WebSocket, data []byte) error
	logger         internal.LogHandler
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
		case envelope := <-pool.send:
			for client := range pool.clients {
				if client.id == envelope.recipient {
					request := envelope.message
					request.UniqueId = client.uniqueId
					data, err := request.MarshalJSON()
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

func (ws *WebSocket) ID() string {
	return ws.id
}

func (ws *WebSocket) UniqueId() string {
	return ws.uniqueId
}

func (ws *WebSocket) SetUniqueId(uniqueId string) {
	ws.uniqueId = uniqueId
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

func (s *Server) SetMessageHandler(handler func(ws *WebSocket, data []byte) error) {
	s.messageHandler = handler
}

func (s *Server) Register(router *httprouter.Router) {
	router.GET(wsEndpoint, s.handleWsRequest)
}

func (s *Server) handleWsRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("id")
	//s.logger.Debug(fmt.Sprintf("connection initiated from remote %s", r.RemoteAddr))

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
	}
	s.pool.register <- &ws

	go ws.readPump()
	go ws.writePump()
}

func (ws *WebSocket) readPump() {
	defer func() {
		ws.close()
	}()
	conn := ws.conn
	for {
		_, message, err := conn.ReadMessage()
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
	}
}

func (ws *WebSocket) writePump() {
	defer func() {
		ws.close()
	}()
	conn := ws.conn
	for {
		select {
		case message, ok := <-ws.send:
			if !ok {
				//ws.logger.Debug(fmt.Sprintf("id %s leaving session", ws.id))
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			ws.logger.RawDataEvent("OUT", string(message))
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				ws.logger.Warn(fmt.Sprintf("socket %s: %s", ws.id, err))
				return
			}
		}
	}
}

// close closing the websocket connection
func (ws *WebSocket) close() {
	ws.pool.unregister <- ws
	_ = ws.conn.Close()
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

func (s *Server) SendResponse(ws *WebSocket, response *ocpp.Response) error {
	callResult, _ := CreateCallResult(response, ws.UniqueId())
	data, err := callResult.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error encoding response: %s", err)
	}
	ws.send <- data
	return nil
}

// SendRequest send request to the websocket
func (s *Server) SendRequest(clientId string, request ocpp.Request) error {
	callRequest, err := CreateCallRequest(request)
	if err != nil {
		return fmt.Errorf("error creating call request: %s", err)
	}
	envelope := &envelope{
		recipient: clientId,
		message:   callRequest,
	}
	s.pool.send <- envelope
	return nil
}
