package ocpp

type WebSocket interface {
	ID() string
	UniqueId() string
	SetUniqueId(uniqueId string)
}
