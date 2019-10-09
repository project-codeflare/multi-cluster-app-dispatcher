/*
Copyright 2017 The Kubernetes Authors.

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
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	client "github.com/kubernetes-sigs/kube-batch/pkg/client/clientset/controller-versioned/clients"
	informerfactory "github.com/kubernetes-sigs/kube-batch/pkg/client/informers/controller-externalversion"
	arbclient "github.com/kubernetes-sigs/kube-batch/pkg/client/informers/controller-externalversion/v1"
	"github.com/kubernetes-sigs/kube-batch/pkg/controller/clusterstate/api"
)

// New returns a Cache implementation.
func New(config *rest.Config) Cache {
	return newClusterStateCache(config)
}

type ClusterStateCache struct {
	sync.Mutex

	kubeclient *kubernetes.Clientset

	podInformer            clientv1.PodInformer
	nodeInformer           clientv1.NodeInformer
	schedulingSpecInformer arbclient.SchedulingSpecInformer

	Jobs  map[api.JobID]*api.JobInfo
	Nodes map[string]*api.NodeInfo

	availableResources *api.Resource
	deletedJobs *cache.FIFO

	errTasks    *cache.FIFO

}
func taskKey(obj interface{}) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("the object is nil")
	}

	task, ok := obj.(*api.TaskInfo)

	if !ok {
		return "", fmt.Errorf("failed to convert %v to TaskInfo", obj)
	}

	return string(task.UID), nil
}

func jobKey(obj interface{}) (string, error) {
	if obj == nil {
		return "", fmt.Errorf("the object is nil")
	}

	job, ok := obj.(*api.JobInfo)

	if !ok {
		return "", fmt.Errorf("failed to convert %v to TaskInfo", obj)
	}

	return string(job.UID), nil
}

func newClusterStateCache(config *rest.Config) *ClusterStateCache {
	sc := &ClusterStateCache{
		Jobs:        make(map[api.JobID]*api.JobInfo),
		Nodes:       make(map[string]*api.NodeInfo),
		errTasks:    cache.NewFIFO(taskKey),
		deletedJobs: cache.NewFIFO(jobKey),

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

	// create informer for pod information
	sc.podInformer = informerFactory.Core().V1().Pods()
	sc.podInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.Pod:
					pod := obj.(*v1.Pod)
					return pod.Status.Phase == v1.PodRunning
					//if pod.Status.Phase != v1.PodSucceeded && pod.Status.Phase != v1.PodFailed {
					//	return true
					//} else {
					//	return false
					//}
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    sc.AddPod,
				UpdateFunc: sc.UpdatePod,
				DeleteFunc: sc.DeletePod,
			},
		})

	// create queue informer
	queueClient, _, err := client.NewClient(config)
	if err != nil {
		panic(err)
	}

	schedulingSpecInformerFactory := informerfactory.NewSharedInformerFactory(queueClient, 0)
	// create informer for Queue information
	sc.schedulingSpecInformer = schedulingSpecInformerFactory.SchedulingSpec().SchedulingSpecs()
	sc.schedulingSpecInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sc.AddSchedulingSpec,
		UpdateFunc: sc.UpdateSchedulingSpec,
		DeleteFunc: sc.DeleteSchedulingSpec,
	})

	sc.availableResources = api.EmptyResource()

	return sc
}

func (sc *ClusterStateCache) Run(stopCh <-chan struct{}) {
	glog.V(8).Infof("Cluster State Cache started.")

	go sc.podInformer.Informer().Run(stopCh)
	go sc.nodeInformer.Informer().Run(stopCh)
	go sc.schedulingSpecInformer.Informer().Run(stopCh)

	// Update cache
	go sc.updateCache()

	// Re-sync error tasks.
	go sc.resync()

	// Cleanup jobs.
	go sc.cleanupJobs()

}

func (sc *ClusterStateCache) WaitForCacheSync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh,
		sc.podInformer.Informer().HasSynced,
		sc.schedulingSpecInformer.Informer().HasSynced,
		sc.nodeInformer.Informer().HasSynced)
}


// Gets available free resoures.
func (sc *ClusterStateCache) GetUnallocatedResources() *api.Resource {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	r := api.EmptyResource()
	return r.Add(sc.availableResources)
}

// Gets available free resoures.
func (sc *ClusterStateCache) saveState(r *api.Resource) error {
	glog.V(12).Infof("Saving Cluster State")

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()
	sc.availableResources.Replace(r)
	glog.V(12).Infof("Updated Cluster State completed.")
	return nil
}

// Gets available free resoures.
func (sc *ClusterStateCache) updateState() error {
	glog.V(11).Infof("Calculating Cluster State")

	cluster := sc.Snapshot()
	total := api.EmptyResource()
	used := api.EmptyResource()
	idle := api.EmptyResource()

	for _, value := range cluster.Nodes {
		total = total.Add(value.Allocatable)
		used = used.Add(value.Used)
		idle = idle.Add(value.Idle)
	}
	glog.V(8).Infof("Total capacity %+v, used %+v, free space %+v", total, used, idle)

	err := sc.saveState(idle)
	return err
}


func (sc *ClusterStateCache) deleteJob(job *api.JobInfo) {
	glog.V(3).Infof("Attempting to delete Job <%v:%v/%v>", job.UID, job.Namespace, job.Name)

	time.AfterFunc(5*time.Second, func() {
		sc.deletedJobs.AddIfNotPresent(job)
	})
}

func (sc *ClusterStateCache) processCleanupJob() error {
	_, err := sc.deletedJobs.Pop(func(obj interface{}) error {
		job, ok := obj.(*api.JobInfo)
		if !ok {
			return fmt.Errorf("failed to convert %v to *v1.Pod", obj)
		}

		func() {
			sc.Mutex.Lock()
			defer sc.Mutex.Unlock()

			if api.JobTerminated(job) {
				delete(sc.Jobs, job.UID)
				glog.V(3).Infof("Job <%v:%v/%v> was deleted.", job.UID, job.Namespace, job.Name)
			} else {
				// Retry
				sc.deleteJob(job)
			}
		}()

		return nil
	})

	return err
}

func (sc *ClusterStateCache) cleanupJobs() {
	for {
		err := sc.processCleanupJob()
		if err != nil {
			glog.Errorf("Failed to process job clean up: %v", err)
		}
	}
}

func (sc *ClusterStateCache) updateCache() {
	glog.V(9).Infof("Starting to update Cluster State Cache")

	for {
		err := sc.updateState()
		if err != nil {
			glog.Errorf("Failed update state: %v", err)
		}

		time.Sleep(3 * time.Second)
	}
}

func (sc *ClusterStateCache) resync() {
	for {
		err := sc.processResyncTask()
		if err != nil {
			glog.Errorf("Failed to process resync: %v", err)
		}
	}
}

func (sc *ClusterStateCache) processResyncTask() error {
	_, err := sc.errTasks.Pop(func(obj interface{}) error {
		task, ok := obj.(*api.TaskInfo)
		if !ok {
			return fmt.Errorf("failed to convert %v to *v1.Pod", obj)
		}

		if err := sc.syncTask(task); err != nil {
			glog.Errorf("Failed to sync pod <%v/%v>", task.Namespace, task.Name)
			return err
		}
		return nil
	})

	return err
}

func (sc *ClusterStateCache) Snapshot() *api.ClusterInfo {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	snapshot := &api.ClusterInfo{
		Nodes: make([]*api.NodeInfo, 0, len(sc.Nodes)),
		Jobs:  make([]*api.JobInfo, 0, len(sc.Jobs)),
	}

	for _, value := range sc.Nodes {
		snapshot.Nodes = append(snapshot.Nodes, value.Clone())
	}

	for _, value := range sc.Jobs {
		// If no scheduling spec, does not handle it.
		if value.SchedSpec == nil && value.PDB == nil {
			glog.V(3).Infof("The scheduling spec of Job <%v> is nil, ignore it.", value.UID)
			continue
		}

		snapshot.Jobs = append(snapshot.Jobs, value.Clone())
	}

	return snapshot
}

func (sc *ClusterStateCache) LoadConf(path string) (map[string]string, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(path)
	if err != nil {
		return nil, err
	}

	confMap, err := sc.kubeclient.CoreV1().ConfigMaps(ns).Get(name, metav1.GetOptions{})
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
			str = str + fmt.Sprintf("\t %s: idle(%v) used(%v) allocatable(%v) pods(%d)\n",
				n.Name, n.Idle, n.Used, n.Allocatable, len(n.Tasks))

			i := 0
			for _, p := range n.Tasks {
				str = str + fmt.Sprintf("\t\t %d: %v\n", i, p)
				i++
			}
		}
	}

	if len(sc.Jobs) != 0 {
		str = str + "Jobs:\n"
		for _, job := range sc.Jobs {
			str = str + fmt.Sprintf("\t Job(%s) name(%s) minAvailable(%v)\n",
				job.UID, job.Name, job.MinAvailable)

			i := 0
			for _, task := range job.Tasks {
				str = str + fmt.Sprintf("\t\t %d: %v\n", i, task)
				i++
			}
		}
	}

	return str
}
