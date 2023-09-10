// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// Copyright 2023 The Multi-Cluster App Dispatcher Authors.
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
	"time"

	"k8s.io/klog/v2"

	clusterstatecache "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/cache"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	unallocatedCPUGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "unallocated_cpu",
		Help: "Unalocated CPU (in Milicores)",
	})
	unallocatedMemoryGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "unallocated_memory",
		Help: "Unalocated Memory (in TBD)",
	})
	unallocatedGPUGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "unallocated_gpu",
		Help: "Unalocated GPU (in TBD)",
	})
)

// register the cluster metrics
func registerClusterMetrics() {

	globalPromRegistry.MustRegister(unallocatedCPUGauge)
	globalPromRegistry.MustRegister(unallocatedMemoryGauge)
	globalPromRegistry.MustRegister(unallocatedGPUGauge)
}

type ClusterMetricsManager struct {
	Message string
}

func NewClusterMetricsManager(clusterStateCache clusterstatecache.Cache) *ClusterMetricsManager {
	clusterMetricsManager := &ClusterMetricsManager{}

	// register cluster metrics
	registerClusterMetrics()

	// update cluster metrics
	go foreverUpdateClusterMetrics(clusterStateCache)

	return clusterMetricsManager
}

// forever thread that updates the cluster metrics
func foreverUpdateClusterMetrics(clusterStateCache clusterstatecache.Cache) {

	for {
		resources := clusterStateCache.GetUnallocatedResources()
		klog.V(9).Infof("[GetExternalMetric] Cache resources: %f", resources)

		unallocatedCPUGauge.Set(float64(resources.MilliCPU))
		unallocatedMemoryGauge.Set(float64(resources.GPU))
		unallocatedGPUGauge.Set(float64(resources.GPU))
		time.Sleep(time.Second)
	}
}
