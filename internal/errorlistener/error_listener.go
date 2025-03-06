package errorlistener

import (
	"evsys/entity"
	"evsys/internal"
	"evsys/metrics/counters"
	"fmt"
)

type Database interface {
	WriteError(data *entity.ErrorData) error
	GetTodayErrorCount() ([]*entity.ErrorCounter, error)
}

type ErrorListener struct {
	db  Database
	log internal.LogHandler
}

func NewErrorListener(db Database, log internal.LogHandler) *ErrorListener {
	log.FeatureEvent("ErrorListener", "", "created")
	return &ErrorListener{db: db, log: log}
}

func (e ErrorListener) OnError(data *entity.ErrorData) {
	err := e.db.WriteError(data)
	if err != nil {
		e.log.Error("writing error data to database", err)
	}
	go e.observeErrors()
}

func (e ErrorListener) UpdateCounter() {
	go e.observeErrors()
}

func (e ErrorListener) observeErrors() {
	counter, err := e.db.GetTodayErrorCount()
	if err != nil {
		e.log.Error("getting today's error count", err)
		return
	}
	for _, c := range counter {
		e.log.FeatureEvent("ErrorListener", c.ChargePointID, fmt.Sprintf("updating counter: %v -- %d", c.ErrorCode, c.Count))
		counters.ErrorsToday(c.Location, c.ChargePointID, c.ErrorCode, c.Count)
	}
}
