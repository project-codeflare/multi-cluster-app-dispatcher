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
	"time"
)

var (
	allocatableCapacityCpu = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_cpu",
		Help:      "Allocatable CPU Capacity (in milicores)",
	})
	allocatableCapacityMemory = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_memory",
		Help:      "Allocatable Memory Capacity",
	})
	allocatableCapacityGpu = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_gpu",
		Help:      "Allocatable GPU Capacity",
	})
)

func registerMetrics() {
	klog.V(10).Infof("Registering metrics")
	metrics.Registry.MustRegister(
		allocatableCapacityCpu,
		allocatableCapacityMemory,
		allocatableCapacityGpu,
	)
}

func updateMetricsLoop(controller *XController, stopCh <-chan struct{}) {
	ticker := time.NewTicker(time.Minute * 1)
	go func() {
		updateMetrics(controller)
		for {
			select {
			case <-ticker.C:
				klog.V(10).Infof("Update metrics loop tick")
				updateMetrics(controller)
			case <-stopCh:
				klog.V(10).Infof("Exiting update metrics loop")
				ticker.Stop()
				return
			}
		}
	}()
}

func updateMetrics(controller *XController) {
	res := controller.GetAllocatableCapacity()
	allocatableCapacityCpu.Set(res.MilliCPU)
	allocatableCapacityMemory.Set(res.Memory)
	allocatableCapacityGpu.Set(float64(res.GPU))
}
