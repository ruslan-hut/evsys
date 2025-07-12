package entity

import (
	"evsys/entity/tariff"
	"sync"
	"time"
)

type Transaction struct {
	Id            int                `json:"transaction_id" bson:"transaction_id"`
	SessionId     string             `json:"session_id" bson:"session_id"`
	IsFinished    bool               `json:"is_finished" bson:"is_finished"`
	ConnectorId   int                `json:"connector_id" bson:"connector_id"`
	ChargePointId string             `json:"charge_point_id" bson:"charge_point_id"`
	IdTag         string             `json:"id_tag" bson:"id_tag"`
	ReservationId *int               `json:"reservation_id,omitempty" bson:"reservation_id"`
	MeterStart    int                `json:"meter_start" bson:"meter_start"`
	MeterStop     int                `json:"meter_stop" bson:"meter_stop"`
	TimeStart     time.Time          `json:"time_start" bson:"time_start"`
	TimeStop      time.Time          `json:"time_stop" bson:"time_stop"`
	Reason        string             `json:"reason" bson:"reason"`
	IdTagNote     string             `json:"id_tag_note" bson:"id_tag_note"`
	Username      string             `json:"username" bson:"username"`
	PaymentAmount int                `json:"payment_amount" bson:"payment_amount"`
	PaymentBilled int                `json:"payment_billed" bson:"payment_billed"`
	PaymentOrder  int                `json:"payment_order" bson:"payment_order"`
	Plan          *PaymentPlan       `json:"payment_plan,omitempty" bson:"payment_plan,omitempty"`
	Tariff        *tariff.Tariff     `json:"tariff,omitempty" bson:"tariff,omitempty"`
	MeterValues   []TransactionMeter `json:"meter_values" bson:"meter_values"`
	PaymentMethod *PaymentMethod     `json:"payment_method,omitempty" bson:"payment_method"`
	PaymentOrders []PaymentOrder     `json:"payment_orders" bson:"payment_orders"`
	UserTag       *UserTag           `json:"user_tag,omitempty" bson:"user_tag"`
	mutex         sync.Mutex
}

func (t *Transaction) Lock() {
	t.mutex.Lock()
}

func (t *Transaction) Unlock() {
	t.mutex.Unlock()
}

func (t *Transaction) Init() {
	t.mutex = sync.Mutex{}
}
