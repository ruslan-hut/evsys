package errorlistener

import (
	"evsys/entity"
	"evsys/internal"
	"evsys/metrics/counters"
	"time"
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
	listener := &ErrorListener{db: db, log: log}
	go listener.startPeriodicUpdate()
	return listener
}

func (e ErrorListener) OnError(data *entity.ErrorData) {
	err := e.db.WriteError(data)
	if err != nil {
		e.log.Error("writing error data to database", err)
	}
	go e.observeErrors()
}

func (e ErrorListener) updateCounter() {
	go e.observeErrors()
}

func (e ErrorListener) observeErrors() {
	counter, err := e.db.GetTodayErrorCount()
	if err != nil {
		e.log.Error("getting today's error count", err)
		return
	}
	for _, c := range counter {
		id := c.ID
		counters.ErrorsToday(id.Location, id.ChargePointID, id.ErrorCode, c.Count)
	}
}

func (e ErrorListener) startPeriodicUpdate() {
	e.updateCounter()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.updateCounter()
		}
	}
}
