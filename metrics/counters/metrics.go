package counters

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

var errorGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "ocpp",
	Name:      "errors_today",
	Help:      "Total number of errors by vendor code.",
}, []string{"location", "code", "charge_point_id"})

func ObserveConnections(location string, count int) {
	if len(location) == 0 {
		return
	}
	connectionsGauge.With(prometheus.Labels{"location": location}).Set(float64(count))
}

func ObserveTransactions(location string, count int) {
	if len(location) == 0 {
		return
	}
	activeTransactionsGauge.With(prometheus.Labels{"location": location}).Set(float64(count))
}

func ObserveError(location, chargePointId, code string) {
	if len(location) == 0 || len(code) == 0 || len(chargePointId) == 0 {
		return
	}
	errorCounts.With(prometheus.Labels{"location": location, "code": code, "charge_point_id": chargePointId}).Inc()
}

func ErrorsToday(location, chargePointId, code string, count int) {
	if len(location) == 0 || len(code) == 0 || len(chargePointId) == 0 {
		return
	}
	errorGauge.With(prometheus.Labels{"location": location, "code": code, "charge_point_id": chargePointId}).Set(float64(count))
}

var transactionGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "ocpp",
	Name:      "transactions_today",
	Help:      "Total number of transactions.",
}, []string{"location", "charge_point_id"})

func TransactionsToday(location, chargePointId string, count int) {
	if len(location) == 0 || len(chargePointId) == 0 {
		return
	}
	transactionGauge.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
		}).Set(float64(count))
}

var transactionCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "ocpp",
	Name:      "transaction_count",
	Help:      "Total number of transactions.",
}, []string{"location", "charge_point_id"})

func CountTransaction(location, chargePointId string) {
	if len(location) == 0 || len(chargePointId) == 0 {
		return
	}
	transactionCounter.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
		}).Inc()
}

var powerGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "ocpp",
	Name:      "consumed_power",
	Help:      "Consumed power.",
}, []string{"location", "charge_point_id"})

var powerCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "ocpp",
	Name:      "consumed_today",
	Help:      "Consumed power.",
}, []string{"location", "charge_point_id"})

func ConsumedToday(location, chargePointId string, power float64) {
	if len(location) == 0 || len(chargePointId) == 0 {
		return
	}
	powerGauge.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
		}).Set(power)
}

func CountConsumedPower(location, chargePointId string, power float64) {
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
}, []string{"location", "charge_point_id", "connector_id"})

func ObservePowerRate(location, chargePointId, connectorId string, power float64) {
	if len(location) == 0 {
		return
	}
	powerRateGauge.With(
		prometheus.Labels{
			"location":        location,
			"charge_point_id": chargePointId,
			"connector_id":    connectorId,
		}).Set(power)
}
