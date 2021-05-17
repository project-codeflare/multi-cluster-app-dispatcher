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
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/utils"
	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	arbapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
)

func isTerminated(status arbapi.TaskStatus) bool {
	return status == arbapi.Succeeded || status == arbapi.Failed
}

func (sc *ClusterStateCache) addTask(pi *arbapi.TaskInfo) error {
	if len(pi.Job) != 0 {
		if _, found := sc.Jobs[pi.Job]; !found {
			sc.Jobs[pi.Job] = arbapi.NewJobInfo(pi.Job)
		}
		klog.V(7).Infof("Adding task: %s to job: %v.", pi.Name, pi.Job)

		sc.Jobs[pi.Job].AddTaskInfo(pi)
	} else {
		klog.V(10).Infof("No job ID for task: %s.", pi.Name)

	}

	if len(pi.NodeName) != 0 {
		if _, found := sc.Nodes[pi.NodeName]; !found {
			sc.Nodes[pi.NodeName] = arbapi.NewNodeInfo(nil)
		}

		node := sc.Nodes[pi.NodeName]
		if !isTerminated(pi.Status) {
			klog.V(10).Infof("Adding Task: %s to node: %s.", pi.Name, pi.NodeName)
			return node.AddTask(pi)
		} else {
			klog.V(10).Infof("Task: %s is terminated.  Did not added not node: %s.", pi.Name, pi.NodeName)
		}
	} else {
		klog.V(10).Infof("No related node found for for task: %s.", pi.Name)

	}

	return nil
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) addPod(pod *v1.Pod) error {
	klog.V(9).Infof("Attempting to add pod: %s.", pod.Name)
	pi := arbapi.NewTaskInfo(pod)
	klog.V(10).Infof("New task: %s created for pod %s add with job id: %v", pi.Name, pod.Name, pi.Job)

	return sc.addTask(pi)
}

func (sc *ClusterStateCache) syncTask(oldTask *arbapi.TaskInfo) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	klog.V(9).Infof("Attempting to sync task: %s.", oldTask.Name)

	newPod, err := sc.kubeclient.CoreV1().Pods(oldTask.Namespace).Get(context.Background(), oldTask.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			sc.deleteTask(oldTask)
			klog.V(3).Infof("Pod <%v/%v> was deleted, removed from cache.", oldTask.Namespace, oldTask.Name)

			return nil
		}
		return fmt.Errorf("failed to get Pod <%v/%v>: err %v", oldTask.Namespace, oldTask.Name, err)
	}

	newTask := arbapi.NewTaskInfo(newPod)

	return sc.updateTask(oldTask, newTask)
}

func (sc *ClusterStateCache) updateTask(oldTask, newTask *arbapi.TaskInfo) error {
	klog.V(9).Infof("Updating.task: %s", oldTask.Name)
	if err := sc.deleteTask(oldTask); err != nil {
		return err
	}

	return sc.addTask(newTask)
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) updatePod(oldPod, newPod *v1.Pod) error {
	klog.V(9).Infof("Attempting to update pod: %s.", oldPod.Name)
	if err := sc.deletePod(oldPod); err != nil {
		return err
	}
	return sc.addPod(newPod)
}

func (sc *ClusterStateCache) deleteTask(pi *arbapi.TaskInfo) error {
	var jobErr, nodeErr error

	klog.V(9).Infof("Attempting to delete task: %s.", pi.Name)

	if len(pi.Job) != 0 {
		if job, found := sc.Jobs[pi.Job]; found {
			jobErr = job.DeleteTaskInfo(pi)
		} else {
			jobErr = fmt.Errorf("failed to find Job <%v> for Task %v/%v",
				pi.Job, pi.Namespace, pi.Name)
		}
	} else {
		klog.V(9).Infof("Job ID for task: %s is empty.", pi.Name)

	}

	if len(pi.NodeName) != 0 {
		node := sc.Nodes[pi.NodeName]
		if node != nil {
			nodeErr = node.RemoveTask(pi)
		}
	} else {
		klog.V(9).Infof("No node name for task: %s is found.", pi.Name)

	}

	if jobErr != nil || nodeErr != nil {
		return arbapi.MergeErrors(jobErr, nodeErr)
	}

	return nil
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) deletePod(pod *v1.Pod) error {
	klog.V(10).Infof("Attempting to delete pod: %s.", pod.Name)

	pi := arbapi.NewTaskInfo(pod)
	// Delete the Task in cache to handle Binding status.
	task := pi
	if job, found := sc.Jobs[pi.Job]; found {
		klog.V(9).Infof("Found job %v to delete for pod: %s, task: %s.", job.UID, pod.Name, pi.Name)
		if t, found := job.Tasks[pi.UID]; found {
			klog.V(10).Infof("Found job task listed in job: %v.", job.UID)
			task = t
		}
	}
	if err := sc.deleteTask(task); err != nil {
		return err
	}

	// If job was terminated, delete it.
	if job, found := sc.Jobs[pi.Job]; found && arbapi.JobTerminated(job) {
		sc.deleteJob(job)
	}

	return nil
}

func (sc *ClusterStateCache) AddPod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert to *v1.Pod: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.addPod(pod)
	if err != nil {
		klog.Errorf("Failed to add pod <%s/%s> into cache: %v",
			pod.Namespace, pod.Name, err)
		return
	} else {
		klog.V(4).Infof("[AddPod] Added pod <%s/%v> into cache.", pod.Namespace, pod.Name)
	}
	return
}

func (sc *ClusterStateCache) UpdatePod(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.updatePod(oldPod, newPod)
	if err != nil {
		klog.Errorf("Failed to update pod %v in cache: %v", oldPod.Name, err)
		return
	}

	klog.V(4).Infof("[UpdatePod] Updated pod <%s/%v> in cache.", oldPod.Namespace, oldPod.Name)

	return
}

func (sc *ClusterStateCache) DeletePod(obj interface{}) {

	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
		klog.V(10).Infof("Handling Delete pod event for: %s.", pod.Name)
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			klog.Errorf("Cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1.Pod: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deletePod(pod)
	if err != nil {
		klog.V(6).Infof("Failed to delete pod %v from cache: %v", pod.Name, err)
		return
	}

	klog.V(3).Infof("Deleted pod <%s/%v> from cache.", pod.Namespace, pod.Name)
	return
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) addNode(node *v1.Node) error {
	if sc.Nodes[node.Name] != nil {
		sc.Nodes[node.Name].SetNode(node)
	} else {
		sc.Nodes[node.Name] = arbapi.NewNodeInfo(node)
	}
	klog.V(10).Infof("Node %s added to cache.", node.Name)

	return nil
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) updateNode(oldNode, newNode *v1.Node) error {
	// Did not delete the old node, just update related info, e.g. allocatable.
	if sc.Nodes[newNode.Name] != nil {
		sc.Nodes[newNode.Name].SetNode(newNode)
		return nil
	}

	return fmt.Errorf("node <%s> does not exist", newNode.Name)
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) deleteNode(node *v1.Node) error {
	if _, ok := sc.Nodes[node.Name]; !ok {
		return fmt.Errorf("node <%s> does not exist", node.Name)
	}
	delete(sc.Nodes, node.Name)
	return nil
}

func (sc *ClusterStateCache) AddNode(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		klog.Errorf("Cannot convert to *v1.Node: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.addNode(node)
	if err != nil {
		klog.Errorf("Failed to add node %s into cache: %v", node.Name, err)
		return
	}
	return
}

func (sc *ClusterStateCache) UpdateNode(oldObj, newObj interface{}) {
	klog.V(10).Infof("Entered UpdateNode()")
	oldNode, ok := oldObj.(*v1.Node)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *v1.Node: %v", oldObj)
		return
	}
	newNode, ok := newObj.(*v1.Node)
	if !ok {
		klog.Errorf("Cannot convert newObj to *v1.Node: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.updateNode(oldNode, newNode)
	if err != nil {
		klog.Errorf("Failed to update node %v in cache: %v", oldNode.Name, err)
		return
	}
	return
}

func (sc *ClusterStateCache) DeleteNode(obj interface{}) {
	var node *v1.Node
	switch t := obj.(type) {
	case *v1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*v1.Node)
		if !ok {
			klog.Errorf("Cannot convert to *v1.Node: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *v1.Node: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deleteNode(node)
	if err != nil {
		klog.Errorf("Failed to delete node %s from cache: %v", node.Name, err)
		return
	}
	return
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) setSchedulingSpec(ss *arbv1.SchedulingSpec) error {
	job := arbapi.JobID(utils.GetController(ss))

	if len(job) == 0 {
		return fmt.Errorf("the controller of SchedulingSpec is empty")
	}

	if _, found := sc.Jobs[job]; !found {
		sc.Jobs[job] = arbapi.NewJobInfo(job)
	}

	sc.Jobs[job].SetSchedulingSpec(ss)

	return nil
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) updateSchedulingSpec(oldQueue, newQueue *arbv1.SchedulingSpec) error {
	return sc.setSchedulingSpec(newQueue)
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) deleteSchedulingSpec(ss *arbv1.SchedulingSpec) error {
	jobID := arbapi.JobID(utils.GetController(ss))

	job, found := sc.Jobs[jobID]
	if !found {
		return fmt.Errorf("can not found job %v:%v/%v", jobID, ss.Namespace, ss.Name)
	}

	// Unset SchedulingSpec
	job.UnsetSchedulingSpec()
	sc.deleteJob(job)

	return nil
}

func (sc *ClusterStateCache) AddSchedulingSpec(obj interface{}) {
	ss, ok := obj.(*arbv1.SchedulingSpec)
	if !ok {
		klog.Errorf("Cannot convert to *arbv1.Queue: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	klog.V(4).Infof("Add SchedulingSpec(%s) into cache, spec(%#v)", ss.Name, ss.Spec)
	err := sc.setSchedulingSpec(ss)
	if err != nil {
		klog.Errorf("Failed to add SchedulingSpec %s into cache: %v", ss.Name, err)
		return
	}
	return
}

func (sc *ClusterStateCache) UpdateSchedulingSpec(oldObj, newObj interface{}) {
	oldSS, ok := oldObj.(*arbv1.SchedulingSpec)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *arbv1.SchedulingSpec: %v", oldObj)
		return
	}
	newSS, ok := newObj.(*arbv1.SchedulingSpec)
	if !ok {
		klog.Errorf("Cannot convert newObj to *arbv1.SchedulingSpec: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.updateSchedulingSpec(oldSS, newSS)
	if err != nil {
		klog.Errorf("Failed to update SchedulingSpec %s into cache: %v", oldSS.Name, err)
		return
	}
	return
}

func (sc *ClusterStateCache) DeleteSchedulingSpec(obj interface{}) {
	var ss *arbv1.SchedulingSpec
	switch t := obj.(type) {
	case *arbv1.SchedulingSpec:
		ss = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		ss, ok = t.Obj.(*arbv1.SchedulingSpec)
		if !ok {
			klog.Errorf("Cannot convert to *arbv1.SchedulingSpec: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *arbv1.SchedulingSpec: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deleteSchedulingSpec(ss)
	if err != nil {
		klog.Errorf("Failed to delete SchedulingSpec %s from cache: %v", ss.Name, err)
		return
	}
	return
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) setPDB(pdb *policyv1.PodDisruptionBudget) error {
	job := arbapi.JobID(utils.GetController(pdb))

	if len(job) == 0 {
		return fmt.Errorf("the controller of PodDisruptionBudget is empty")
	}

	if _, found := sc.Jobs[job]; !found {
		sc.Jobs[job] = arbapi.NewJobInfo(job)
	}

	sc.Jobs[job].SetPDB(pdb)

	return nil
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) updatePDB(oldPDB, newPDB *policyv1.PodDisruptionBudget) error {
	return sc.setPDB(newPDB)
}

// Assumes that lock is already acquired.
func (sc *ClusterStateCache) deletePDB(pdb *policyv1.PodDisruptionBudget) error {
	jobID := arbapi.JobID(utils.GetController(pdb))

	job, found := sc.Jobs[jobID]
	if !found {
		return fmt.Errorf("can not found job %v:%v/%v", jobID, pdb.Namespace, pdb.Name)
	}

	// Unset SchedulingSpec
	job.UnsetPDB()
	sc.deleteJob(job)

	return nil
}

func (sc *ClusterStateCache) AddPDB(obj interface{}) {
	pdb, ok := obj.(*policyv1.PodDisruptionBudget)
	if !ok {
		klog.Errorf("Cannot convert to *policyv1.PodDisruptionBudget: %v", obj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.setPDB(pdb)
	if err != nil {
		klog.Errorf("Failed to add PodDisruptionBudget %s into cache: %v", pdb.Name, err)
		return
	}
	return
}

func (sc *ClusterStateCache) UpdatePDB(oldObj, newObj interface{}) {
	oldPDB, ok := oldObj.(*policyv1.PodDisruptionBudget)
	if !ok {
		klog.Errorf("Cannot convert oldObj to *policyv1.PodDisruptionBudget: %v", oldObj)
		return
	}
	newPDB, ok := newObj.(*policyv1.PodDisruptionBudget)
	if !ok {
		klog.Errorf("Cannot convert newObj to *policyv1.PodDisruptionBudget: %v", newObj)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.updatePDB(oldPDB, newPDB)
	if err != nil {
		klog.Errorf("Failed to update PodDisruptionBudget %s into cache: %v", oldPDB.Name, err)
		return
	}
	return
}

func (sc *ClusterStateCache) DeletePDB(obj interface{}) {
	var pdb *policyv1.PodDisruptionBudget
	switch t := obj.(type) {
	case *policyv1.PodDisruptionBudget:
		pdb = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pdb, ok = t.Obj.(*policyv1.PodDisruptionBudget)
		if !ok {
			klog.Errorf("Cannot convert to *policyv1.PodDisruptionBudget: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to *policyv1.PodDisruptionBudget: %v", t)
		return
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	err := sc.deletePDB(pdb)
	if err != nil {
		klog.Errorf("Failed to delete PodDisruptionBudget %s from cache: %v", pdb.Name, err)
		return
	}
	return
}
