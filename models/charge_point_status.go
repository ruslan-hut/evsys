package models

type ChargePointStatus struct {
	ChargePointID string            `json:"charge_point_id" bson:"_id"`
	ConnectorID   int               `json:"connector_id" bson:"connector_id"`
	Status        string            `json:"status" bson:"status"`
	Time          string            `json:"time" bson:"time"`
	TransactionId int               `json:"transaction_id" bson:"transaction_id"`
	Connectors    []ConnectorStatus `json:"connectors" bson:"connectors"`
}

type ConnectorStatus struct {
	ConnectorID   int    `json:"connector_id" bson:"connector_id"`
	Status        string `json:"status" bson:"status"`
	Time          string `json:"time" bson:"time"`
	TransactionId int    `json:"transaction_id" bson:"transaction_id"`
	Info          string `json:"info" bson:"info"`
}
