package client

import (
	"encoding/json"
	"evsys/internal/handlers"
	"evsys/internal/ocpp16/messages"
	"evsys/ocpp"
	"evsys/types"
	"evsys/utility"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"time"
)

const (
	clientRoot               = "/cp"
	clientPoint              = "/cp/:id"
	wsRoot                   = "/ws/:id"
	headerProto              = "Sec-Websocket-Protocol"
	subProtocol16            = "ocpp1.6"
	defaultHeartbeatInterval = 600
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type handler struct {
}

func (h *handler) Register(router *httprouter.Router) {
	router.GET(clientRoot, h.GetRequest)
	router.GET(clientPoint, h.PointRequest)
	router.GET(wsRoot, h.SocketConnect)
}

func NewHandler() handlers.Handler {
	return &handler{}
}

func (h *handler) GetRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	log.Println("get request")
	w.WriteHeader(200)
	_, err := w.Write([]byte("cp..."))
	if err != nil {
		log.Fatal(err)
	}
}

func (h *handler) PointRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	log.Printf("point id: %s", params.ByName("id"))
	w.WriteHeader(200)
	_, err := w.Write([]byte("cp ID..."))
	if err != nil {
		log.Fatal(err)
	}
}

func (h *handler) SocketConnect(w http.ResponseWriter, r *http.Request, params httprouter.Params) {

	id := params.ByName("id")
	log.Printf("websocket request %s from remote %s", id, r.RemoteAddr)

	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	protocols := websocket.Subprotocols(r)
	log.Printf("requested protocol: %v", protocols)
	var header http.Header = nil
	if utility.Contains(protocols, subProtocol16) {
		header = http.Header{
			headerProto: []string{subProtocol16},
		}
	}

	ws, err := upgrader.Upgrade(w, r, header)
	if err != nil {
		log.Println("websocket upgrade: ", err)
		return
	}
	defer socketClose(ws)

	log.Printf("%s socket up, ready to receive messages", subProtocol16)
	socketReader(ws)
}

func sendResponse(conn *websocket.Conn, data []byte) {
	log.Println(">>> sending response")
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Println("error sending response; ", err)
	}
}

func socketReader(conn *websocket.Conn) {
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("read message error; ", err)
			return
		}
		data, err := ParseJson(p)
		if err != nil {
			log.Println("decode message error; ", err)
			return
		}
		request, err := handlers.ParseRequest(data)
		if err != nil {
			log.Println("parse request error; ", err)
			return
		}

		confirmation, err := GetResponse(request.Feature)
		if err != nil {
			log.Println("failed to response; ", err)
			return
		}

		callResult, err := handlers.CreateCallResult(confirmation, request.UniqueId)
		jsonMessage, err := callResult.MarshalJSON()
		if err != nil {
			log.Println("error encoding response; ", err)
			return
		}

		sendResponse(conn, jsonMessage)
	}
}

func GetResponse(action string) (*ocpp.Response, error) {
	var confirmation ocpp.Response
	switch action {
	case string(handlers.BootNotification):
		confirmation = messages.NewBootNotificationResponse(types.NewDateTime(time.Now()), defaultHeartbeatInterval, messages.RegistrationStatusAccepted)
	default:
		return nil, utility.Err(fmt.Sprintf("unsupported feature requested: %s", action))
	}
	return &confirmation, nil
}

func socketClose(conn *websocket.Conn) {
	if err := conn.Close(); err != nil {
		log.Println("error on socket close; ", err)
	}
}

func ParseJson(b []byte) ([]interface{}, error) {
	var array []interface{}
	err := json.Unmarshal(b, &array)
	return array, err
}
