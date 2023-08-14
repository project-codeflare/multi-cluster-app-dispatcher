package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Global Prometheus Registry
var globalPromRegistry = prometheus.NewRegistry()

// metricsHandler returns a http.Handler that serves the prometheus metrics
func PrometheusHandler() http.Handler {
	// Add Go module build info.
	globalPromRegistry.MustRegister(collectors.NewBuildInfoCollector())
	globalPromRegistry.MustRegister(collectors.NewGoCollector())
	globalPromRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	handlerOpts := promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	}

	return promhttp.HandlerFor(globalPromRegistry, handlerOpts)
}