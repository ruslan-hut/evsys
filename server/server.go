package server

import (
	"evsys/internal"
	"evsys/internal/config"
	"evsys/utility"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"net"
	"net/http"
)

const (
	wsEndpoint = "/ws/:id"
)

type Server struct {
	conf           *config.Config
	httpServer     *http.Server
	upgrader       websocket.Upgrader
	messageHandler func(ws *WebSocket, data []byte) error
	logger         internal.LogHandler
}

type WebSocket struct {
	conn     *websocket.Conn
	id       string
	uniqueId string
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

func NewServer(conf *config.Config) *Server {
	server := Server{
		conf:     conf,
		upgrader: websocket.Upgrader{Subprotocols: []string{}},
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

func (s *Server) SetLogger(logger internal.LogHandler) {
	s.logger = logger
}

func (s *Server) Register(router *httprouter.Router) {
	router.GET(wsEndpoint, s.handleWsRequest)
}

func (s *Server) handleWsRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("id")
	s.logger.Debug(fmt.Sprintf("connection initiated from remote %s", r.RemoteAddr))

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

	s.logger.Debug(fmt.Sprintf("upgraded socket for %s and ready to receive data", id))
	ws := WebSocket{
		conn: conn,
		id:   id,
	}

	go s.messageReader(&ws)
}

func (s *Server) messageReader(ws *WebSocket) {
	conn := ws.conn
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, 3001) {
				s.logger.Debug(fmt.Sprintf("id %s leaving session", ws.id))
			} else {
				s.logger.Debug(fmt.Sprintf("id %s is closing session %s", ws.id, err))
			}
			err = conn.Close()
			if err != nil {
				s.logger.Warn(fmt.Sprintf("error while closing socket %s %s", ws.id, err))
			}
			return
		}
		s.logger.RawDataEvent("IN", string(message))
		if s.messageHandler != nil {
			err = s.messageHandler(ws, message)
			if err != nil {
				s.logger.Error(fmt.Sprintf("handling message from %s", ws.id), err)
				continue
			}
		}
	}
}

func (s *Server) Start() error {
	if s.conf == nil {
		return utility.Err("configuration not loaded")
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

func (s *Server) SendResponse(ws *WebSocket, response *Response) error {
	callResult, _ := CreateCallResult(response, ws.UniqueId())
	data, err := callResult.MarshalJSON()
	s.logger.RawDataEvent("OUT", string(data))
	if err != nil {
		s.logger.Error("error encoding response", err)
		return err
	}
	if err = ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		s.logger.Error("error sending response", err)
	}
	return err
}
