package ocpp

import "evsys/ocpp/common"

type WebSocket interface {
	ID() string
	UniqueId() string
	SetUniqueId(uniqueId string)
	IsClosed() bool
	// GetProtocol returns the negotiated OCPP protocol version for this connection
	GetProtocol() common.ProtocolVersion
	// SetProtocol sets the protocol version for this connection
	SetProtocol(common.ProtocolVersion)
}
