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

package pod

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/informers"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clientset "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/maputils"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"
)

var queueJobName = "appwrapper.mcad.ibm.com"

const (
	// QueueJobNameLabel label string for queuejob name
	QueueJobNameLabel string = "appwrapper-name"
)

// QueueJobResPod Controller for QueueJob pods
type QueueJobResPod struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset

	// A TTLCache of pod creates/deletes each rc expects to see

	// A store of pods, populated by the podController
	podStore    corelisters.PodLister
	podInformer corev1informer.PodInformer

	podSynced func() bool

	// A counter that stores the current terminating pod no of QueueJob
	// this is used to avoid to re-create the pods of a QueueJob before
	// all the old resources are terminated
	deletedResourcesCounter *maputils.SyncCounterMap
	rtScheme                *runtime.Scheme
	jsonSerializer          *json.Serializer

	// Reference manager to manage membership of queuejob resource and its members
	// refManager queuejobresources.RefManager
	// A counter that store the current terminating pods no of QueueJob
	// this is used to avoid to re-create the pods of a QueueJob before
	// all the old pods are terminated
	deletedPodsCounter *maputils.SyncCounterMap
}

// Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypePod, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResPod(config)
	})
}

// NewQueueJobResPod Creates a new controller for QueueJob pods
func NewQueueJobResPod(config *rest.Config) queuejobresources.Interface {
	// create k8s clientset

	qjrPod := &QueueJobResPod{
		clients:            kubernetes.NewForConfigOrDie(config),
		arbclients:         clientset.NewForConfigOrDie(config),
		deletedPodsCounter: maputils.NewSyncCounterMap(),
	}

	// create informer for pod information
	qjrPod.podInformer = informers.NewSharedInformerFactory(qjrPod.clients, 0).Core().V1().Pods()
	qjrPod.podInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *v1.Pod:
					klog.V(6).Infof("[QueueJobResPod-FilterFunc] Filter pod name(%s) namespace(%s) status(%s)\n", t.Name, t.Namespace, t.Status.Phase)
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrPod.addPod,
				UpdateFunc: qjrPod.updatePod,
				DeleteFunc: qjrPod.deletePod,
			},
		})

	qjrPod.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrPod.rtScheme)

	qjrPod.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrPod.rtScheme, qjrPod.rtScheme)

	qjrPod.podStore = qjrPod.podInformer.Lister()
	qjrPod.podSynced = qjrPod.podInformer.Informer().HasSynced

	return qjrPod
}

// Run the main goroutine responsible for watching and pods.
func (qjrPod *QueueJobResPod) Run(stopCh <-chan struct{}) {
	qjrPod.podInformer.Informer().Run(stopCh)
}

func (qjrPod *QueueJobResPod) addPod(obj interface{}) {
	return
}

func (qjrPod *QueueJobResPod) updatePod(old, cur interface{}) {
	return
}

func (qjrPod *QueueJobResPod) deletePod(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
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

	// update delete pod counter for a QueueJob
	if len(pod.Labels) != 0 && len(pod.Labels[QueueJobNameLabel]) > 0 {
		qjrPod.deletedPodsCounter.DecreaseCounter(fmt.Sprintf("%s/%s", pod.Namespace, pod.Labels[QueueJobNameLabel]))
	}
}

// filterActivePods returns pods that have not terminated.
func filterActivePods(pods []*v1.Pod) []*v1.Pod {
	var result []*v1.Pod
	for _, p := range pods {
		if isPodActive(p) {
			result = append(result, p)
		} else {
			klog.V(4).Infof("Ignoring inactive pod %v/%v in state %v, deletion time %v",
				p.Namespace, p.Name, p.Status.Phase, p.DeletionTimestamp)
		}
	}
	return result
}

func isPodActive(p *v1.Pod) bool {
	return v1.PodSucceeded != p.Status.Phase &&
		v1.PodFailed != p.Status.Phase &&
		p.DeletionTimestamp == nil
}

func (qjrPod *QueueJobResPod) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {

	sel := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			queueJobName: queuejob.Name,
		},
	}
	selector, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return fmt.Errorf("couldn't convert QueueJob selector: %v", err)
	}
	// List all pods under QueueJob
	// pods, errt := qjrPod.podStore.Pods(queuejob.Namespace).List(selector)
	pods, errt := qjrPod.podStore.Pods("").List(selector)

	if errt != nil {
		return errt
	}

	running := int32(queuejobresources.FilterPods(pods, v1.PodRunning))
	podPhases := []v1.PodPhase{v1.PodRunning, v1.PodSucceeded}
	totalResourcesConsumedForPodPhases := clusterstateapi.EmptyResource()
	for _, phase := range podPhases {
		totalResourcesConsumedForPodPhases.Add(queuejobresources.GetPodResourcesByPhase(phase, pods))
	}
	pending := int32(queuejobresources.FilterPods(pods, v1.PodPending))
	succeeded := int32(queuejobresources.FilterPods(pods, v1.PodSucceeded))
	failed := int32(queuejobresources.FilterPods(pods, v1.PodFailed))
	podsConditionMap := queuejobresources.PendingPodsFailedSchd(pods)
	klog.V(10).Infof("[UpdateQueueJobStatus] There are %d pods of AppWrapper %s:  pending %d, running %d, succeeded %d, failed %d, pendingpodsfailedschd %d, total resource consumed %v",
		len(pods), queuejob.Name, pending, running, succeeded, failed, len(podsConditionMap), totalResourcesConsumedForPodPhases)

	queuejob.Status.Pending = pending
	queuejob.Status.Running = running
	queuejob.Status.Succeeded = succeeded
	queuejob.Status.Failed = failed
	// Total resources by all running pods
	queuejob.Status.TotalGPU = int32(totalResourcesConsumedForPodPhases.GPU)
	queuejob.Status.TotalCPU = int32(totalResourcesConsumedForPodPhases.MilliCPU)
	queuejob.Status.TotalMemory = int32(totalResourcesConsumedForPodPhases.Memory)

	queuejob.Status.PendingPodConditions = nil
	for podName, cond := range podsConditionMap {
		podCond := GeneratePodFailedCondition(podName, cond)
		queuejob.Status.PendingPodConditions = append(queuejob.Status.PendingPodConditions, podCond)
	}

	return nil
}

// GeneratePodFailedCondition returns condition of a AppWrapper condition.
func GeneratePodFailedCondition(podName string, podCondition []v1.PodCondition) arbv1.PendingPodSpec {
	return arbv1.PendingPodSpec{
		PodName:    podName,
		Conditions: podCondition,
	}
}
