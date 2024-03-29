package server

import (
	"evsys/internal"
	"evsys/models"
	"evsys/ocpp/remotetrigger"
	"fmt"
	"time"
)

const featureNameTrigger = "Trigger"

type Trigger struct {
	connectors map[int]*models.Connector
	Register   chan *models.Connector
	Unregister chan int
	server     *Server
	logger     internal.LogHandler
}

func NewTrigger(server *Server, logger internal.LogHandler) *Trigger {
	return &Trigger{
		connectors: make(map[int]*models.Connector),
		Register:   make(chan *models.Connector),
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
			if _, ok := t.connectors[connector.CurrentTransactionId]; ok {
				return
			}
			t.logger.FeatureEvent(featureNameTrigger, connector.ChargePointId, fmt.Sprintf("start watching on connector: %v transaction: %v", connector.Id, connector.CurrentTransactionId))
			t.connectors[connector.CurrentTransactionId] = connector
		case transactionId := <-t.Unregister:
			if _, ok := t.connectors[transactionId]; ok {
				t.logger.FeatureEvent(featureNameTrigger, "", fmt.Sprintf("stop watching on transaction: %v", transactionId))
				delete(t.connectors, transactionId)
			}
		}
	}
}
