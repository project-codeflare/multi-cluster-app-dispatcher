/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

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

package provider

import (
	"context"
	"strings"
	"sync"

	"k8s.io/klog/v2"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/metrics/pkg/apis/external_metrics"

	clusterstatecache "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/cache"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"
)

// CustomMetricResource wraps provider.CustomMetricInfo in a struct which stores the Name and Namespace of the resource
// So that we can accurately store and retrieve the metric as if this were an actual metrics server.
type CustomMetricResource struct {
	provider.CustomMetricInfo
	types.NamespacedName
}

// externalMetric provides examples for metrics which would otherwise be reported from an external source
// TODO (damemi): add dynamic external metrics instead of just hardcoded examples
type ExternalMetric struct {
	info   provider.ExternalMetricInfo
	labels map[string]string
	Value  external_metrics.ExternalMetricValue
}

var (
	defaultValueExternalMetrics = []ExternalMetric{
		{
			info: provider.ExternalMetricInfo{
				Metric: "cluster-external-metric",
			},
			labels: map[string]string{"cluster": "cpu"},
			Value: external_metrics.ExternalMetricValue{
				MetricName: "cluster-external-metric",
				MetricLabels: map[string]string{
					"cluster": "cpu",
				},
				Value: *resource.NewQuantity(0, resource.DecimalSI),
			},
		},
		{
			info: provider.ExternalMetricInfo{
				Metric: "cluster-external-metric",
			},
			labels: map[string]string{"cluster": "memory"},
			Value: external_metrics.ExternalMetricValue{
				MetricName: "cluster-external-metric",
				MetricLabels: map[string]string{
					"cluster": "memory",
				},
				Value: *resource.NewQuantity(0, resource.DecimalSI),
			},
		},
		{
			info: provider.ExternalMetricInfo{
				Metric: "cluster-external-metric",
			},
			labels: map[string]string{"cluster": "gpu"},
			Value: external_metrics.ExternalMetricValue{
				MetricName: "cluster-external-metric",
				MetricLabels: map[string]string{
					"cluster": "gpu",
				},
				Value: *resource.NewQuantity(0, resource.DecimalSI),
			},
		},
		{
			info: provider.ExternalMetricInfo{
				Metric: "other-external-metric",
			},
			labels: map[string]string{},
			Value: external_metrics.ExternalMetricValue{
				MetricName:   "other-external-metric",
				MetricLabels: map[string]string{},
				Value:        *resource.NewQuantity(44, resource.DecimalSI),
			},
		},
	}
)

type metricValue struct {
	labels labels.Set
	value  resource.Quantity
}

// clusterMetricsProvider is a sample implementation of provider.ExternalMetricsProvider
type clusterMetricsProvider struct {
	client dynamic.Interface
	mapper apimeta.RESTMapper

	valuesLock      sync.RWMutex
	values          map[CustomMetricResource]metricValue
	externalMetrics []ExternalMetric
	cache2          clusterstatecache.Cache
}

var _ provider.ExternalMetricsProvider = (*clusterMetricsProvider)(nil)

// NewFakeProvider returns an instance of clusterMetricsProvider
func NewFakeProvider(client dynamic.Interface, mapper apimeta.RESTMapper, clusterStateCache clusterstatecache.Cache) (provider.ExternalMetricsProvider) {
	klog.V(10).Infof("[NewFakeProvider] Entered NewFakeProvider()")
	provider := &clusterMetricsProvider{
		client:          client,
		mapper:          mapper,
		values:          make(map[CustomMetricResource]metricValue),
		externalMetrics: defaultValueExternalMetrics,
		cache2:          clusterStateCache,
	}
	return provider
}

func (p *clusterMetricsProvider) GetExternalMetric(_ context.Context, namespace string, metricSelector labels.Selector,
	info provider.ExternalMetricInfo) (*external_metrics.ExternalMetricValueList, error) {
	klog.V(10).Infof("[GetExternalMetric] Entered GetExternalMetric()")
	klog.V(9).Infof("[GetExternalMetric] metricsSelector: %s, metricsInfo: %s", metricSelector.String(), info.Metric)
	p.valuesLock.RLock()
	defer p.valuesLock.RUnlock()

	var matchingMetrics []external_metrics.ExternalMetricValue
	for _, metric := range p.externalMetrics {
		klog.V(9).Infof("[GetExternalMetric] externalMetricsInfo: %s, externalMetricValue: %v, externalMetricLabels: %v ",
			metric.info.Metric, metric.Value, metric.labels)
		if metric.info.Metric == info.Metric && metricSelector.Matches(labels.Set(metric.labels)) {
			metricValue := metric.Value
			labelVal := metric.labels["cluster"]
			klog.V(9).Infof("[GetExternalMetric] cluster label value: %s, ", labelVal)
			// Set memory Value
			if strings.Compare(labelVal, "memory") == 0 {
				resources := p.cache2.GetUnallocatedResources()
				klog.V(9).Infof("[GetExternalMetric] Cache resources: %v", resources)

				klog.V(10).Infof("[GetExternalMetric] Setting memory metric Value: %f.", resources.Memory)
				metricValue.Value = *resource.NewQuantity(int64(resources.Memory), resource.DecimalSI)
				// metricValue.Value = *resource.NewQuantity(4500000000, resource.DecimalSI)
			} else if strings.Compare(labelVal, "cpu") == 0 {
				// Set cpu Value
				resources := p.cache2.GetUnallocatedResources()
				klog.V(9).Infof("[GetExternalMetric] Cache resources: %f", resources)

				klog.V(10).Infof("[GetExternalMetric] Setting cpu metric Value: %v.", resources.MilliCPU)
				metricValue.Value = *resource.NewQuantity(int64(resources.MilliCPU), resource.DecimalSI)
			} else if strings.Compare(labelVal, "gpu") == 0 {
				// Set gpu Value
				resources := p.cache2.GetUnallocatedResources()
				klog.V(9).Infof("[GetExternalMetric] Cache resources: %f", resources)

				klog.V(10).Infof("[GetExternalMetric] Setting gpu metric Value: %v.", resources.GPU)
				metricValue.Value = *resource.NewQuantity(resources.GPU, resource.DecimalSI)
			} else {
				klog.V(10).Infof("[GetExternalMetric] Not setting cpu/memory metric Value")
			}

			metricValue.Timestamp = metav1.Now()
			matchingMetrics = append(matchingMetrics, metricValue)
		}
	}
	return &external_metrics.ExternalMetricValueList{
		Items: matchingMetrics,
	}, nil
}

func (p *clusterMetricsProvider) ListAllExternalMetrics() []provider.ExternalMetricInfo {
	klog.V(10).Infof("Entered ListAllExternalMetrics()")
	p.valuesLock.RLock()
	defer p.valuesLock.RUnlock()

	var externalMetricsInfo []provider.ExternalMetricInfo
	for _, metric := range p.externalMetrics {
		externalMetricsInfo = append(externalMetricsInfo, metric.info)
		klog.V(9).Infof("Add metric=%v to externalMetricsInfo", metric)
	}
	klog.V(9).Infof("ExternalMetricsInfo=%v", externalMetricsInfo)
	return externalMetricsInfo
}
