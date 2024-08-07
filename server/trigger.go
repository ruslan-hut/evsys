package server

import (
	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp/remotetrigger"
	"fmt"
	"time"
)

const featureNameTrigger = "Trigger"

type Trigger struct {
	connectors map[int]*entity.Connector
	Register   chan *entity.Connector
	Unregister chan int
	server     *Server
	logger     internal.LogHandler
}

func NewTrigger(server *Server, logger internal.LogHandler) *Trigger {
	return &Trigger{
		connectors: make(map[int]*entity.Connector),
		Register:   make(chan *entity.Connector),
		Unregister: make(chan int),
		server:     server,
		logger:     logger,
	}
}

func (t *Trigger) Start() {
	go t.listen()
	go t.triggerMeterValues()
}

func (t *Trigger) triggerMeterValues() {
	message := "MeterValues"
	waitStep := 20
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for _, connector := range t.connectors {
				request := remotetrigger.NewTriggerMessageRequest(remotetrigger.MessageTrigger(message), connector.Id)
				_, err := t.server.SendRequest(connector.ChargePointId, request)
				if err != nil {
					t.logger.FeatureEvent(featureNameTrigger, connector.ChargePointId, fmt.Sprintf("error sending request: %v", err))
				}
			}
		}
	}
}

func (t *Trigger) listen() {
	for {
		select {
		case connector := <-t.Register:
			t.logger.FeatureEvent(featureNameTrigger, connector.ChargePointId, fmt.Sprintf("start watching on connector: %v transaction: %v", connector.Id, connector.CurrentTransactionId))
			t.connectors[connector.CurrentTransactionId] = connector
		case transactionId := <-t.Unregister:
			connector, ok := t.connectors[transactionId]
			if !ok {
				continue
			}
			t.logger.FeatureEvent(featureNameTrigger, connector.ChargePointId, fmt.Sprintf("stop watching on transaction: %v", transactionId))
			delete(t.connectors, transactionId)
		}
	}
}

func (t *Trigger) UnregisterConnector(connector *entity.Connector) {
	for id, c := range t.connectors {
		if c == connector {
			t.Unregister <- id
		}
	}
}
