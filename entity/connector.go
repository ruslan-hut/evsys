package entity

import (
	"strconv"
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
	State                string    `json:"state" bson:"state"`
	Info                 string    `json:"info" bson:"info"`
	VendorId             string    `json:"vendor_id" bson:"vendor_id"`
	ErrorCode            string    `json:"error_code" bson:"error_code"`
	Type                 string    `json:"type" bson:"type"`
	Power                int       `json:"power" bson:"power"`
	CurrentPowerLimit    int       `json:"current_power_limit" bson:"current_power_limit"`
	CurrentTransactionId int       `json:"current_transaction_id" bson:"current_transaction_id"`
	EvseId               *int      `json:"evse_id,omitempty" bson:"evse_id,omitempty"` // OCPP 2.0.1+ EVSE identifier (nullable for 1.6 compatibility)
	// LastProfile is the charge point's answer to the last power limit installed
	// here. CurrentPowerLimit records what was asked for and is written before
	// the answer arrives, so on its own it cannot distinguish a limit in force
	// from one the charge point refused. Nil until a profile has been sent.
	LastProfile *ProfileVerdict `json:"last_profile,omitempty" bson:"last_profile,omitempty"`
	mutex       sync.Mutex
}

// Statuses reported for an installed charging profile. The first three are
// OCPP's own; the rest describe answers that never arrived or made no sense,
// which are just as much a failure to enforce a limit.
const (
	ProfileStatusAccepted     = "Accepted"
	ProfileStatusRejected     = "Rejected"
	ProfileStatusNotSupported = "NotSupported"
	ProfileStatusNoResponse   = "NoResponse"
	ProfileStatusUnreadable   = "Unreadable"
)

// ProfileVerdict is what a charge point said about a charging profile, kept so
// the question "is this limit actually in force" can be answered from the
// database instead of by grepping the log.
type ProfileVerdict struct {
	Status     string    `json:"status" bson:"status"`
	Limit      int       `json:"limit" bson:"limit"`
	StackLevel int       `json:"stack_level" bson:"stack_level"`
	Time       time.Time `json:"time" bson:"time"`
}

// Accepted reports whether the charge point took the profile.
func (v *ProfileVerdict) Accepted() bool {
	return v != nil && v.Status == ProfileStatusAccepted
}

func (c *Connector) Lock() {
	c.mutex.Lock()
}

func (c *Connector) Unlock() {
	c.mutex.Unlock()
}

func (c *Connector) Init() {
	c.mutex = sync.Mutex{}
}

func (c *Connector) ID() string {
	return strconv.Itoa(c.Id)
}

func NewConnector(id int, chargePointId string) *Connector {
	return &Connector{
		Id:                   id,
		ChargePointId:        chargePointId,
		IsEnabled:            true,
		CurrentTransactionId: -1,
		mutex:                sync.Mutex{},
	}
}
