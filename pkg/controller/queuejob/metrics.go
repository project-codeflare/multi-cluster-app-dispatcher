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
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"reflect"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"time"
)

var (
	allocatableCapacityCpu = prometheus.NewGauge(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "allocatable_capacity_cpu",
		Help:      "Allocatable CPU Capacity (in millicores)",
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
	appWrappersCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "mcad",
		Name:      "appwrappers_count",
		Help:      "AppWrappers count per state",
	}, []string{"state"})
)

func init() {
	klog.V(10).Infof("Registering metrics")
	metrics.Registry.MustRegister(
		allocatableCapacityCpu,
		allocatableCapacityMemory,
		allocatableCapacityGpu,
		appWrappersCount,
	)
}

func updateMetricsLoop(controller *XController, stopCh <-chan struct{}) {
	updateMetricsLoopGeneric(controller, stopCh, time.Minute*1, updateAllocatableCapacity)
	updateMetricsLoopGeneric(controller, stopCh, time.Second*5, updateQueue)
}

func getFuncName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

func updateMetricsLoopGeneric(controller *XController, stopCh <-chan struct{}, d time.Duration, updateFunc func(xController *XController)) {
	ticker := time.NewTicker(d)
	go func() {
		updateFunc(controller)
		for {
			select {
			case <-ticker.C:
				if klog.V(10).Enabled() {
					klog.Infof("[updateMetricsLoopGeneric] Update metrics loop tick: %v", getFuncName(updateFunc))
				}
				updateFunc(controller)
			case <-stopCh:
				if klog.V(10).Enabled() {
					klog.Infof("[updateMetricsLoopGeneric] Exiting update metrics loop: %v", getFuncName(updateFunc))
				}
				ticker.Stop()
				return
			}
		}
	}()
}

func updateAllocatableCapacity(controller *XController) {
	res := controller.GetAllocatableCapacity()
	allocatableCapacityCpu.Set(res.MilliCPU)
	allocatableCapacityMemory.Set(res.Memory)
	allocatableCapacityGpu.Set(float64(res.GPU))
}

func updateQueue(controller *XController) {
	awList, err := controller.appWrapperLister.List(labels.Everything())

	if err != nil {
		klog.Errorf("[updateQueue] Unable to obtain the list of AppWrappers: %+v", err)
		return
	}
	stateToAppWrapperCount := map[arbv1.AppWrapperState]int{
		arbv1.AppWrapperStateEnqueued:              0,
		arbv1.AppWrapperStateActive:                0,
		arbv1.AppWrapperStateDeleted:               0,
		arbv1.AppWrapperStateFailed:                0,
		arbv1.AppWrapperStateCompleted:             0,
		arbv1.AppWrapperStateRunningHoldCompletion: 0,
	}
	for _, aw := range awList {
		state := aw.Status.State
		stateToAppWrapperCount[state]++
	}
	for state, count := range stateToAppWrapperCount {
		appWrappersCount.WithLabelValues(string(state)).Set(float64(count))
	}
}
