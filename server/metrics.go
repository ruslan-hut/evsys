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

var transactionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "ocpp",
	Name:      "transaction_count",
	Help:      "Total number of transactions.",
}, []string{"location", "charge_point_id"})

func countTransaction(location, chargePointId string) {
	if len(location) == 0 || len(chargePointId) == 0 {
		return
	}
	transactionCounter.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
		}).Inc()
}

var powerCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "ocpp",
	Name:      "consumed_power",
	Help:      "Consumed power.",
}, []string{"location", "charge_point_id"})

func countConsumedPower(location, chargePointId string, power float64) {
	if len(location) == 0 || len(chargePointId) == 0 {
		return
	}
	powerCounter.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
		}).Add(power)
}

var powerRateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "ocpp",
	Name:      "current_power_rate",
	Help:      "Power rate on current transactions.",
}, []string{"location", "charge_point_id"})

func observePowerRate(location, chargePointId string, power float64) {
	if len(location) == 0 {
		return
	}
	powerRateGauge.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
		}).Set(power)
}
