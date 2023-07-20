package models

import (
	"sync"
	"time"
)

type Transaction struct {
	Id            int       `json:"transaction_id" bson:"transaction_id"`
	IsFinished    bool      `json:"is_finished" bson:"is_finished"`
	ConnectorId   int       `json:"connector_id" bson:"connector_id"`
	ChargePointId string    `json:"charge_point_id" bson:"charge_point_id"`
	IdTag         string    `json:"id_tag" bson:"id_tag"`
	ReservationId *int      `json:"reservation_id,omitempty" bson:"reservation_id"`
	MeterStart    int       `json:"meter_start" bson:"meter_start"`
	MeterStop     int       `json:"meter_stop" bson:"meter_stop"`
	TimeStart     time.Time `json:"time_start" bson:"time_start"`
	TimeStop      time.Time `json:"time_stop" bson:"time_stop"`
	Reason        string    `json:"reason" bson:"reason"`
	IdTagNote     string    `json:"id_tag_note" bson:"id_tag_note"`
	Username      string    `json:"username" bson:"username"`
	PaymentAmount int       `json:"payment_amount" bson:"payment_amount"`
	PaymentBilled int       `json:"payment_billed" bson:"payment_billed"`
	PaymentOrder  int       `json:"payment_order" bson:"payment_order"`
	mutex         *sync.Mutex
}

func (t *Transaction) Lock() {
	t.mutex.Lock()
}

func (t *Transaction) Unlock() {
	t.mutex.Unlock()
}

func (t *Transaction) Init() {
	if t.mutex == nil {
		t.mutex = &sync.Mutex{}
	}
}
