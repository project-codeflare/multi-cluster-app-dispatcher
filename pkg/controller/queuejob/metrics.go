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
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func registerMetrics(controller *XController) {
	allocatableCapacityCpu := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_cpu",
		Help:      "Allocatable CPU Capacity (in milicores)",
	}, func() float64 { return controller.GetAllocatableCapacity().MilliCPU })
	metrics.Registry.MustRegister(allocatableCapacityCpu)

	allocatableCapacityMemory := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_memory",
		Help:      "Allocatable Memory Capacity",
	}, func() float64 { return controller.GetAllocatableCapacity().Memory })
	metrics.Registry.MustRegister(allocatableCapacityMemory)

	allocatableCapacityGpu := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_gpu",
		Help:      "Allocatable GPU Capacity",
	}, func() float64 { return float64(controller.GetAllocatableCapacity().GPU) })
	metrics.Registry.MustRegister(allocatableCapacityGpu)
}
