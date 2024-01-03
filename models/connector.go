package models

import (
	"sync"
	"time"
)

type Connector struct {
	Id                   int       `json:"connector_id" bson:"connector_id"`
	IdName               string    `json:"connector_id_name" bson:"connector_id_name"`
	ChargePointId        string    `json:"charge_point_id" bson:"charge_point_id"`
	IsEnabled            bool      `json:"is_enabled" bson:"is_enabled"`
	Status               string    `json:"status" bson:"status"`
	StatusTime           time.Time `json:"status_time" bson:"status_time"`
	Info                 string    `json:"info" bson:"info"`
	VendorId             string    `json:"vendor_id" bson:"vendor_id"`
	ErrorCode            string    `json:"error_code" bson:"error_code"`
	Type                 string    `json:"type" bson:"type"`
	Power                int       `json:"power" bson:"power"`
	CurrentTransactionId int       `json:"current_transaction_id" bson:"current_transaction_id"`
	mutex                *sync.Mutex
}

func (c *Connector) Lock() {
	c.mutex.Lock()
}

func (c *Connector) Unlock() {
	c.mutex.Unlock()
}

func (c *Connector) Init() {
	if c.mutex == nil {
		c.mutex = &sync.Mutex{}
	}
}

func NewConnector(id int, chargePointId string) *Connector {
	return &Connector{
		Id:                   id,
		ChargePointId:        chargePointId,
		IsEnabled:            true,
		CurrentTransactionId: -1,
		mutex:                &sync.Mutex{},
	}
}
