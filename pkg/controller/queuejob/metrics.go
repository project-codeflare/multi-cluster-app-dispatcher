/*
Copyright 2023 The Multi-Cluster App Dispatcher Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package queuejob

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	requestedMilliCpu = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "mcad",
		Name:      "requested_cpu",
		Help:      "Histogram of requested CPU (in millicores)",
		Buckets:   []float64{100, 200, 500, 1_000, 2_000, 5_000, 10_000},
	})
	requestedMemory = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "mcad",
		Name:      "requested_memory_bytes",
		Help:      "Histogram of requested memory",
		Buckets:   prometheus.ExponentialBuckets(128*1024*1024, 2, 10), // Max bucket: 128MiB*2^(10-1) = 64GiB
	})
	requestedGpu = prometheus.NewHistogram(prometheus.HistogramOpts{
		Subsystem: "mcad",
		Name:      "requested_gpu",
		Help:      "Histogram of requested GPU",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // Max bucket: 1*2^(10-1) = 512
	})
)

func init() {
	klog.V(10).Infof("Registering metrics")
	metrics.Registry.MustRegister(
		requestedMilliCpu,
		requestedMemory,
		requestedGpu,
	)
}
