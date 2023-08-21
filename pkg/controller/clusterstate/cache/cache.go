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

package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	dto "github.com/prometheus/client_model/go"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
)

// New returns a Cache implementation.
func New(config *rest.Config) Cache {
	return newClusterStateCache(config)
}

type ClusterStateCache struct {
	sync.Mutex

	kubeclient *kubernetes.Clientset

	nodeInformer clientv1.NodeInformer

	Nodes map[string]*api.NodeInfo

	availableResources *api.Resource
	availableHistogram *api.ResourceHistogram
	resourceCapacities *api.Resource
}

func newClusterStateCache(config *rest.Config) *ClusterStateCache {
	sc := &ClusterStateCache{
		Nodes: make(map[string]*api.NodeInfo),
	}

	sc.kubeclient = kubernetes.NewForConfigOrDie(config)

	informerFactory := informers.NewSharedInformerFactory(sc.kubeclient, 0)

	// create informer for node information
	sc.nodeInformer = informerFactory.Core().V1().Nodes()
	sc.nodeInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.AddNode,
			UpdateFunc: sc.UpdateNode,
			DeleteFunc: sc.DeleteNode,
		},
		0,
	)

	sc.availableResources = api.EmptyResource()
	sc.availableHistogram = api.NewResourceHistogram(api.EmptyResource(), api.EmptyResource())
	sc.resourceCapacities = api.EmptyResource()

	return sc
}

func (sc *ClusterStateCache) Run(stopCh <-chan struct{}) {
	klog.V(8).Infof("Cluster State Cache started.")
	//go sc.nodeInformer.Informer().Run(stopCh)

	// Update cache
	//go wait.Until(sc.updateCache, 0, stopCh)

}

func (sc *ClusterStateCache) WaitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh,
		sc.nodeInformer.Informer().HasSynced)
}

// Gets available free resoures.
func (sc *ClusterStateCache) GetUnallocatedResources() *api.Resource {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	r := api.EmptyResource()
	return r.Add(sc.availableResources)
}

func (sc *ClusterStateCache) GetUnallocatedHistograms() map[string]*dto.Metric {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	rtval := make(map[string]*dto.Metric)
	rtcpu := &dto.Metric{}
	(*sc.availableHistogram.GPU).Write(rtcpu)
	rtval["gpu"] = rtcpu
	return rtval
}

// Gets available free resoures.
func (sc *ClusterStateCache) GetUnallocatedResourcesHistogram() *api.ResourceHistogram {
	return sc.availableHistogram
}

// Gets the full capacity of resources in the cluster
func (sc *ClusterStateCache) GetResourceCapacities() *api.Resource {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	r := api.EmptyResource()
	return r.Add(sc.resourceCapacities)
}

// Save the cluster state.
func (sc *ClusterStateCache) saveState(available *api.Resource, capacity *api.Resource,
	availableHistogram *api.ResourceHistogram) error {
	klog.V(12).Infof("Saving Cluster State")

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()
	sc.availableResources.Replace(available)
	sc.resourceCapacities.Replace(capacity)
	sc.availableHistogram = availableHistogram
	klog.V(12).Infof("Updated Cluster State completed.")
	return nil
}

// Gets available free resoures.
func (sc *ClusterStateCache) updateState() error {
	klog.V(11).Infof("Calculating Cluster State")

	cluster := sc.Snapshot()
	total := api.EmptyResource()
	used := api.EmptyResource()
	idle := api.EmptyResource()
	idleMin := api.EmptyResource()
	idleMax := api.EmptyResource()

	firstNode := true
	for _, value := range cluster.Nodes {
		// Do not use any Unschedulable nodes in calculations
		if value.Unschedulable == true {
			klog.V(6).Infof("[updateState] %s is marked as unschedulable node Total: %v, Used: %v, and Idle: %v will not be included in cluster state calculation.",
				value.Name, value.Allocatable, value.Used, value.Idle)
			continue
		}

		total = total.Add(value.Allocatable)
		used = used.Add(value.Used)
		idle = idle.Add(value.Idle)

		// Collect Min and Max for histogram
		if firstNode {
			idleMin.MilliCPU = idle.MilliCPU
			idleMin.Memory = idle.Memory
			idleMin.GPU = idle.GPU

			idleMax.MilliCPU = idle.MilliCPU
			idleMax.Memory = idle.Memory
			idleMax.GPU = idle.GPU
			firstNode = false
		} else {
			if value.Idle.MilliCPU < idleMin.MilliCPU {
				idleMin.MilliCPU = value.Idle.MilliCPU
			} else if value.Idle.MilliCPU > idleMax.MilliCPU {
				idleMax.MilliCPU = value.Idle.MilliCPU
			}

			if value.Idle.Memory < idleMin.Memory {
				idleMin.Memory = value.Idle.Memory
			} else if value.Idle.Memory > idleMax.Memory {
				idleMax.Memory = value.Idle.Memory
			}

			if value.Idle.GPU < idleMin.GPU {
				idleMin.GPU = value.Idle.GPU
			} else if value.Idle.GPU > idleMax.GPU {
				idleMax.GPU = value.Idle.GPU
			}
		}
	}

	// Create available histograms
	newIdleHistogram := api.NewResourceHistogram(idleMin, idleMax)
	for _, value := range cluster.Nodes {
		newIdleHistogram.Observer(value.Idle)
	}

	klog.V(8).Infof("Total capacity %+v, used %+v, free space %+v", total, used, idle)
	if klog.V(12).Enabled() {
		// CPU histogram
		metricCPU := &dto.Metric{}
		(*newIdleHistogram.MilliCPU).Write(metricCPU)
		klog.V(12).Infof("[updateState] CPU histogram:\n%s", proto.MarshalTextString(metricCPU))

		// Memory histogram
		metricMem := &dto.Metric{}
		(*newIdleHistogram.Memory).Write(metricMem)
		klog.V(12).Infof("[updateState] Memory histogram:\n%s", proto.MarshalTextString(metricMem))

		// GPU histogram
		metricGPU := &dto.Metric{}
		(*newIdleHistogram.GPU).Write(metricGPU)
		klog.V(12).Infof("[updateState] GPU histogram:\n%s", proto.MarshalTextString(metricGPU))
	}

	err := sc.saveState(idle, total, newIdleHistogram)
	return err
}

func (sc *ClusterStateCache) updateCache() {
	klog.V(9).Infof("Starting to update Cluster State Cache")
	err := sc.updateState()
	if err != nil {
		klog.Errorf("Failed update state: %v", err)
	}
	return
}

func (sc *ClusterStateCache) Snapshot() *api.ClusterInfo {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	snapshot := &api.ClusterInfo{
		Nodes: make([]*api.NodeInfo, 0, len(sc.Nodes)),
	}

	for _, value := range sc.Nodes {
		snapshot.Nodes = append(snapshot.Nodes, value.Clone())
	}

	return snapshot
}

func (sc *ClusterStateCache) LoadConf(path string) (map[string]string, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(path)
	if err != nil {
		return nil, err
	}

	confMap, err := sc.kubeclient.CoreV1().ConfigMaps(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return confMap.Data, nil
}

func (sc *ClusterStateCache) String() string {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	str := "Cache:\n"

	if len(sc.Nodes) != 0 {
		str = str + "Nodes:\n"
		for _, n := range sc.Nodes {
			str = str + fmt.Sprintf("\t %s: idle(%v) used(%v) allocatable(%v)\n",
				n.Name, n.Idle, n.Used, n.Allocatable)

		}
	}

	return str
}
