package ocpp

type WebSocketServer interface {
	SendResponse(ws WebSocket, response Response) error
	SendRequest(clientId string, request Request) error
}
