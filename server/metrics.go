package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var connectionsGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "server",
	Name:      "connections_active",
	Help:      "Number of active ws connections",
})

var errorCounts = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "ocpp",
	Name:      "vendor_error_count",
	Help:      "Total number of errors by vendor code.",
}, []string{"code,charge_point_id"})

func observeConnections(count int) {
	connectionsGauge.Set(float64(count))
}

func observeError(chargePointId, code string) {
	if len(code) == 0 || len(chargePointId) == 0 {
		return
	}
	errorCounts.With(prometheus.Labels{"code": code, "charge_point_id": chargePointId}).Inc()
}
