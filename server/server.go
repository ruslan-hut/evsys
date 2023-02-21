package server

import (
	"evsys/internal/config"
	"evsys/utility"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"log"
	"net"
	"net/http"
)

type ApiCallType string

const (
	wsEndpoint         = "/ws/:id"
	apiReadLogEndpoint = "/api/v1/log"

	ApiReadLog ApiCallType = "apiReadLog"
)

type Server struct {
	conf           *config.Config
	httpServer     *http.Server
	upgrader       websocket.Upgrader
	messageHandler func(ws *WebSocket, data []byte) error
	apiHandler     func(ac *ApiCall) error
}

type ApiCall struct {
	writer   *http.ResponseWriter
	CallType ApiCallType
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

func (s *Server) Register(router *httprouter.Router) {
	router.GET(wsEndpoint, s.handleWsRequest)
	router.GET(apiReadLogEndpoint, s.readLog)
}

func (s *Server) handleWsRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	id := params.ByName("id")
	log.Printf("connection initiated from remote %s", r.RemoteAddr)

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
		log.Println("upgrade failed: ", err)
		return
	}

	log.Printf("[%s] socket up, ready to receive messages", id)
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
				log.Printf("[%s] leaving session", ws.id)
			} else {
				log.Printf("[%s] %s; closing session", ws.id, err)
			}
			err = conn.Close()
			if err != nil {
				log.Printf("[%s] error while closing socket: %s", ws.id, err)
			}
			return
		}
		if s.messageHandler != nil {
			err = s.messageHandler(ws, message)
			if err != nil {
				log.Printf("[%s] error: %s", ws.id, err)
				continue
			}
		}
	}
}

func (s *Server) readLog(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log.Printf("api call received from remote %s", r.RemoteAddr)
	ac := &ApiCall{
		writer:   &w,
		CallType: ApiReadLog,
	}
	go s.handleApiRequest(ac)
}

func (s *Server) handleApiRequest(ac *ApiCall) {
	if s.apiHandler != nil {
		if err := s.apiHandler(ac); err != nil {
			log.Println("error handling api request;", err)
		}
	}
}

func (s *Server) Start() error {
	if s.conf == nil {
		return utility.Err("configuration not loaded")
	}
	serverAddress := fmt.Sprintf("%s:%s", s.conf.Listen.BindIP, s.conf.Listen.Port)
	log.Printf("initializing listener on %s", serverAddress)
	listener, err := net.Listen("tcp", serverAddress)
	if err != nil {
		return err
	}
	if s.conf.Listen.TLS {
		log.Println("starting https TLS server")
		err = s.httpServer.ServeTLS(listener, s.conf.Listen.CertFile, s.conf.Listen.KeyFile)
	} else {
		log.Println("starting http server")
		err = s.httpServer.Serve(listener)
	}
	return err
}

func (s *Server) SendResponse(ws *WebSocket, response *Response) error {
	callResult, _ := CreateCallResult(response, ws.UniqueId())
	data, err := callResult.MarshalJSON()
	if err != nil {
		log.Println("error encoding response; ", err)
		return err
	}
	if err = ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Println("error sending response; ", err)
	}
	return err
}
