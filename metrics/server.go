package metrics

import (
	"evsys/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

func Listen(conf *config.Config) error {
	if !conf.Metrics.Enabled {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	address := conf.Metrics.BindIP + ":" + conf.Metrics.Port
	log.Println("starting metrics server on " + address)
	return http.ListenAndServe(address, mux)
}
