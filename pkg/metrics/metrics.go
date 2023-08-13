// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// Copyright 2022 The Multi-Cluster App Dispatcher Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// ------------------------------------------------------ {COPYRIGHT-END} ---
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dummyCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "dummy_counter",
		Help: "A dummy Prometheus counter",
	})

	gpuGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gpu",
		Help: "Number of Available GPUs",
	})
)

// Global Prometheus Registry
var globalPromRegistry = prometheus.NewRegistry()

// MetricsHandler returns a http.Handler that serves the prometheus metrics
func Handler() http.Handler {

	// register standrad metrics
	globalPromRegistry.MustRegister(collectors.NewBuildInfoCollector())
	globalPromRegistry.MustRegister(collectors.NewGoCollector())
	globalPromRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// register custom metrics
	registerCustomMetrics()

	// update custom metrics
	go foreverUpdateCustomMetrics()

	handlerOpts := promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	}

	return promhttp.HandlerFor(globalPromRegistry, handlerOpts)
}

// register the custom metrics
func registerCustomMetrics() {

	globalPromRegistry.MustRegister(dummyCounter)
	globalPromRegistry.MustRegister(gpuGauge)
}

// forever thread that updates the custom metrics
func foreverUpdateCustomMetrics() {
	for {
		dummyCounter.Inc()
		gpuGauge.Inc()
		time.Sleep(time.Second)
	}
}
