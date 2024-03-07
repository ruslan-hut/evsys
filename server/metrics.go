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

func observeConnections(count int) {
	connectionsGauge.Set(float64(count))
}
