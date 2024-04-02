package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var connectionsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "server",
	Name:      "connections_active",
	Help:      "Number of active ws connections",
}, []string{"location"})

var activeTransactionsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "server",
	Name:      "transactions_active",
	Help:      "Number of active transactions",
}, []string{"location"})

var errorCounts = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "ocpp",
	Name:      "vendor_error_count",
	Help:      "Total number of errors by vendor code.",
}, []string{"location", "code", "charge_point_id"})

func observeConnections(location string, count int) {
	if len(location) == 0 {
		return
	}
	connectionsGauge.With(prometheus.Labels{"location": location}).Set(float64(count))
}

func observeTransactions(location string, count int) {
	if len(location) == 0 {
		return
	}
	activeTransactionsGauge.With(prometheus.Labels{"location": location}).Set(float64(count))
}

func observeError(location, chargePointId, code string) {
	if len(location) == 0 || len(code) == 0 || len(chargePointId) == 0 {
		return
	}
	errorCounts.With(prometheus.Labels{"location": location, "code": code, "charge_point_id": chargePointId}).Inc()
}
