
package api

import (
	"github.com/prometheus/client_golang/prometheus"
	"math"

	"k8s.io/klog/v2"
)

const (
	BucketCount = 20  //Must be > 0
	tolerance = 0.1
)
type ResourceHistogram struct {
	MilliCPU *prometheus.Histogram
	Memory   *prometheus.Histogram
	GPU      *prometheus.Histogram
}

func NewResourceHistogram(min *Resource, max *Resource) *ResourceHistogram {

	start := max.MilliCPU
	width := 1.0
	count := 2
	diff := math.Abs(min.MilliCPU - max.MilliCPU)
	if diff >= tolerance {
		start = min.MilliCPU
		width = (diff/(BucketCount - 1))
		count = BucketCount + 1
	}
	klog.V(10).Infof("[NewResourceHistogram] Start histogram numbers for CPU: start=%f, width=%f, count=%d",
		start, width, count)
	millicpuHist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "millicpu",
		Buckets: prometheus.LinearBuckets(start, width, count),})

	start = max.Memory
	width = 1.0
	count = 2
	diff = math.Abs(min.Memory - max.Memory)
	if diff >= tolerance {
		start = min.Memory
		width = (diff/(BucketCount - 1))
		count = BucketCount + 1
	}
	klog.V(10).Infof("[NewResourceHistogram] Start histogram numbers for Memory: start=%f, width=%f, count=%d",
		start, width, count)
	memoryHist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "memory",
		Buckets: prometheus.LinearBuckets(start, width, count),})

	start = float64(max.GPU)
	width = 1.0
	count = 2
	diff = math.Abs(float64(min.GPU - max.GPU))
	if diff >= tolerance {
		start = float64(min.GPU)
		width = (diff/(BucketCount - 1))
		count = BucketCount + 1
	}
	klog.V(10).Infof("[NewResourceHistogram] Start histogram numbers for GPU: start=%f, width=%f, count=%d",
		start, width, count)
	gpuHist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "gpu",
		Buckets: prometheus.LinearBuckets(start, width, count),})

	rh := &ResourceHistogram{
		MilliCPU: &millicpuHist,
		Memory:   &memoryHist,
		GPU:      &gpuHist,
	}
	return rh
}

func (rh *ResourceHistogram) Observer(r *Resource) {
	(*rh.MilliCPU).Observe(r.MilliCPU)
	(*rh.Memory).Observe(r.Memory)
	(*rh.GPU).Observe(float64(r.GPU))
}


