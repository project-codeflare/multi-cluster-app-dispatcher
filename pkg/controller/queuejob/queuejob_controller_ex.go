/*
Copyright 2019, 2021, 2022 The Multi-Cluster App Dispatcher Authors.

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
	"context"
	jsons "encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/hashicorp/go-multierror"
	qmutils "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/util"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota/quotaforestmanager"
	dto "github.com/prometheus/client_model/go"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/metrics/adapter"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/genericresource"
	"k8s.io/apimachinery/pkg/labels"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clientset "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned"

	informerFactory "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions"
	arbinformers "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions/controller/v1beta1"

	arblisters "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1beta1"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobdispatch"

	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
)

// XController the AppWrapper Controller type
type XController struct {
	config       *rest.Config
	serverOption *options.ServerOption

	appwrapperInformer arbinformers.AppWrapperInformer
	// resources registered for the AppWrapper
	// qjobRegisteredResources queuejobresources.RegisteredResources
	// controllers for these resources
	// qjobResControls map[arbv1.ResourceType]queuejobresources.Interface

	// Captures all available resources in the cluster
	genericresources *genericresource.GenericResources

	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset

	// A store of jobs
	appWrapperLister arblisters.AppWrapperLister
	appWrapperSynced func() bool

	// QueueJobs that need to be initialized
	// Add labels and selectors to AppWrapper
	// initQueue *cache.FIFO

	// QueueJobs that need to sync up after initialization
	// updateQueue *cache.FIFO

	// eventQueue that need to sync up
	eventQueue *cache.FIFO

	// QJ queue that needs to be allocated
	qjqueue SchedulingQueue

	// TODO: Do we need this local cache?
	// our own local cache, used for computing total amount of resources
	// cache clusterstatecache.Cache

	// is dispatcher or deployer?
	isDispatcher bool

	// Agent map: agentID -> JobClusterAgent
	agentMap  map[string]*queuejobdispatch.JobClusterAgent
	agentList []string

	// Map for AppWrapper -> JobClusterAgent
	dispatchMap map[string]string

	// Metrics API Server
	metricsAdapter *adapter.MetricsAdapter

	// EventQueueforAgent
	agentEventQueue *cache.FIFO

	// Quota Manager
	quotaManager quota.QuotaManagerInterface

	// Active Scheduling AppWrapper
	schedulingAW    *arbv1.AppWrapper
	schedulingMutex sync.RWMutex
}

type JobAndClusterAgent struct {
	queueJobKey      string
	queueJobAgentKey string
}

// RegisterAllQueueJobResourceTypes - registers all resources
// func RegisterAllQueueJobResourceTypes(regs *queuejobresources.RegisteredResources) {
// 	respod.Register(regs)
// }

func GetQueueJobKey(obj interface{}) (string, error) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		return "", fmt.Errorf("not a AppWrapper")
	}

	return fmt.Sprintf("%s/%s", qj.Namespace, qj.Name), nil
}

// UpdateQueueJobStatus was part of pod informer, this is now a method of queuejob_controller file.
// This change is done in an effort to simplify the controller and enable to move to controller runtime.
func (qjm *XController) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {

	labelSelector := fmt.Sprintf("%s=%s", "appwrapper.mcad.ibm.com", queuejob.Name)
	pods, errt := qjm.clients.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if errt != nil {
		return errt
	}

	running := int32(FilterPods(pods.Items, v1.PodRunning))
	podPhases := []v1.PodPhase{v1.PodRunning, v1.PodSucceeded}
	totalResourcesConsumedForPodPhases := clusterstateapi.EmptyResource()
	for _, phase := range podPhases {
		totalResourcesConsumedForPodPhases.Add(GetPodResourcesByPhase(phase, pods.Items))
	}
	pending := int32(FilterPods(pods.Items, v1.PodPending))
	succeeded := int32(FilterPods(pods.Items, v1.PodSucceeded))
	failed := int32(FilterPods(pods.Items, v1.PodFailed))
	podsConditionMap := PendingPodsFailedSchd(pods.Items)
	klog.V(10).Infof("[UpdateQueueJobStatus] There are %d pods of AppWrapper %s:  pending %d, running %d, succeeded %d, failed %d, pendingpodsfailedschd %d, total resource consumed %v",
		len(pods.Items), queuejob.Name, pending, running, succeeded, failed, len(podsConditionMap), totalResourcesConsumedForPodPhases)

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

// allocatableCapacity calculates the capacity available on each node by substracting resources
// consumed by existing pods.
// For a large cluster with thousands of nodes and hundreds of thousands of pods this
// method could be a performance bottleneck
// We can then move this method to a seperate thread that basically runs every X interval and
// provides resources available to the next AW that needs to be dispatched.
// Obviously the thread would need locking and timer to expire cache.
// May be moved to controller runtime can help.
func (qjm *XController) allocatableCapacity() *clusterstateapi.Resource {
	capacity := clusterstateapi.EmptyResource()
	nodes, _ := qjm.clients.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	startTime := time.Now()
	for _, node := range nodes.Items {
		// skip unschedulable nodes
		if node.Spec.Unschedulable {
			continue
		}
		nodeResource := clusterstateapi.NewResource(node.Status.Allocatable)
		capacity.Add(nodeResource)
		var specNodeName = "spec.nodeName"
		labelSelector := fmt.Sprintf("%s=%s", specNodeName, node.Name)
		podList, err := qjm.clients.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: labelSelector})
		// TODO: when no pods are listed, do we send entire node capacity as available
		// this will cause false positive dispatch.
		if err != nil {
			klog.Errorf("[allocatableCapacity] Error listing pods %v", err)
		}
		for _, pod := range podList.Items {
			if _, ok := pod.GetLabels()["appwrappers.mcad.ibm.com"]; !ok && pod.Status.Phase != v1.PodFailed && pod.Status.Phase != v1.PodSucceeded {
				for _, container := range pod.Spec.Containers {
					usedResource := clusterstateapi.NewResource(container.Resources.Requests)
					capacity.Sub(usedResource)
				}
			}
		}
	}
	klog.Info("[allocatableCapacity] The avaible capacity to dispatch appwrapper is %v and time took to calculate is %v", capacity, time.Now().Sub(startTime))
	return capacity
}

// NewJobController create new AppWrapper Controller
func NewJobController(config *rest.Config, serverOption *options.ServerOption) *XController {
	cc := &XController{
		config:          config,
		serverOption:    serverOption,
		clients:         kubernetes.NewForConfigOrDie(config),
		arbclients:      clientset.NewForConfigOrDie(config),
		eventQueue:      cache.NewFIFO(GetQueueJobKey),
		agentEventQueue: cache.NewFIFO(GetQueueJobKey),
		// initQueue:       cache.NewFIFO(GetQueueJobKey),
		// updateQueue: cache.NewFIFO(GetQueueJobKey),
		qjqueue: NewSchedulingQueue(),
		// cache is turned-off, issue: https://github.com/project-codeflare/multi-cluster-app-dispatcher/issues/588
		// cache:        clusterstatecache.New(config),
		schedulingAW: nil,
	}
	// TODO: work on enabling metrics adapter for correct MCAD mode
	// metrics adapter is implemented through dynamic client which looks at all the
	// resources installed in the cluster to construct cache. May be this is need in
	// multi-cluster mode, so for now it is turned-off: https://github.com/project-codeflare/multi-cluster-app-dispatcher/issues/585
	// cc.metricsAdapter = adapter.New(serverOption, config, cc.cache)

	cc.genericresources = genericresource.NewAppWrapperGenericResource(config)

	// cc.qjobResControls = map[arbv1.ResourceType]queuejobresources.Interface{}
	// RegisterAllQueueJobResourceTypes(&cc.qjobRegisteredResources)

	// initialize pod sub-resource control
	// resControlPod, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePod, config)
	// if err != nil {
	// 	klog.Errorf("fail to create queuejob resource control")
	// 	return nil
	// }
	// if !found {
	// 	klog.Errorf("queuejob resource type Pod not found")
	// 	return nil
	// }
	// cc.qjobResControls[arbv1.ResourceTypePod] = resControlPod

	appWrapperClient, err := clientset.NewForConfig(cc.config)
	if err != nil {
		klog.Fatalf("Could not instantiate k8s client, err=%v", err)
	}
	cc.appwrapperInformer = informerFactory.NewSharedInformerFactory(appWrapperClient, 0).Workload().V1beta1().AppWrappers()
	cc.appwrapperInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *arbv1.AppWrapper:
					klog.V(10).Infof("[Informer] Filter Name=%s Version=%s Local=%t FilterIgnore=%t Sender=%s &qj=%p qj=%+v", t.Name, t.ResourceVersion, t.Status.Local, t.Status.FilterIgnore, t.Status.Sender, t, t)
					// todo: This is a current workaround for duplicate message bug.
					// if t.Status.Local == true { // ignore duplicate message from cache
					//	return false
					// }
					// t.Status.Local = true // another copy of this will be recognized as duplicate
					return true
					//					return !t.Status.FilterIgnore  // ignore update messages
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    cc.addQueueJob,
				UpdateFunc: cc.updateQueueJob,
				DeleteFunc: cc.deleteQueueJob,
			},
		})
	cc.appWrapperLister = cc.appwrapperInformer.Lister()
	cc.appWrapperSynced = cc.appwrapperInformer.Informer().HasSynced

	// Setup Quota
	if serverOption.QuotaEnabled {
		dispatchedAWDemands, dispatchedAWs := cc.getDispatchedAppWrappers()
		cc.quotaManager, err = quotaforestmanager.NewQuotaManager(dispatchedAWDemands, dispatchedAWs, cc.appWrapperLister,
			config, serverOption)
		if err != nil {
			klog.Error("Failed to instantiate quota manager: %#v", err)
			return nil
		}
	} else {
		cc.quotaManager = nil
	}

	// Set dispatcher mode or agent mode
	cc.isDispatcher = serverOption.Dispatcher
	if cc.isDispatcher {
		klog.Infof("[Controller] Dispatcher mode")
	} else {
		klog.Infof("[Controller] Agent mode")
	}

	// create agents and agentMap
	cc.agentMap = map[string]*queuejobdispatch.JobClusterAgent{}
	cc.agentList = []string{}
	for _, agentconfig := range strings.Split(serverOption.AgentConfigs, ",") {
		agentData := strings.Split(agentconfig, ":")
		jobClusterAgent := queuejobdispatch.NewJobClusterAgent(agentconfig, cc.agentEventQueue)
		if jobClusterAgent != nil {
			cc.agentMap["/root/kubernetes/"+agentData[0]] = jobClusterAgent
			cc.agentList = append(cc.agentList, "/root/kubernetes/"+agentData[0])
		}
	}

	if cc.isDispatcher && len(cc.agentMap) == 0 {
		klog.Errorf("Dispatcher mode: no agent information")
		return nil
	}

	// create (empty) dispatchMap
	cc.dispatchMap = map[string]string{}

	return cc
}

func (qjm *XController) waitForPodCountUpdates(searchCond *arbv1.AppWrapperCondition) bool {

	// Continue reserviing resourses if dispatched condition not found
	if searchCond == nil {
		klog.V(10).Infof("[waitForPodCountUpdates] No condition not found.")
		return true
	}

	// Current time
	now := metav1.NowMicro()
	nowPtr := &now

	// Last time AW was dispatched
	dispactedTS := searchCond.LastUpdateMicroTime
	dispactedTSPtr := &dispactedTS

	// Error checking
	if nowPtr.Before(dispactedTSPtr) {
		klog.Errorf("[waitForPodCountUpdates] Current timestamp: %s is before condition latest update timestamp: %s",
			now.String(), dispactedTS.String())
		return true
	}

	// Duration since last time AW was dispatched
	timeSinceDispatched := now.Sub(dispactedTS.Time)

	// Convert timeout default from milli-seconds to microseconds
	timeoutMicroSeconds := qjm.serverOption.DispatchResourceReservationTimeout * 1000

	// Don't reserve resources if timeout is hit
	if timeSinceDispatched.Microseconds() > timeoutMicroSeconds {
		return false
		klog.V(4).Infof("[waitForPodCountUpdates] Dispatch duration time %d microseconds has reached timeout value of %d microseconds",
			timeSinceDispatched.Microseconds(), timeoutMicroSeconds)
	}

	klog.V(10).Infof("[waitForPodCountUpdates] Dispatch duration time %d microseconds has not reached timeout value of %d microseconds",
		timeSinceDispatched.Microseconds(), timeoutMicroSeconds)
	return true
}

// TODO: We can use informer to filter AWs that do not meet the minScheduling spec.
// we still need a thread for dispatch duration but minScheduling spec can definetly be moved to an informer
func (qjm *XController) PreemptQueueJobs() {
	ctx := context.Background()

	qjobs := qjm.GetQueueJobsEligibleForPreemption()
	for _, aw := range qjobs {
		if aw.Status.State == arbv1.AppWrapperStateCompleted || aw.Status.State == arbv1.AppWrapperStateDeleted || aw.Status.State == arbv1.AppWrapperStateFailed {
			continue
		}

		var updateNewJob *arbv1.AppWrapper
		var message string
		newjob, err := qjm.getAppWrapper(aw.Namespace, aw.Name, "[PreemptQueueJobs] get fresh app wrapper")
		if err != nil {
			klog.Warningf("[PreemptQueueJobs] failed in retrieving a fresh copy of the app wrapper '%s/%s', err=%v. Will try to preempt on the next run.", aw.Namespace, aw.Name, err)
			continue
		}
		newjob.Status.CanRun = false
		newjob.Status.FilterIgnore = true // update QueueJobState only
		cleanAppWrapper := false
		// If dispatch deadline is exceeded no matter what the state of AW, kill the job and set status as Failed.
		if (aw.Status.State == arbv1.AppWrapperStateActive) && (aw.Spec.SchedSpec.DispatchDuration.Limit > 0) {
			if aw.Spec.SchedSpec.DispatchDuration.Overrun {
				index := getIndexOfMatchedCondition(aw, arbv1.AppWrapperCondPreemptCandidate, "DispatchDeadlineExceeded")
				if index < 0 {
					message = fmt.Sprintf("Dispatch deadline exceeded. allowed to run for %v seconds", aw.Spec.SchedSpec.DispatchDuration.Limit)
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "DispatchDeadlineExceeded", message)
					newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "DispatchDeadlineExceeded", "")
					newjob.Status.Conditions[index] = *cond.DeepCopy()
				}
				// should the AW state be set in this method??
				newjob.Status.State = arbv1.AppWrapperStateFailed
				newjob.Status.QueueJobState = arbv1.AppWrapperCondFailed
				newjob.Status.Running = 0
				updateNewJob = newjob.DeepCopy()

				err := qjm.updateStatusInEtcdWithRetry(ctx, updateNewJob, "PreemptQueueJobs - CanRun: false -- DispatchDeadlineExceeded")
				if err != nil {
					klog.Warningf("[PreemptQueueJobs] status update  CanRun: false -- DispatchDeadlineExceeded for '%s/%s' failed", aw.Namespace, aw.Name)
					continue
				}
				// cannot use cleanup AW, since it puts AW back in running state
				qjm.qjqueue.AddUnschedulableIfNotPresent(updateNewJob)

				// Move to next AW
				continue
			}
		}
		//MCAD should allow other controller enough time to spawn and initialize pods
		//DISPATCH_RESOURCE_RESERVATION_TIMEOUT config option in MCAD will be used to wait
		var canPreempt bool = false
		//finds outs if we ran out of reservation time by comparing with AW dispatched condition time
		for _, cond := range aw.Status.Conditions {
			if cond.Type == arbv1.AppWrapperCondDispatched {
				canPreempt = qjm.waitForPodCountUpdates(&cond)
			}
		}

		if canPreempt && ((aw.Status.Running + aw.Status.Succeeded) < int32(aw.Spec.SchedSpec.MinAvailable)) && aw.Status.State == arbv1.AppWrapperStateActive {
			index := getIndexOfMatchedCondition(aw, arbv1.AppWrapperCondPreemptCandidate, "MinPodsNotRunning")
			if index < 0 {
				message = fmt.Sprintf("Insufficient number of Running and Completed pods, minimum=%d, running=%d, completed=%d.", aw.Spec.SchedSpec.MinAvailable, aw.Status.Running, aw.Status.Succeeded)
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "MinPodsNotRunning", message)
				newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
			} else {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "MinPodsNotRunning", "")
				newjob.Status.Conditions[index] = *cond.DeepCopy()
			}

			if aw.Spec.SchedSpec.Requeuing.InitialTimeInSeconds == 0 {
				aw.Spec.SchedSpec.Requeuing.InitialTimeInSeconds = aw.Spec.SchedSpec.Requeuing.TimeInSeconds
			}
			if aw.Spec.SchedSpec.Requeuing.GrowthType == "exponential" {
				if newjob.Status.RequeueingTimeInSeconds == 0 {
					newjob.Status.RequeueingTimeInSeconds += aw.Spec.SchedSpec.Requeuing.TimeInSeconds
				} else {
					newjob.Status.RequeueingTimeInSeconds += newjob.Status.RequeueingTimeInSeconds
				}
			} else if aw.Spec.SchedSpec.Requeuing.GrowthType == "linear" {
				newjob.Status.RequeueingTimeInSeconds += aw.Spec.SchedSpec.Requeuing.InitialTimeInSeconds
			}

			if aw.Spec.SchedSpec.Requeuing.MaxTimeInSeconds > 0 {
				if aw.Spec.SchedSpec.Requeuing.MaxTimeInSeconds <= newjob.Status.RequeueingTimeInSeconds {
					newjob.Status.RequeueingTimeInSeconds = aw.Spec.SchedSpec.Requeuing.MaxTimeInSeconds
				}
			}

			if newjob.Spec.SchedSpec.Requeuing.MaxNumRequeuings > 0 && newjob.Spec.SchedSpec.Requeuing.NumRequeuings == newjob.Spec.SchedSpec.Requeuing.MaxNumRequeuings {
				newjob.Status.State = arbv1.AppWrapperStateDeleted
				cleanAppWrapper = true
			} else {
				newjob.Status.NumberOfRequeueings += 1
			}

			updateNewJob = newjob.DeepCopy()
		} else {
			// If pods failed scheduling generate new preempt condition
			message = fmt.Sprintf("Pods failed scheduling failed=%v, running=%v.", len(aw.Status.PendingPodConditions), aw.Status.Running)
			index := getIndexOfMatchedCondition(newjob, arbv1.AppWrapperCondPreemptCandidate, "PodsFailedScheduling")
			// ignore co-scheduler failed scheduling events. This is a temp
			// work-around until co-scheduler version 0.22.X perf issues are resolved.
			if index < 0 {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "PodsFailedScheduling", message)
				newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
			} else {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "PodsFailedScheduling", message)
				newjob.Status.Conditions[index] = *cond.DeepCopy()
			}

			updateNewJob = newjob.DeepCopy()
		}

		err = qjm.updateStatusInEtcdWithRetry(ctx, updateNewJob, "PreemptQueueJobs - CanRun: false -- MinPodsNotRunning")
		if err != nil {
			klog.Warningf("[PreemptQueueJobs] status update for '%s/%s' failed, skipping app wrapper err =%v", aw.Namespace, aw.Name, err)
			continue
		}

		if cleanAppWrapper {
			klog.V(4).Infof("[PreemptQueueJobs] Deleting AppWrapper %s/%s due to maximum number of re-queueing(s) exceeded.", aw.Name, aw.Namespace)
			go qjm.Cleanup(ctx, updateNewJob)
		} else {
			// Only back-off AWs that are in state running and not in state Failed
			if updateNewJob.Status.State != arbv1.AppWrapperStateFailed {
				klog.Infof("[PreemptQueueJobs] Adding preempted AppWrapper %s/%s to back off queue.", aw.Name, aw.Namespace)
				go qjm.backoff(ctx, updateNewJob, "PreemptionTriggered", string(message))
			}
		}
	}
}

func (qjm *XController) preemptAWJobs(ctx context.Context, preemptAWs []*arbv1.AppWrapper) {
	if preemptAWs == nil {
		return
	}

	for _, aw := range preemptAWs {
		apiCacheAWJob, err := qjm.getAppWrapper(aw.Namespace, aw.Name, "[preemptAWJobs] get fresh app wrapper")
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Warningf("[preemptAWJobs] App wrapper '%s/%s' was not found when getting a fresh copy. ", aw.Namespace, aw.Name)
				continue
			}
			klog.Errorf("[preemptAWJobs] Failed to get AppWrapper to from API Cache %s/%s: err = %v",
				aw.Namespace, aw.Name, err)
			continue
		}
		apiCacheAWJob.Status.CanRun = false
		err = qjm.updateStatusInEtcdWithRetry(ctx, apiCacheAWJob, "preemptAWJobs - CanRun: false")
		if err != nil {
			if apierrors.IsNotFound(err) {
				klog.Warningf("[preemptAWJobs] App wrapper '%s/%s' was not found when updating status. ", aw.Namespace, aw.Name)
				continue
			}
			klog.Warningf("[preemptAWJobs] status update for '%s/%s' failed, err=%v", aw.Namespace, aw.Name, err)
		}
	}
}

func (qjm *XController) GetQueueJobsEligibleForPreemption() []*arbv1.AppWrapper {
	qjobs := make([]*arbv1.AppWrapper, 0)

	queueJobs, err := qjm.appWrapperLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Errorf("List of queueJobs %+v", qjobs)
		return qjobs
	}

	if !qjm.isDispatcher { // Agent Mode
		for _, value := range queueJobs {

			// Skip if AW Pending or just entering the system and does not have a state yet.
			if (value.Status.State == arbv1.AppWrapperStateEnqueued) || (value.Status.State == "") {
				continue
			}

			if value.Status.State == arbv1.AppWrapperStateActive && value.Spec.SchedSpec.DispatchDuration.Limit > 0 {
				awDispatchDurationLimit := value.Spec.SchedSpec.DispatchDuration.Limit
				dispatchDuration := value.Status.ControllerFirstDispatchTimestamp.Add(time.Duration(awDispatchDurationLimit) * time.Second)
				currentTime := time.Now()
				dispatchTimeExceeded := !currentTime.Before(dispatchDuration)

				if dispatchTimeExceeded {
					klog.V(8).Infof("Appwrapper Dispatch limit exceeded, currentTime %v, dispatchTimeInSeconds %v", currentTime, dispatchDuration)
					value.Spec.SchedSpec.DispatchDuration.Overrun = true
					qjobs = append(qjobs, value)
					// Got AW which exceeded dispatch runtime limit, move to next AW
					continue
				}
			}
			replicas := value.Spec.SchedSpec.MinAvailable

			if (int(value.Status.Running) + int(value.Status.Succeeded)) < replicas {

				// Find the dispatched condition if there is any
				numConditions := len(value.Status.Conditions)
				var dispatchedCondition arbv1.AppWrapperCondition
				dispatchedConditionExists := false

				for i := numConditions - 1; i > 0; i-- {
					dispatchedCondition = value.Status.Conditions[i]
					if dispatchedCondition.Type != arbv1.AppWrapperCondDispatched {
						continue
					}
					dispatchedConditionExists = true
					break
				}

				// Check for the minimum age and then skip preempt if current time is not beyond minimum age
				// The minimum age is controlled by the requeuing.TimeInSeconds stanza
				// For preemption, the time is compared to the last condition or the dispatched condition in the AppWrapper, whichever happened later
				lastCondition := value.Status.Conditions[numConditions-1]
				var condition arbv1.AppWrapperCondition

				if dispatchedConditionExists && dispatchedCondition.LastTransitionMicroTime.After(lastCondition.LastTransitionMicroTime.Time) {
					condition = dispatchedCondition
				} else {
					condition = lastCondition
				}
				var requeuingTimeInSeconds int
				if value.Status.RequeueingTimeInSeconds > 0 {
					requeuingTimeInSeconds = value.Status.RequeueingTimeInSeconds
				} else if value.Spec.SchedSpec.Requeuing.InitialTimeInSeconds == 0 {
					requeuingTimeInSeconds = value.Spec.SchedSpec.Requeuing.TimeInSeconds
				} else {
					requeuingTimeInSeconds = value.Spec.SchedSpec.Requeuing.InitialTimeInSeconds
				}

				minAge := condition.LastTransitionMicroTime.Add(time.Duration(requeuingTimeInSeconds) * time.Second)
				currentTime := time.Now()

				if currentTime.Before(minAge) {
					continue
				}

				if replicas > 0 {
					klog.V(3).Infof("AppWrapper '%s/%s' is eligible for preemption Running: %d - minAvailable: %d , Succeeded: %d !!!", value.Namespace, value.Name, value.Status.Running, replicas, value.Status.Succeeded)
					qjobs = append(qjobs, value)
				}
			} else {
				// Preempt when schedulingSpec stanza is not set but pods fails scheduling.
				// ignore co-scheduler pods
				if len(value.Status.PendingPodConditions) > 0 {
					klog.V(3).Infof("AppWrapper '%s/%s' is eligible for preemption Running: %d , Succeeded: %d due to failed scheduling !!!", value.Namespace, value.Status.Running, value.Status.Succeeded)
					qjobs = append(qjobs, value)
				}
			}
		}
	}

	return qjobs
}

func (qjm *XController) GetAggregatedResourcesPerGenericItem(cqj *arbv1.AppWrapper) []*clusterstateapi.Resource {
	var retVal []*clusterstateapi.Resource

	// Get all pods and related resources
	for _, genericItem := range cqj.Spec.AggrResources.GenericItems {
		itemsList, _ := genericresource.GetListOfPodResourcesFromOneGenericItem(&genericItem)
		for i := 0; i < len(itemsList); i++ {
			retVal = append(retVal, itemsList[i])
		}
	}

	return retVal
}

// Gets all objects owned by AW from API server, check user supplied status and set whole AW status
func (qjm *XController) getAppWrapperCompletionStatus(caw *arbv1.AppWrapper) arbv1.AppWrapperState {

	// Get all pods and related resources
	countCompletionRequired := 0
	for i, genericItem := range caw.Spec.AggrResources.GenericItems {
		if len(genericItem.CompletionStatus) > 0 {
			objectName := genericItem.GenericTemplate
			var unstruct unstructured.Unstructured
			unstruct.Object = make(map[string]interface{})
			var blob interface{}
			if err := jsons.Unmarshal(objectName.Raw, &blob); err != nil {
				klog.Errorf("[getAppWrapperCompletionStatus] Error unmarshalling, err=%#v", err)
			}
			unstruct.Object = blob.(map[string]interface{}) // set object to the content of the blob after Unmarshalling
			name := ""
			if md, ok := unstruct.Object["metadata"]; ok {
				metadata := md.(map[string]interface{})
				if objectName, ok := metadata["name"]; ok {
					name = objectName.(string)
				}
			}
			if len(name) == 0 {
				klog.Warningf("[getAppWrapperCompletionStatus] object name not present for appwrapper: '%s/%s", caw.Namespace, caw.Name)
			}
			klog.V(4).Infof("[getAppWrapperCompletionStatus] Checking if item %d named %s completed for appwrapper: '%s/%s'...", i+1, name, caw.Namespace, caw.Name)
			status := qjm.genericresources.IsItemCompleted(&genericItem, caw.Namespace, caw.Name, name)
			if !status {
				klog.V(4).Infof("[getAppWrapperCompletionStatus] Item %d named %s not completed for appwrapper: '%s/%s'", i+1, name, caw.Namespace, caw.Name)
				// early termination because a required item is not completed
				return caw.Status.State
			}

			// only consider count completion required for valid items
			countCompletionRequired = countCompletionRequired + 1

		}
	}
	klog.V(4).Infof("[getAppWrapperCompletionStatus] App wrapper '%s/%s' countCompletionRequired %d, podsRunning %d, podsPending %d", caw.Namespace, caw.Name, countCompletionRequired, caw.Status.Running, caw.Status.Pending)

	// Set new status only when completion required flag is present in genericitems array
	if countCompletionRequired > 0 {
		if caw.Status.Running == 0 && caw.Status.Pending == 0 {
			return arbv1.AppWrapperStateCompleted
		}

		if caw.Status.Pending > 0 || caw.Status.Running > 0 {
			return arbv1.AppWrapperStateRunningHoldCompletion
		}
	}
	// return previous condition
	return caw.Status.State
}

func (qjm *XController) GetAggregatedResources(cqj *arbv1.AppWrapper) *clusterstateapi.Resource {
	allocated := clusterstateapi.EmptyResource()

	for _, genericItem := range cqj.Spec.AggrResources.GenericItems {
		qjv, err := genericresource.GetResources(&genericItem)
		if err != nil {
			klog.V(8).Infof("[GetAggregatedResources] Failure aggregating resources for %s/%s, err=%#v, genericItem=%#v",
				cqj.Namespace, cqj.Name, err, genericItem)
		}
		allocated = allocated.Add(qjv)
	}

	return allocated
}

func (qjm *XController) getProposedPreemptions(requestingJob *arbv1.AppWrapper, availableResourcesWithoutPreemption *clusterstateapi.Resource,
	preemptableAWs map[float64][]string, preemptableAWsMap map[string]*arbv1.AppWrapper) []*arbv1.AppWrapper {

	if requestingJob == nil {
		klog.Warning("[getProposedPreemptions] Invalid job to evaluate.  Job is set to nil.")
		return nil
	}

	aggJobReq := qjm.GetAggregatedResources(requestingJob)
	if aggJobReq.LessEqual(availableResourcesWithoutPreemption) {
		klog.V(10).Infof("[getProposedPreemptions] Job fits without preemption.")
		return nil
	}

	if preemptableAWs == nil || len(preemptableAWs) < 1 {
		klog.V(10).Infof("[getProposedPreemptions] No preemptable jobs.")
		return nil
	} else {
		klog.V(10).Infof("[getProposedPreemptions] Processing %v candidate jobs for preemption.", len(preemptableAWs))
	}

	// Sort keys of map
	priorityKeyValues := make([]float64, len(preemptableAWs))
	i := 0
	for key := range preemptableAWs {
		priorityKeyValues[i] = key
		i++
	}
	sort.Float64s(priorityKeyValues)

	// Get list of proposed preemptions
	var proposedPreemptions []*arbv1.AppWrapper
	foundEnoughResources := false
	preemptable := clusterstateapi.EmptyResource()

	for _, priorityKey := range priorityKeyValues {
		if foundEnoughResources {
			break
		}
		appWrapperIds := preemptableAWs[priorityKey]
		for _, awId := range appWrapperIds {
			aggaw := qjm.GetAggregatedResources(preemptableAWsMap[awId])
			preemptable.Add(aggaw)
			klog.V(4).Infof("[getProposedPreemptions] Adding %s to proposed preemption list on order to dispatch: %s.", awId, requestingJob.Name)
			proposedPreemptions = append(proposedPreemptions, preemptableAWsMap[awId])
			if aggJobReq.LessEqual(preemptable) {
				foundEnoughResources = true
				break
			}
		}
	}

	if !foundEnoughResources {
		klog.V(10).Infof("[getProposedPreemptions] Not enought preemptable jobs to dispatch %s.", requestingJob.Name)
	}

	return proposedPreemptions
}

func (qjm *XController) getDispatchedAppWrappers() (map[string]*clusterstateapi.Resource, map[string]*arbv1.AppWrapper) {
	awrRetVal := make(map[string]*clusterstateapi.Resource)
	awsRetVal := make(map[string]*arbv1.AppWrapper)
	// Setup and break down an informer to get a list of appwrappers bofore controllerinitialization completes
	appWrapperClient, err := clientset.NewForConfig(qjm.config)
	if err != nil {
		klog.Errorf("[getDispatchedAppWrappers] Failure creating client for initialization informer err=%#v", err)
		return awrRetVal, awsRetVal
	}
	queueJobInformer := informerFactory.NewSharedInformerFactory(appWrapperClient, 0).Workload().V1beta1().AppWrappers()
	queueJobInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *arbv1.AppWrapper:
					klog.V(10).Infof("[getDispatchedAppWrappers] Filtered name=%s/%s",
						t.Namespace, t.Name)
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjm.addQueueJob,
				UpdateFunc: qjm.updateQueueJob,
				DeleteFunc: qjm.deleteQueueJob,
			},
		})
	queueJobLister := queueJobInformer.Lister()
	queueJobSynced := queueJobInformer.Informer().HasSynced

	stopCh := make(chan struct{})
	defer close(stopCh)

	go queueJobInformer.Informer().Run(stopCh)

	cache.WaitForCacheSync(stopCh, queueJobSynced)

	appwrappers, err := queueJobLister.AppWrappers("").List(labels.Everything())

	if err != nil {
		klog.Errorf("[getDispatchedAppWrappers] List of AppWrappers err=%+v", err)
		return awrRetVal, awsRetVal
	}

	for _, aw := range appwrappers {
		// Get dispatched jobs
		if aw.Status.CanRun {
			id := qmutils.CreateId(aw.Namespace, aw.Name)
			awrRetVal[id] = qjm.GetAggregatedResources(aw)
			awsRetVal[id] = aw
		}
	}
	klog.V(10).Infof("[getDispatchedAppWrappers] List of runnable AppWrappers dispatched or to be dispatched: %+v",
		awrRetVal)
	return awrRetVal, awsRetVal
}

func (qjm *XController) addTotalSnapshotResourcesConsumedByAw(totalgpu int32, totalcpu int32, totalmemory int32) *clusterstateapi.Resource {
	totalResource := clusterstateapi.EmptyResource()
	totalResource.GPU = int64(totalgpu)
	totalResource.MilliCPU = float64(totalcpu)
	totalResource.Memory = float64(totalmemory)

	return totalResource

}

func (qjm *XController) getAggregatedAvailableResourcesPriority(unallocatedClusterResources *clusterstateapi.
	Resource, targetpr float64, requestingJob *arbv1.AppWrapper, agentId string) (*clusterstateapi.Resource, []*arbv1.AppWrapper) {
	// get available free resources in the cluster.
	r := unallocatedClusterResources.Clone()
	// Track preemption resources
	preemptable := clusterstateapi.EmptyResource()
	preemptableAWs := make(map[float64][]string)
	preemptableAWsMap := make(map[string]*arbv1.AppWrapper)
	// Resources that can fit but have not dispatched.
	pending := clusterstateapi.EmptyResource()
	klog.V(3).Infof("[getAggAvaiResPri] Idle cluster resources %+v", r)

	queueJobs, err := qjm.appWrapperLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Errorf("[getAggAvaiResPri] Unable to obtain the list of queueJobs %+v", err)
		return r, nil
	}
	// for all AWs that have canRun status are true
	// in non-preemption mode, we reserve resources for AWs
	// reserving is done by subtracting total AW resources from pods owned by AW that are running or completed.
	// AW can be running but items owned by it can be completed or there might be new set of pods yet to be spawned
	for _, value := range queueJobs {
		klog.V(10).Infof("[getAggAvaiResPri] %s: Evaluating job: %s to calculate aggregated resources.", time.Now().String(), value.Name)
		if value.Name == requestingJob.Name {
			klog.V(11).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since it is the job being processed.", time.Now().String(), value.Name)
			continue
			// } else if !value.Status.CanRun {
			// 	// canRun is false when AW completes or it is preempted
			// 	// when preempted AW is cleanedup and resources will be released by preempt thread
			// 	// when AW is completed cluster state will reflect available resources
			// 	// in both cases we do not account for resources.
			// 	klog.V(6).Infof("[getAggAvaiResPri] %s: AW %s cannot run, so not accounting resoources", time.Now().String(), value.Name)
			// 	continue
		} else if value.Status.SystemPriority < targetpr {
			// Dispatcher Mode: Ensure this job is part of the target cluster
			if qjm.isDispatcher {
				// Get the job key
				klog.V(10).Infof("[getAggAvaiResPri] %s: Getting job key for: %s.", time.Now().String(), value.Name)
				queueJobKey, _ := GetQueueJobKey(value)
				klog.V(10).Infof("[getAggAvaiResPri] %s: Getting dispatchid for: %s.", time.Now().String(), queueJobKey)
				dispatchedAgentId := qjm.dispatchMap[queueJobKey]

				// If this is not in the same cluster then skip
				if strings.Compare(dispatchedAgentId, agentId) != 0 {
					klog.V(10).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since it is in cluster %s which is not in the same cluster under evaluation: %s.",
						time.Now().String(), value.Name, dispatchedAgentId, agentId)
					continue
				}

			}

			err := qjm.UpdateQueueJobStatus(value)
			if err != nil {
				klog.Warningf("[getAggAvaiResPri] Error updating pod status counts for AppWrapper job: %s, err=%+v", value.Name, err)
			}

			totalResource := qjm.addTotalSnapshotResourcesConsumedByAw(value.Status.TotalGPU, value.Status.TotalCPU, value.Status.TotalMemory)
			klog.V(10).Infof("[getAggAvaiResPri] total resources consumed by Appwrapper %v when lower priority compared to target are %v", value.Name, totalResource)
			preemptable = preemptable.Add(totalResource)
			klog.V(6).Infof("[getAggAvaiResPri] %s proirity %v is lower target priority %v reclaiming total preemptable resources %v", value.Name, value.Status.SystemPriority, targetpr, totalResource)
			queueJobKey, _ := GetQueueJobKey(value)
			addPreemptableAWs(preemptableAWs, value, queueJobKey, preemptableAWsMap)
			continue
		} else if qjm.isDispatcher {
			// Dispatcher job does not currently track pod states.  This is
			// a workaround until implementation of pod state is complete.
			// Currently calculation for available resources only considers priority.
			klog.V(10).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since priority %f is >= %f of requesting job: %s.", time.Now().String(),
				value.Name, value.Status.SystemPriority, targetpr, requestingJob.Name)
			continue
		} else if value.Status.CanRun {
			qjv := clusterstateapi.EmptyResource()
			for _, genericItem := range value.Spec.AggrResources.GenericItems {
				res, _ := genericresource.GetResources(&genericItem)
				qjv.Add(res)
				klog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in genericItem=%T for job %s which can-run is set to: %v but state is still pending.", qjv, genericItem, value.Name, value.Status.CanRun)
			}

			err := qjm.UpdateQueueJobStatus(value)
			if err != nil {
				klog.Warningf("[getAggAvaiResPri] Error updating pod status counts for AppWrapper job: %s, err=%+v", value.Name, err)
			}

			totalResource := qjm.addTotalSnapshotResourcesConsumedByAw(value.Status.TotalGPU, value.Status.TotalCPU, value.Status.TotalMemory)
			klog.V(6).Infof("[getAggAvaiResPri] total resources consumed by Appwrapper %v when CanRun are %v", value.Name, totalResource)
			delta, err := qjv.NonNegSub(totalResource)
			pending = pending.Add(delta)
			if err != nil {
				klog.Warningf("[getAggAvaiResPri] Subtraction of resources failed, adding entire appwrapper resoources %v, %v", qjv, err)
				pending = pending.Add(qjv)
			}
			klog.V(6).Infof("[getAggAvaiResPri] The value of pending is %v", pending)
			continue
		}
	}

	proposedPremptions := qjm.getProposedPreemptions(requestingJob, r, preemptableAWs, preemptableAWsMap)

	klog.V(6).Infof("[getAggAvaiResPri] Schedulable idle cluster resources: %+v, subtracting dispatched resources: %+v and adding preemptable cluster resources: %+v", r, pending, preemptable)
	r = r.Add(preemptable)
	r, _ = r.NonNegSub(pending)

	klog.V(3).Infof("[getAggAvaiResPri] %+v available resources to schedule", r)
	return r, proposedPremptions
}

func addPreemptableAWs(preemptableAWs map[float64][]string, value *arbv1.AppWrapper, queueJobKey string, preemptableAWsMap map[string]*arbv1.AppWrapper) {
	preemptableAWs[value.Status.SystemPriority] = append(preemptableAWs[value.Status.SystemPriority], queueJobKey)
	preemptableAWsMap[queueJobKey] = value
	klog.V(10).Infof("[getAggAvaiResPri] %s: Added %s to candidate preemptable job with priority %f.", time.Now().String(), value.Name, value.Status.SystemPriority)
}

func (qjm *XController) chooseAgent(ctx context.Context, qj *arbv1.AppWrapper) string {

	qjAggrResources := qjm.GetAggregatedResources(qj)
	klog.V(2).Infof("[chooseAgent] Aggregated Resources of XQJ %s: %v\n", qj.Name, qjAggrResources)

	agentId := qjm.agentList[rand.Int()%len(qjm.agentList)]
	klog.V(2).Infof("[chooseAgent] Agent %s is chosen randomly\n", agentId)
	unallocatedResources := qjm.agentMap[agentId].AggrResources
	priorityindex := qj.Status.SystemPriority
	resources, proposedPreemptions := qjm.getAggregatedAvailableResourcesPriority(unallocatedResources, priorityindex, qj, agentId)

	klog.V(2).Infof("[chooseAgent] Aggr Resources of Agent %s: %v\n", agentId, resources)

	if qjAggrResources.LessEqual(resources) {
		klog.V(2).Infof("[chooseAgent] Agent %s has enough resources\n", agentId)

		// Now evaluate quota
		if qjm.serverOption.QuotaEnabled {
			if qjm.quotaManager != nil {
				if fits, preemptAWs, _ := qjm.quotaManager.Fits(qj, qjAggrResources, nil, proposedPreemptions); fits {
					klog.V(2).Infof("[chooseAgent] AppWrapper %s has enough quota.\n", qj.Name)
					qjm.preemptAWJobs(ctx, preemptAWs)
					return agentId
				} else {
					klog.V(2).Infof("[chooseAgent] AppWrapper %s  does not have enough quota\n", qj.Name)
				}
			} else {
				klog.Errorf("[chooseAgent] Quota evaluation is enable but not initialize.  AppWrapper %s/%s does not have enough quota\n", qj.Name, qj.Namespace)
			}
		} else {
			// Quota is not enabled to return selected agent
			return agentId
		}
	} else {
		klog.V(2).Infof("[chooseAgent] Agent %s does not have enough resources\n", agentId)
	}
	return ""
}

func (qjm *XController) nodeChecks(histograms map[string]*dto.Metric, aw *arbv1.AppWrapper) bool {
	ok := true
	allPods := qjm.GetAggregatedResourcesPerGenericItem(aw)

	// Check only GPUs at this time
	var podsToCheck []*clusterstateapi.Resource

	for _, pod := range allPods {
		if pod.GPU > 0 {
			podsToCheck = append(podsToCheck, pod)
		}
	}

	gpuHistogram := histograms["gpu"]

	if gpuHistogram != nil {
		buckets := gpuHistogram.Histogram.Bucket
		// Go through pods needing checking
		for _, gpuPod := range podsToCheck {

			// Go through each bucket of the histogram to find a valid bucket
			bucketFound := false
			for _, bucket := range buckets {
				ub := bucket.UpperBound
				if ub == nil {
					klog.Errorf("Unable to get upperbound of histogram bucket.")
					continue
				}
				c := bucket.GetCumulativeCount()
				var fGPU float64 = float64(gpuPod.GPU)
				if fGPU < *ub && c > 1 {
					// Found a valid node
					bucketFound = true
					break
				}
			}
			if !bucketFound {
				ok = false
				break
			}
		}
	}

	return ok
}

// Thread to find queue-job(QJ) for next schedule
func (qjm *XController) ScheduleNext(qj *arbv1.AppWrapper) {
	ctx := context.Background()
	var err error = nil
	// TODO: do we really need locking now since we have a single thread processing an AW ?
	qjm.schedulingMutex.Lock()
	qjm.schedulingAW = qj
	qjm.schedulingMutex.Unlock()
	// ensure that current active appwrapper is reset at the end of this function, to prevent
	// the appwrapper from being added in syncjob
	defer qjm.schedulingAWAtomicSet(nil)
	// TODO: Retry value is set to 1, do we really need retries?
	scheduleNextRetrier := retrier.New(retrier.ExponentialBackoff(1, 100*time.Millisecond), &EtcdErrorClassifier{})
	scheduleNextRetrier.SetJitter(0.05)
	// Retry the execution
	err = scheduleNextRetrier.Run(func() error {
		klog.Infof("[ScheduleNext] activeQ.Pop %s *Delay=%.6f seconds RemainingLength=%d &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.Length(), qj,
			qj.ResourceVersion, qj.Status)

		apiCacheAWJob, retryErr := qjm.getAppWrapper(qj.Namespace, qj.Name, "[ScheduleNext] -- get fresh copy after queue pop")
		if retryErr != nil {
			if apierrors.IsNotFound(retryErr) {
				klog.Warningf("[ScheduleNext] app wrapper '%s/%s' not found skiping dispatch", qj.Namespace, qj.Name)
				return nil
			}
			klog.Errorf("[ScheduleNext] Unable to get AW %s from API cache &aw=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, retryErr)
			return retryErr
		}
		// make sure qj has the latest information
		if larger(apiCacheAWJob.ResourceVersion, qj.ResourceVersion) {
			klog.V(10).Infof("[ScheduleNext] '%s/%s' found more recent copy from cache          &qj=%p          qj=%+v", qj.Namespace, qj.Name, qj, qj)
			klog.V(10).Infof("[ScheduleNext] '%s/%s' found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiCacheAWJob.Namespace, apiCacheAWJob.Name, apiCacheAWJob, apiCacheAWJob)
			apiCacheAWJob.DeepCopyInto(qj)
		}
		if qj.Status.CanRun {
			klog.V(4).Infof("[ScheduleNext] AppWrapper '%s/%s' from priority queue is already scheduled. Ignoring request: Status=%+v", qj.Namespace, qj.Name, qj.Status)
			return nil
		}

		// Re-compute SystemPriority for DynamicPriority policy
		if qjm.serverOption.DynamicPriority {
			klog.V(4).Info("[ScheduleNext]  dynamic priority enabled")
			//  Create newHeap to temporarily store qjqueue jobs for updating SystemPriority
			tempQ := newHeap(cache.MetaNamespaceKeyFunc, HigherSystemPriorityQJ)
			qj.Status.SystemPriority = float64(qj.Spec.Priority) + qj.Spec.PrioritySlope*(time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time)).Seconds()
			tempQ.Add(qj)
			for qjm.qjqueue.Length() > 0 {
				qjtemp, _ := qjm.qjqueue.Pop()
				qjtemp.Status.SystemPriority = float64(qjtemp.Spec.Priority) + qjtemp.Spec.PrioritySlope*(time.Now().Sub(qjtemp.Status.ControllerFirstTimestamp.Time)).Seconds()
				tempQ.Add(qjtemp)
			}
			// move AppWrappers back to activeQ and sort based on SystemPriority
			for tempQ.data.Len() > 0 {
				qjtemp, _ := tempQ.Pop()
				qjm.qjqueue.AddIfNotPresent(qjtemp.(*arbv1.AppWrapper))
			}
			// Print qjqueue.ativeQ for debugging
			if klog.V(4).Enabled() {
				pq := qjm.qjqueue.(*PriorityQueue)
				if qjm.qjqueue.Length() > 0 {
					for key, element := range pq.activeQ.data.items {
						qjtemp := element.obj
						klog.V(4).Infof("[ScheduleNext] AfterCalc: qjqLength=%d Key=%s index=%d Priority=%.1f SystemPriority=%.1f QueueJobState=%s",
							qjm.qjqueue.Length(), key, element.index, float64(qjtemp.Spec.Priority), qjtemp.Status.SystemPriority, qjtemp.Status.QueueJobState)
					}
				}
			}

			// Retrieve HeadOfLine after priority update
			qj, retryErr = qjm.qjqueue.Pop()
			if retryErr != nil {
				klog.V(3).Infof("[ScheduleNext] Cannot pop QueueJob from qjqueue! err=%#v", retryErr)
				return err
			}
			klog.V(3).Infof("[ScheduleNext] activeQ.Pop_afterPriorityUpdate %s *Delay=%.6f seconds RemainingLength=%d &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.Length(), qj, qj.ResourceVersion, qj.Status)
			apiCacheAWJob, retryErr := qjm.getAppWrapper(qj.Namespace, qj.Name, "[ScheduleNext] -- after dynamic priority pop")
			if retryErr != nil {
				if apierrors.IsNotFound(retryErr) {
					return nil
				}
				klog.Errorf("[ScheduleNext] failed to get a fresh copy of the app wrapper '%s/%s', err=%#v", qj.Namespace, qj.Name, retryErr)
				return err
			}
			if apiCacheAWJob.Status.CanRun {
				klog.Infof("[ScheduleNext] AppWrapper job: %s from API is already scheduled. Ignoring request: Status=%+v", qj.Name, qj.Status)
				return nil
			}
			apiCacheAWJob.DeepCopyInto(qj)
			qjm.schedulingAWAtomicSet(qj)
		}

		qj.Status.QueueJobState = arbv1.AppWrapperCondHeadOfLine
		qjm.addOrUpdateCondition(qj, arbv1.AppWrapperCondHeadOfLine, v1.ConditionTrue, "FrontOfQueue.", "")

		qj.Status.FilterIgnore = true // update QueueJobState only
		retryErr = qjm.updateStatusInEtcd(ctx, qj, "ScheduleNext - setHOL")
		if retryErr != nil {
			if apierrors.IsConflict(retryErr) {
				klog.Warningf("[ScheduleNext] Conflict error detected when updating status in etcd for app wrapper '%s/%s, status = %+v this may be due to appwrapper deletion.", qj.Namespace, qj.Name, qj.Status)
				return nil
			} else {
				klog.Errorf("[ScheduleNext] Failed to updated status in etcd for app wrapper '%s/%s', status = %+v, err=%v", qj.Namespace, qj.Name, qj.Status, retryErr)
			}
			return retryErr
		}
		qjm.qjqueue.AddUnschedulableIfNotPresent(qj) // working on qj, avoid other threads putting it back to activeQ

		klog.V(4).Infof("[ScheduleNext] after Pop qjqLength=%d qj %s Version=%s activeQ=%t Unsched=%t Status=%v", qjm.qjqueue.Length(), qj.Name, qj.ResourceVersion, qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj.Status)
		if qjm.isDispatcher {
			klog.Infof("[ScheduleNext] [Dispatcher Mode] Attempting to dispatch next appwrapper: '%s/%s Status=%v", qj.Namespace, qj.Name, qj.Status)
		} else {
			klog.Infof("[ScheduleNext] [Agent Mode] Attempting to dispatch next appwrapper: '%s/%s' Status=%v", qj.Namespace, qj.Name, qj.Status)
		}

		dispatchFailedReason := "AppWrapperNotRunnable."
		dispatchFailedMessage := ""
		if qjm.isDispatcher { // Dispatcher Mode
			agentId := qjm.chooseAgent(ctx, qj)
			if agentId != "" { // A proper agent is found.
				// Update states (CanRun=True) of XQJ in API Server
				// Add XQJ -> Agent Map
				apiCacheAWJob, retryErr := qjm.getAppWrapper(qj.Namespace, qj.Name, "[ScheduleNext] [Dispatcher Mode] get appwrapper")
				if retryErr != nil {
					if apierrors.IsNotFound(retryErr) {
						klog.Warningf("[ScheduleNext] app wrapper '%s/%s' not found skiping dispatch", qj.Namespace, qj.Name)
						return nil
					}
					klog.Errorf("[ScheduleNext] [Dispatcher Mode] failed to retrieve the app wrapper '%s/%s', err=%#v", qj.Namespace, qj.Name, err)
					return err
				}
				// make sure qj has the latest information
				if larger(apiCacheAWJob.ResourceVersion, qj.ResourceVersion) {
					klog.V(10).Infof("[ScheduleNext] [Dispatcher Mode] App wrapper '%s/%s' found more recent copy from cache          &qj=%p          qj=%+v", qj.Namespace, qj.Name, qj, qj)
					klog.V(10).Infof("[ScheduleNext] [Dispatcher Mode] App wrapper '%s/%s' found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiCacheAWJob.Namespace, apiCacheAWJob.Name, apiCacheAWJob, apiCacheAWJob)
					apiCacheAWJob.DeepCopyInto(qj)
				}
				qj.Status.CanRun = true
				queueJobKey, _ := GetQueueJobKey(qj)
				qjm.dispatchMap[queueJobKey] = agentId
				klog.V(10).Infof("[ScheduleNext] [Dispatcher Mode] %s, %s: ScheduleNextBeforeEtcd", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
				retryErr = qjm.updateStatusInEtcd(ctx, qj, "[ScheduleNext] [Dispatcher Mode] - setCanRun")
				if retryErr != nil {
					if apierrors.IsConflict(err) {
						klog.Warningf("[ScheduleNext] [Dispatcher Mode] Conflict error detected when updating status in etcd for app wrapper '%s/%s, status = %+v. Retrying update.", qj.Namespace, qj.Name, qj.Status)
					} else {
						klog.Errorf("[ScheduleNext] [Dispatcher Mode] Failed to updated status in etcd for app wrapper '%s/%s', status = %+v, err=%v", qj.Namespace, qj.Name, qj.Status, err)
					}
					return retryErr
				}
				klog.V(10).Infof("[ScheduleNext] [Dispatcher Mode] %s, %s: ScheduleNextAfterEtcd", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
				return nil
			} else {
				dispatchFailedMessage = "Cannot find an cluster with enough resources to dispatch AppWrapper."
				klog.V(2).Infof("[ScheduleNex] [Dispatcher Mode] %s %s\n", dispatchFailedReason, dispatchFailedMessage)
				go qjm.backoff(ctx, qj, dispatchFailedReason, dispatchFailedMessage)
			}
		} else { // Agent Mode
			aggqj := qjm.GetAggregatedResources(qj)

			// HeadOfLine logic
			HOLStartTime := time.Now()
			forwarded := false
			fowardingLoopCount := 1
			quotaFits := false
			// Try to forward to eventQueue for at most HeadOfLineHoldingTime
			for !forwarded {
				klog.V(4).Infof("[ScheduleNext] [Agent Mode] Forwarding loop iteration: %d", fowardingLoopCount)
				priorityindex := qj.Status.SystemPriority
				// Support for Non-Preemption
				if !qjm.serverOption.Preemption {
					priorityindex = -math.MaxFloat64
				}
				// Disable Preemption under DynamicPriority.  Comment out if allow DynamicPriority and Preemption at the same time.
				if qjm.serverOption.DynamicPriority {
					priorityindex = -math.MaxFloat64
				}
				// cache now is a method inside the controller.
				// The reimplementation should fix issue : https://github.com/project-codeflare/multi-cluster-app-dispatcher/issues/550
				var unallocatedResources = clusterstateapi.EmptyResource()
				unallocatedResources = qjm.allocatableCapacity()
				for unallocatedResources.IsEmpty() {
					unallocatedResources.Add(qjm.allocatableCapacity())
					if !unallocatedResources.IsEmpty() {
						break
					}
				}
				resources, proposedPreemptions := qjm.getAggregatedAvailableResourcesPriority(
					unallocatedResources, priorityindex, qj, "")
				klog.Infof("[ScheduleNext] [Agent Mode] Appwrapper '%s/%s' with resources %v to be scheduled on aggregated idle resources %v", qj.Namespace, qj.Name, aggqj, resources)

				// Jobs dispatched with quota management may be borrowing quota from other tree nodes making those jobs preemptable, regardless of their priority.
				// Cluster resources need to be considered to determine if both quota and resources (after deleting borrowing AppWrappers) are availabe for the new AppWrapper
				// We perform a "quota check" first followed by a "resource check"
				fits := true
				if qjm.serverOption.QuotaEnabled {
					if qjm.quotaManager != nil {
						// Quota tree design:
						// - All AppWrappers without quota submission will consume quota from the 'default' node.
						// - All quota trees in the system should have a 'default' node so AppWrappers without
						//   quota specification can be dispatched
						// - If the AppWrapper doesn't have a quota label, then one is added for every tree with the 'default' value
						// - Depending on how the 'default' node is configured, AppWrappers that don't specify quota could be
						//   preemptable by default (e.g., 'default' node with 'cpu: 0m' and 'memory: 0Mi' quota and 'hardLimit: false'
						//   such node borrows quota from other nodes already in the system)
						allTrees := qjm.quotaManager.GetValidQuotaLabels()
						newLabels := make(map[string]string)
						for key, value := range qj.Labels {
							newLabels[key] = value
						}
						updateLabels := false
						for _, treeName := range allTrees {
							if _, quotaSetForAW := newLabels[treeName]; !quotaSetForAW {
								newLabels[treeName] = "default"
								updateLabels = true
							}
						}
						if updateLabels {
							tempAW, retryErr := qjm.getAppWrapper(qj.Namespace, qj.Name, "[ScheduleNext] [Agent Mode] update labels")
							if retryErr != nil {
								if apierrors.IsNotFound(retryErr) {
									klog.Warningf("[ScheduleNext] [Agent Mode] app wrapper '%s/%s' not found while trying to update labels, skiping dispatch.", qj.Namespace, qj.Name)
									return nil
								}
								return retryErr
							}
							tempAW.SetLabels(newLabels)
							updatedAW, retryErr := qjm.updateEtcd(ctx, tempAW, "ScheduleNext [Agent Mode] - setDefaultQuota")
							if retryErr != nil {
								if apierrors.IsConflict(err) {
									klog.Warningf("[ScheduleNext] [Agent mode] Conflict error detected when updating labels in etcd for app wrapper '%s/%s, status = %+v. Retrying update.", qj.Namespace, qj.Name, qj.Status)
								} else {
									klog.Errorf("[ScheduleNext] [Agent mode] Failed to update labels in etcd for app wrapper '%s/%s', status = %+v, err=%v", qj.Namespace, qj.Name, qj.Status, err)
								}
								return retryErr
							}
							klog.Infof("[ScheduleNext] [Agent Mode] Default quota added to AW '%s/%s'", qj.Namespace, qj.Name)
							updatedAW.DeepCopyInto(qj)
						}

						// Allocate consumer into quota tree and check if there are enough resources to dispatch it
						var msg string
						var preemptAWs []*arbv1.AppWrapper
						quotaFits, preemptAWs, msg = qjm.quotaManager.Fits(qj, aggqj, resources, proposedPreemptions)
						klog.Info("%s %s %s", quotaFits, preemptAWs, msg)

						if quotaFits {
							klog.Infof("[ScheduleNext] [Agent mode] quota evaluation successful for app wrapper '%s/%s' activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v",
								qj.Namespace, qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
							// Set any jobs that are marked for preemption
							qjm.preemptAWJobs(ctx, preemptAWs)
						} else { // Not enough free quota to dispatch appwrapper
							dispatchFailedMessage = "Insufficient quota and/or resources to dispatch AppWrapper."
							dispatchFailedReason = "quota limit exceeded"
							klog.Infof("[ScheduleNext] [Agent Mode] Blocking dispatch for app wrapper '%s/%s' due to quota limits, activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v msg=%s",
								qj.Namespace, qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status, msg)
							// Call update etcd here to retrigger AW execution for failed quota
							// TODO: quota management tests fail if this is converted into go-routine, need to inspect why?
							qjm.backoff(context.Background(), qj, dispatchFailedReason, dispatchFailedMessage)
						}
						fits = quotaFits
					} else {
						// Quota manager not initialized
						dispatchFailedMessage = "Quota evaluation is enabled but not initialized. Insufficient quota to dispatch AppWrapper."
						klog.Errorf("[ScheduleNext] [Agent Mode] Quota evaluation is enabled but not initialized.  AppWrapper '%s/%s' does not have enough quota", qj.Namespace, qj.Name)
						fits = false
					}
				} else {
					klog.V(4).Infof("[ScheduleNext] [Agent Mode] Quota evaluation not enabled for '%s/%s' at %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Namespace,
						qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)

					if aggqj.LessEqual(resources) { // Check if enough resources to dispatch
						fits = true
						klog.Infof("[ScheduleNext] [Agent Mode] available resourse successful check for '%s/%s' at %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v.",
							qj.Name, qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
					} else { // Not enough free resources to dispatch HOL
						fits = false
						dispatchFailedMessage = "Insufficient resources to dispatch AppWrapper."
						klog.Infof("[ScheduleNext] [Agent Mode] Failed to dispatch app wrapper '%s/%s' due to insuficient resources, activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v",
							qj.Namespace, qj.Name, qjm.qjqueue.IfExistActiveQ(qj),
							qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
						// TODO: Remove forwarded logic as a big AW will never be forwarded
						forwarded = true
						// should we call backoff or update etcd?
						go qjm.backoff(ctx, qj, dispatchFailedReason, dispatchFailedMessage)
					}
				}
				forwarded = true
				if fits {
					// aw is ready to go!
					tempAW, retryErr := qjm.getAppWrapper(qj.Namespace, qj.Name, "[ScheduleNext] [Agent Mode]  -- ready to dispatch")
					if retryErr != nil {
						if apierrors.IsNotFound(retryErr) {
							return nil
						}
						klog.Errorf("[ScheduleNext] [Agent Mode] Failed to get fresh copy of the app wrapper '%s/%s' to update status, err = %v", qj.Namespace, qj.Name, err)
						return retryErr
					}
					tempAW.Status.CanRun = true
					tempAW.Status.FilterIgnore = true // update CanRun & Spec.  no need to trigger event
					retryErr = qjm.updateStatusInEtcd(ctx, tempAW, "ScheduleNext - setCanRun")
					if retryErr != nil {
						if qjm.quotaManager != nil && quotaFits {
							// Quota was allocated for this appwrapper, release it.
							qjm.quotaManager.Release(qj)
						}
						if apierrors.IsNotFound(retryErr) {
							klog.Warningf("[ScheduleNext] [Agent Mode] app wrapper '%s/%s' not found after status update, skiping dispatch.", qj.Namespace, qj.Name)
							return nil
						} else if apierrors.IsConflict(retryErr) {
							klog.Warningf("[ScheduleNext] [Agent mode] Conflict error detected when updating status in etcd for app wrapper '%s/%s, status = %+v. Retrying update.", qj.Namespace, qj.Name, qj.Status)
						} else if retryErr != nil {
							klog.Errorf("[ScheduleNext] [Agent mode] Failed to update status in etcd for app wrapper '%s/%s', status = %+v, err=%v", qj.Namespace, qj.Name, qj.Status, err)
						}
						return retryErr
					}
					tempAW.DeepCopyInto(qj)
					forwarded = true
				}

				// TODO: Remove schedulingTimeExpired flag: https://github.com/project-codeflare/multi-cluster-app-dispatcher/issues/586
				schedulingTimeExpired := false
				if forwarded {
					break
				} else if schedulingTimeExpired {
					// stop trying to dispatch after HeadOfLineHoldingTime
					// release quota if allocated
					if qjm.quotaManager != nil && quotaFits {
						// Quota was allocated for this appwrapper, release it.
						qjm.quotaManager.Release(qj)
					}
					break
				} else { // Try to dispatch again after one second
					if qjm.quotaManager != nil && quotaFits {
						// release any quota as the qj will be tried again and the quota might have been allocated.
						qjm.quotaManager.Release(qj)
					}
					time.Sleep(time.Second * 1)
				}
				fowardingLoopCount += 1
			}
			if !forwarded { // start thread to backoff
				klog.Infof("[ScheduleNext] [Agent Mode] backing off app wrapper '%s/%s' after waiting for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v",
					qj.Namespace, qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				if qjm.quotaManager != nil && quotaFits {
					qjm.quotaManager.Release(qj)
				}
				go qjm.backoff(ctx, qj, dispatchFailedReason, dispatchFailedMessage)
			}
		}
		return nil
	})
	if apierrors.IsNotFound(err) {
		klog.Warningf("[ScheduleNext] app wrapper '%s/%s' not found skiping dispatch", qj.Namespace, qj.Name)
		return
	}
	if err != nil {
		klog.Warningf("[ScheduleNext] failed to dispatch the app wrapper '%s/%s', err= %v", qj.Namespace, qj.Name, err)
		klog.Warningf("[ScheduleNext] retrying dispatch")
		qjm.qjqueue.AddIfNotPresent(qj)
	}
}

// Update AppWrappers in etcd
// todo: This is a current workaround for duplicate message bug.
func (cc *XController) updateEtcd(ctx context.Context, currentAppwrapper *arbv1.AppWrapper, caller string) (*arbv1.AppWrapper, error) {
	klog.V(4).Infof("[updateEtcd] trying to update '%s/%s' called by '%s'", currentAppwrapper.Namespace, currentAppwrapper.Name, caller)
	currentAppwrapper.Status.Sender = "before " + caller // set Sender string to indicate code location
	currentAppwrapper.Status.Local = false               // for Informer FilterFunc to pickup
	updatedAppwrapper, err := cc.arbclients.WorkloadV1beta1().AppWrappers(currentAppwrapper.Namespace).Update(ctx, currentAppwrapper, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	if larger(currentAppwrapper.ResourceVersion, updatedAppwrapper.ResourceVersion) {
		klog.Warningf("[updateEtcd] current app wrapper '%s/%s' called by '%s' has version %s", currentAppwrapper.Namespace, currentAppwrapper.Name, caller, currentAppwrapper.ResourceVersion)
		klog.Warningf("[updateEtcd] updated app wrapper '%s/%s' called by '%s' has version %s", updatedAppwrapper.Namespace, updatedAppwrapper.Name, caller, updatedAppwrapper.ResourceVersion)
	}

	klog.V(4).Infof("[updateEtcd] update success '%s/%s' called by '%s'", currentAppwrapper.Namespace, currentAppwrapper.Name, caller)
	return updatedAppwrapper.DeepCopy(), nil
}

func (cc *XController) updateStatusInEtcd(ctx context.Context, currentAppwrapper *arbv1.AppWrapper, caller string) error {
	klog.V(4).Infof("[updateStatusInEtcd] trying to update '%s/%s' called by '%s'", currentAppwrapper.Namespace, currentAppwrapper.Name, caller)
	currentAppwrapper.Status.Sender = "before " + caller // set Sender string to indicate code location
	updatedAppwrapper, err := cc.arbclients.WorkloadV1beta1().AppWrappers(currentAppwrapper.Namespace).UpdateStatus(ctx, currentAppwrapper, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	if larger(currentAppwrapper.ResourceVersion, updatedAppwrapper.ResourceVersion) {
		klog.Warningf("[updateStatusInEtcd] current app wrapper '%s/%s' called by '%s' has version %s", currentAppwrapper.Namespace, currentAppwrapper.Name, caller, currentAppwrapper.ResourceVersion)
		klog.Warningf("[updateStatusInEtcd] updated app wrapper '%s/%s' called by '%s' has version %s", updatedAppwrapper.Namespace, updatedAppwrapper.Name, caller, updatedAppwrapper.ResourceVersion)
	}
	updatedAppwrapper.DeepCopyInto(currentAppwrapper)
	klog.V(4).Infof("[updateStatusInEtcd] update success '%s/%s' called by '%s'", currentAppwrapper.Namespace, currentAppwrapper.Name, caller)
	return nil
}

func (cc *XController) updateStatusInEtcdWithRetry(ctx context.Context, source *arbv1.AppWrapper, caller string) error {
	klog.V(4).Infof("[updateStatusInEtcdWithMergeFunction] trying to update '%s/%s' version '%s' called by '%s'", source.Namespace, source.Name, source.ResourceVersion, caller)
	source.Status.Sender = "before " + caller // set Sender string to indicate code location
	updateStatusRetrierRetrier := retrier.New(retrier.ExponentialBackoff(10, 100*time.Millisecond), &EtcdErrorClassifier{})
	updateStatusRetrierRetrier.SetJitter(0.05)
	updatedAW := source.DeepCopy()
	err := updateStatusRetrierRetrier.RunCtx(ctx, func(localContext context.Context) error {
		var retryErr error
		updatedAW, retryErr = cc.arbclients.WorkloadV1beta1().AppWrappers(updatedAW.Namespace).UpdateStatus(localContext, updatedAW, metav1.UpdateOptions{})
		if retryErr != nil && apierrors.IsConflict(retryErr) {
			dest, retryErr := cc.getAppWrapper(source.Namespace, source.Name, caller)
			if retryErr != nil && !apierrors.IsNotFound(retryErr) {
				klog.Warningf("[updateStatusInEtcdWithMergeFunction] retrying the to update '%s/%s'  version '%s' called by '%s'", source.Namespace, source.Name, source.ResourceVersion, caller)
				source.Status.DeepCopyInto(&dest.Status)
				dest.Status.Sender = "before " + caller // set Sender string to indicate code location
				dest.DeepCopyInto(updatedAW)
			}
			return retryErr
		}
		if retryErr == nil {
			updatedAW.DeepCopyInto(source)
		}
		return retryErr
	})
	if err != nil {
		klog.V(4).Infof("[updateStatusInEtcdWithMergeFunction] update failure '%s/%s' called by '%s'", source.Namespace, source.Name, caller)
		return err
	}
	klog.V(4).Infof("[updateStatusInEtcdWithMergeFunction] update success '%s/%s' version '%s' called by '%s'", source.Namespace, source.Name, source.ResourceVersion, caller)
	return nil
}

func (qjm *XController) addOrUpdateCondition(aw *arbv1.AppWrapper, condType arbv1.AppWrapperConditionType,
	condStatus v1.ConditionStatus, condReason string, condMsg string) {
	var dupConditionExists bool = false
	if aw.Status.Conditions != nil && len(aw.Status.Conditions) > 0 {
		// Find a matching condition based on fields not related to timestamps
		for _, condition := range aw.Status.Conditions {
			if condition.Type == condType && condition.Status == condStatus &&
				condition.Reason == condReason && condition.Message == condMsg {
				oldLastUpdateMicroTime := condition.LastUpdateMicroTime
				condition.LastUpdateMicroTime = metav1.NowMicro()
				condition.LastTransitionMicroTime = metav1.NowMicro()
				dupConditionExists = true
				klog.V(10).Infof("[addOrUpdateCondition] Updated timestamp of condition for AppWrapper %s/%s from timestamp %v to %+v",
					aw.Name, aw.Name, oldLastUpdateMicroTime, condition)
				break
			}
		}
	}

	// Only add new condition if is is a new one otherwise it is assumed that the condition was updated above.
	if !dupConditionExists {
		cond := GenerateAppWrapperCondition(condType, condStatus, condReason, condMsg)
		aw.Status.Conditions = append(aw.Status.Conditions, cond)
	}
}

func (qjm *XController) backoff(ctx context.Context, q *arbv1.AppWrapper, reason string, message string) {

	etcUpdateRetrier := retrier.New(retrier.ExponentialBackoff(10, 100*time.Millisecond), &EtcdErrorClassifier{})
	err := etcUpdateRetrier.Run(func() error {
		apiCacheAWJob, retryErr := qjm.getAppWrapper(q.Namespace, q.Name, "[backoff] - Rejoining")
		if retryErr != nil {
			return retryErr
		}
		q.Status.DeepCopyInto(&apiCacheAWJob.Status)
		apiCacheAWJob.Status.QueueJobState = arbv1.AppWrapperCondBackoff
		apiCacheAWJob.Status.FilterIgnore = true // update QueueJobState only, no work needed
		// Update condition
		qjm.addOrUpdateCondition(apiCacheAWJob, arbv1.AppWrapperCondBackoff, v1.ConditionTrue, reason, message)
		if retryErr := qjm.updateStatusInEtcd(ctx, apiCacheAWJob, "[backoff] - Rejoining"); retryErr != nil {
			if apierrors.IsConflict(retryErr) {
				klog.Warningf("[backoff] Conflict when upating AW status in etcd '%s/%s'. Retrying.", apiCacheAWJob.Namespace, apiCacheAWJob.Name)
			}
			return retryErr
		}
		return nil
	})
	if err != nil {
		klog.Errorf("[backoff] Failed to update status for %s/%s.  Continuing with possible stale object without updating conditions. err=%s", q.Namespace, q.Name, err)
	}
	qjm.qjqueue.AddUnschedulableIfNotPresent(q)
	klog.V(3).Infof("[backoff] %s move to unschedulableQ before sleep for %d seconds. activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", q.Name,
		qjm.serverOption.BackoffTime, qjm.qjqueue.IfExistActiveQ(q), qjm.qjqueue.IfExistUnschedulableQ(q), q, q.ResourceVersion, q.Status)
	time.Sleep(time.Duration(qjm.serverOption.BackoffTime) * time.Second)
	qjm.qjqueue.MoveToActiveQueueIfExists(q)

	klog.V(3).Infof("[backoff] '%s/%s' activeQ Add after sleep for %d seconds. activeQ=%t Unsched=%t &aw=%p Version=%s Status=%+v", q.Namespace, q.Name,
		qjm.serverOption.BackoffTime, qjm.qjqueue.IfExistActiveQ(q), qjm.qjqueue.IfExistUnschedulableQ(q), q, q.ResourceVersion, q.Status)
}

// Run starts AppWrapper Controller
func (cc *XController) Run(stopCh <-chan struct{}) {
	go cc.appwrapperInformer.Informer().Run(stopCh)

	// go cc.qjobResControls[arbv1.ResourceTypePod].Run(stopCh)

	cache.WaitForCacheSync(stopCh, cc.appWrapperSynced)

	// cache is turned off, issue: https://github.com/project-codeflare/multi-cluster-app-dispatcher/issues/588
	// update snapshot of ClientStateCache every second
	// cc.cache.Run(stopCh)

	// start preempt thread is used to preempt AWs that have partial pods or have reached dispatch duration
	go wait.Until(cc.PreemptQueueJobs, 60*time.Second, stopCh)

	// This thread is used to update AW that has completionstatus set to Complete or RunningHoldCompletion
	go wait.Until(cc.UpdateQueueJobs, 5*time.Second, stopCh)

	if cc.isDispatcher {
		go wait.Until(cc.UpdateAgent, 2*time.Second, stopCh) // In the Agent?
		for _, jobClusterAgent := range cc.agentMap {
			go jobClusterAgent.Run(stopCh)
		}
		go wait.Until(cc.agentEventQueueWorker, time.Second, stopCh) // Update Agent Worker
	}

	go wait.Until(cc.worker, 0, stopCh)
}

func (qjm *XController) UpdateAgent() {
	ctx := context.Background()
	klog.V(3).Infof("[Controller] Update AggrResources for All Agents\n")
	for _, jobClusterAgent := range qjm.agentMap {
		jobClusterAgent.UpdateAggrResources(ctx)
	}
}

// Move AW from Running to Completed or RunningHoldCompletion
// Do not use event queues! Running AWs move to Completed, from which it will never transition to any other state.
// State transition: Running->RunningHoldCompletion->Completed
func (qjm *XController) UpdateQueueJobs() {
	queueJobs, err := qjm.appWrapperLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Errorf("[UpdateQueueJobs] Failed to get a list of active appwrappers, err=%+v", err)
		return
	}
	containsCompletionStatus := false
	for _, newjob := range queueJobs {
		for _, item := range newjob.Spec.AggrResources.GenericItems {
			if len(item.CompletionStatus) > 0 {
				containsCompletionStatus = true
			}
		}
		if (newjob.Status.State == arbv1.AppWrapperStateActive || newjob.Status.State == arbv1.AppWrapperStateRunningHoldCompletion) && containsCompletionStatus {
			err := qjm.UpdateQueueJobStatus(newjob)
			if err != nil {
				klog.Errorf("[UpdateQueueJobs]  Error updating pod status counts for AppWrapper job: %s, err=%+v", newjob.Name, err)
				continue
			}
			klog.V(6).Infof("[UpdateQueueJobs] %s: qjqueue=%t &qj=%p Version=%s Status=%+v", newjob.Name, qjm.qjqueue.IfExist(newjob), newjob, newjob.ResourceVersion, newjob.Status)
			// set appwrapper status to Complete or RunningHoldCompletion
			derivedAwStatus := qjm.getAppWrapperCompletionStatus(newjob)

			klog.Infof("[UpdateQueueJobs]  Got completion status '%s' for app wrapper '%s/%s' Version=%s Status.CanRun=%t Status.State=%s, pod counts [Pending: %d, Running: %d, Succeded: %d, Failed %d]", derivedAwStatus, newjob.Namespace, newjob.Name, newjob.ResourceVersion,
				newjob.Status.CanRun, newjob.Status.State, newjob.Status.Pending, newjob.Status.Running, newjob.Status.Succeeded, newjob.Status.Failed)

			// Set Appwrapper state to complete if all items in Appwrapper
			// are completed
			if derivedAwStatus == arbv1.AppWrapperStateRunningHoldCompletion {
				newjob.Status.State = derivedAwStatus
				var updateQj *arbv1.AppWrapper
				index := getIndexOfMatchedCondition(newjob, arbv1.AppWrapperCondRunningHoldCompletion, "SomeItemsCompleted")
				if index < 0 {
					newjob.Status.QueueJobState = arbv1.AppWrapperCondRunningHoldCompletion
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunningHoldCompletion, v1.ConditionTrue, "SomeItemsCompleted", "")
					newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
					newjob.Status.FilterIgnore = true // Update AppWrapperCondRunningHoldCompletion
					updateQj = newjob.DeepCopy()
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunningHoldCompletion, v1.ConditionTrue, "SomeItemsCompleted", "")
					newjob.Status.Conditions[index] = *cond.DeepCopy()
					updateQj = newjob.DeepCopy()
				}
				err := qjm.updateStatusInEtcdWithRetry(context.Background(), updateQj, "[UpdateQueueJobs]  setRunningHoldCompletion")
				if err != nil {
					// TODO: implement retry
					klog.Errorf("[UpdateQueueJobs]  Error updating status 'setRunningHoldCompletion' for AppWrapper: '%s/%s',Status=%+v, err=%+v.", newjob.Namespace, newjob.Name, newjob.Status, err)
				}
			}
			// Set appwrapper status to complete
			if derivedAwStatus == arbv1.AppWrapperStateCompleted {
				newjob.Status.State = derivedAwStatus
				newjob.Status.CanRun = false
				var updateQj *arbv1.AppWrapper
				index := getIndexOfMatchedCondition(newjob, arbv1.AppWrapperCondCompleted, "PodsCompleted")
				if index < 0 {
					newjob.Status.QueueJobState = arbv1.AppWrapperCondCompleted
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondCompleted, v1.ConditionTrue, "PodsCompleted", "")
					newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
					newjob.Status.FilterIgnore = true // Update AppWrapperCondCompleted
					updateQj = newjob.DeepCopy()
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondCompleted, v1.ConditionTrue, "PodsCompleted", "")
					newjob.Status.Conditions[index] = *cond.DeepCopy()
					updateQj = newjob.DeepCopy()
				}
				err := qjm.updateStatusInEtcdWithRetry(context.Background(), updateQj, "[UpdateQueueJobs] setCompleted")
				if err != nil {
					if qjm.quotaManager != nil {
						qjm.quotaManager.Release(updateQj)
					}
					// TODO: Implement retry
					klog.Errorf("[UpdateQueueJobs]  Error updating status 'setCompleted' AppWrapper: '%s/%s',Status=%+v, err=%+v.", newjob.Namespace, newjob.Name, newjob.Status, err)
				}
				if qjm.quotaManager != nil {
					qjm.quotaManager.Release(updateQj)
				}
				// Delete AW from both queue's
				qjm.eventQueue.Delete(updateQj)
				qjm.qjqueue.Delete(updateQj)
			}
			klog.Infof("[UpdateQueueJobs]  Done getting completion status for app wrapper '%s/%s' Version=%s Status.CanRun=%t Status.State=%s, pod counts [Pending: %d, Running: %d, Succeded: %d, Failed %d]", newjob.Namespace, newjob.Name, newjob.ResourceVersion,
				newjob.Status.CanRun, newjob.Status.State, newjob.Status.Pending, newjob.Status.Running, newjob.Status.Succeeded, newjob.Status.Failed)
		}
	}
}

func (cc *XController) addQueueJob(obj interface{}) {
	firstTime := metav1.NowMicro()
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("[Informer-addQJ] object is not AppWrapper. object=%+v", obj)
		return
	}
	klog.V(6).Infof("[Informer-addQJ] %s &qj=%p  qj=%+v", qj.Name, qj, qj)
	if qj.Status.QueueJobState == "" {
		qj.Status.ControllerFirstTimestamp = firstTime
		qj.Status.SystemPriority = float64(qj.Spec.Priority)
		qj.Status.QueueJobState = arbv1.AppWrapperCondInit
		qj.Status.Conditions = []arbv1.AppWrapperCondition{
			{
				Type:                    arbv1.AppWrapperCondInit,
				Status:                  v1.ConditionTrue,
				LastUpdateMicroTime:     metav1.NowMicro(),
				LastTransitionMicroTime: metav1.NowMicro(),
			},
		}
	} else {
		klog.Warningf("[Informer-addQJ] Received and add by the informer for AppWrapper job %s which already has been seen and initialized current state %s with timestamp: %s, elapsed time of %.6f",
			qj.Name, qj.Status.State, qj.Status.ControllerFirstTimestamp, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds())
	}

	klog.V(6).Infof("[Informer-addQJ] %s Delay=%.6f seconds CreationTimestamp=%s ControllerFirstTimestamp=%s",
		qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.CreationTimestamp, qj.Status.ControllerFirstTimestamp)

	klog.V(6).Infof("[Informer-addQJ] enqueue %s &qj=%p Version=%s Status=%+v", qj.Name, qj, qj.ResourceVersion, qj.Status)
	cc.enqueue(qj)
}

func (cc *XController) updateQueueJob(oldObj, newObj interface{}) {
	newQJ, ok := newObj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("[Informer-updateQJ] new object is not AppWrapper. object=%+v", newObj)
		return
	}
	oldQJ, ok := oldObj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("[Informer-updateQJ] old object is not AppWrapper.  enqueue(newQJ).  oldObj=%+v", oldObj)
		return
	}
	// AppWrappers may come out of order.  Ignore old ones.
	if (oldQJ.Namespace == newQJ.Namespace) && (oldQJ.Name == newQJ.Name) && (larger(oldQJ.ResourceVersion, newQJ.ResourceVersion)) {
		klog.V(6).Infof("[Informer-updateQJ] '%s/%s' ignored OutOfOrder arrival &oldQJ=%p oldQJ=%+v", oldQJ.Namespace, oldQJ.Name, oldQJ, oldQJ)
		klog.V(6).Infof("[Informer-updateQJ] '%s/%s' ignored OutOfOrder arrival &newQJ=%p newQJ=%+v", newQJ.Namespace, newQJ.Name, newQJ, newQJ)
		return
	}

	if equality.Semantic.DeepEqual(newQJ.Status, oldQJ.Status) {
		klog.V(6).Infof("[Informer-updateQJ] No change to status field of AppWrapper: '%s/%s', oldAW=%+v, newAW=%+v.", newQJ.Namespace, newQJ.Name, oldQJ.Status, newQJ.Status)
	}

	klog.V(6).Infof("[Informer-updateQJ] '%s/%s' *Delay=%.6f seconds normal enqueue Version=%s Status=%v", newQJ.Namespace, newQJ.Name, time.Now().Sub(newQJ.Status.ControllerFirstTimestamp.Time).Seconds(), newQJ.ResourceVersion, newQJ.Status)
	// cc.eventQueue.Delete(oldObj)
	cc.enqueue(newQJ)
}

// a, b arbitrary length numerical string.  returns true if a larger than b
func larger(a, b string) bool {
	if len(a) > len(b) {
		return true
	} // Longer string is larger
	if len(a) < len(b) {
		return false
	} // Longer string is larger
	return a > b // Equal length, lexicographic order
}

// When an AW is deleted, do not add such AWs to the event queue.
// AW can never be brought back when it is deleted by an external client, so do not bother adding it to event queue.
// There will be a scenario, where an AW is in middle of dispatch cycle and it may be deleted. At that point when such an
// AW is added to etcd a conflict error will be raised. This will cause the current AW to be skipped.
// If there are large number of delete's may be informer misses few delete events for this simplification.
// For 1K AW all of them are deleted from the system, and the next batch of re-submitted AW begins processing in less than 2 mins
func (cc *XController) deleteQueueJob(obj interface{}) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("[Informer-deleteQJ] obj is not AppWrapper. obj=%+v", obj)
		return
	}
	// we delete the job from the queue if it is there, ignoring errors
	if cc.serverOption.QuotaEnabled && cc.quotaManager != nil {
		cc.quotaManager.Release(qj)
	}
	cc.qjqueue.Delete(qj)
	cc.eventQueue.Delete(qj)
}

func (cc *XController) enqueue(obj interface{}) error {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		return fmt.Errorf("[enqueue] obj is not AppWrapper. obj=%+v", obj)
	}

	err := cc.eventQueue.Add(qj) // add to FIFO queue if not in, update object & keep position if already in FIFO queue
	if err != nil {
		klog.Errorf("[enqueue] Fail to enqueue %s to eventQueue, ignore.  *Delay=%.6f seconds &qj=%p Version=%s Status=%+v err=%#v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj, qj.ResourceVersion, qj.Status, err)
	} else {
		klog.V(10).Infof("[enqueue] %s *Delay=%.6f seconds eventQueue.Add_byEnqueue &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj, qj.ResourceVersion, qj.Status)
	}
	return err
}

func (cc *XController) enqueueIfNotPresent(obj interface{}) error {
	aw, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		return fmt.Errorf("[enqueueIfNotPresent] obj is not AppWrapper. obj=%+v", obj)
	}

	err := cc.eventQueue.AddIfNotPresent(aw) // add to FIFO queue if not in, update object & keep position if already in FIFO queue
	return err
}

func (cc *XController) agentEventQueueWorker() {
	ctx := context.Background()
	if _, err := cc.agentEventQueue.Pop(func(obj interface{}) error {
		var queuejob *arbv1.AppWrapper
		switch v := obj.(type) {
		case *arbv1.AppWrapper:
			queuejob = v
		default:
			klog.Errorf("Un-supported type of %v", obj)
			return nil
		}

		if queuejob == nil {
			if acc, err := meta.Accessor(obj); err != nil {
				klog.Warningf("Failed to get AppWrapper for %v/%v", acc.GetNamespace(), acc.GetName())
			}

			return nil
		}
		klog.V(3).Infof("[Controller: Dispatcher Mode] XQJ Status Update from AGENT: Name:%s, Status: %+v\n", queuejob.Name, queuejob.Status)

		// sync AppWrapper
		if err := cc.updateQueueJobStatus(ctx, queuejob); err != nil {
			klog.Errorf("Failed to sync AppWrapper %s, err %#v", queuejob.Name, err)
			// If any error, requeue it.
			return err
		}

		return nil
	}); err != nil {
		klog.Errorf("Fail to pop item from updateQueue, err %#v", err)
		return
	}
}

func (cc *XController) updateQueueJobStatus(ctx context.Context, queueJobFromAgent *arbv1.AppWrapper) error {
	queueJobInEtcd, err := cc.appWrapperLister.AppWrappers(queueJobFromAgent.Namespace).Get(queueJobFromAgent.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if cc.isDispatcher {
				cc.Cleanup(ctx, queueJobFromAgent)
				cc.qjqueue.Delete(queueJobFromAgent)
			}
			return nil
		}
		return err
	}
	if len(queueJobFromAgent.Status.State) == 0 || queueJobInEtcd.Status.State == queueJobFromAgent.Status.State {
		return nil
	}
	new_flag := queueJobFromAgent.Status.State
	queueJobInEtcd.Status.State = new_flag
	_, err = cc.arbclients.WorkloadV1beta1().AppWrappers(queueJobInEtcd.Namespace).Update(ctx, queueJobInEtcd, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (cc *XController) worker() {
	ctx := context.Background()
	defer func() {
		if pErr := recover(); pErr != nil {
			klog.Errorf("[worker] Panic occurred error: %v, stacktrace: %s", pErr, string(debug.Stack()))
		}
	}()
	item, err := cc.eventQueue.Pop(func(obj interface{}) error {
		var queuejob *arbv1.AppWrapper
		switch v := obj.(type) {
		case *arbv1.AppWrapper:
			queuejob = v
		default:
			klog.Errorf("[worker] eventQueue.Pop un-supported type. obj=%+v", obj)
			return nil
		}
		klog.V(10).Infof("[worker] '%s/%s' *Delay=%.6f seconds eventQueue.Pop_begin &newQJ=%p Version=%s Status=%+v", queuejob.Namespace, queuejob.Name, time.Now().Sub(queuejob.Status.ControllerFirstTimestamp.Time).Seconds(), queuejob, queuejob.ResourceVersion, queuejob.Status)

		if queuejob == nil {
			if acc, err := meta.Accessor(obj); err != nil {
				klog.Warningf("[worker] Failed to get AppWrapper for '%s/%s'", acc.GetNamespace(), acc.GetName())
			}

			return nil
		}

		// asmalvan - starts
		// TODO: Should this be part of ScheduleNext() method?
		if queuejob.Status.State == arbv1.AppWrapperStateCompleted {
			return nil
		}

		// First execution of qj to set Status.State = Enqueued
		if !queuejob.Status.CanRun && (queuejob.Status.State != arbv1.AppWrapperStateEnqueued && queuejob.Status.State != arbv1.AppWrapperStateDeleted) {
			// if there are running resources for this job then delete them because the job was put in
			// pending state...

			// If this the first time seeing this AW, no need to delete.
			stateLen := len(queuejob.Status.State)
			if stateLen > 0 {
				klog.V(2).Infof("[worker] Deleting resources for AppWrapper Job '%s/%s' because it was preempted, status.CanRun=%t, status.State=%s", queuejob.Namespace, queuejob.Name, queuejob.Status.CanRun, queuejob.Status.State)
				err00 := cc.Cleanup(ctx, queuejob)
				if err00 != nil {
					klog.Errorf("[worker] Failed to delete resources for AppWrapper Job '%s/%s', err=%v", queuejob.Namespace, queuejob.Name, err00)
					return err00
				}
				klog.V(2).Infof("[worker] Delete resources for AppWrapper Job '%s/%s' due to preemption was sucessfull, status.CanRun=%t, status.State=%s", queuejob.Namespace, queuejob.Name, queuejob.Status.CanRun, queuejob.Status.State)
			}

			queuejob.Status.State = arbv1.AppWrapperStateEnqueued
			klog.V(10).Infof("[worker] before add to activeQ %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", queuejob.Name, cc.qjqueue.IfExistActiveQ(queuejob), cc.qjqueue.IfExistUnschedulableQ(queuejob), queuejob, queuejob.ResourceVersion, queuejob.Status)
			index := getIndexOfMatchedCondition(queuejob, arbv1.AppWrapperCondQueueing, "AwaitingHeadOfLine")
			if index < 0 {
				queuejob.Status.QueueJobState = arbv1.AppWrapperCondQueueing
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondQueueing, v1.ConditionTrue, "AwaitingHeadOfLine", "")
				queuejob.Status.Conditions = append(queuejob.Status.Conditions, cond)
			} else {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondQueueing, v1.ConditionTrue, "AwaitingHeadOfLine", "")
				queuejob.Status.Conditions[index] = *cond.DeepCopy()
			}

			queuejob.Status.FilterIgnore = true // Update Queueing status, add to qjqueue for ScheduleNext
			err := cc.updateStatusInEtcdWithRetry(ctx, queuejob, "worker - setQueueing")
			if err != nil {
				klog.Errorf("[worker] Error updating status 'setQueueing' AppWrapper: '%s/%s',Status=%+v, err=%+v.", queuejob.Namespace, queuejob.Name, queuejob.Status, err)
				return err
			}

			return nil
		}
		// scheduleNext method takes a dispatched AW which has not been evaluated, extract resources requested by AW
		// compares it with available unallocated cluster resources, performs quota check
		// if everything passes then CanRun is set to true and AW is ready for dispatch
		if !queuejob.Status.CanRun && (queuejob.Status.State != arbv1.AppWrapperStateActive) {
			cc.ScheduleNext(queuejob)
			// When an AW passes ScheduleNext gate then we want to progress AW to Running to begin with
			// Sync queuejob will not unwrap an AW to spawn genericItems
			if queuejob.Status.CanRun {

				// errs := make(chan error, 1)
				// go func() {
				// 	errs <- cc.syncQueueJob(ctx, queuejob)
				// }()

				// // later:
				// if err := <-errs; err != nil {
				// 	return err
				// }
				if err := cc.syncQueueJob(ctx, queuejob); err != nil {
					// If any error, requeue it.
					return err
				}
			}
		}
		// asmalvan- ends

		klog.V(10).Infof("[worker] Ending %s Delay=%.6f seconds &newQJ=%p Version=%s Status=%+v", queuejob.Name, time.Now().Sub(queuejob.Status.ControllerFirstTimestamp.Time).Seconds(), queuejob, queuejob.ResourceVersion, queuejob.Status)

		return nil
	})
	if err != nil && !CanIgnoreAPIError(err) && !IsJsonSyntaxError(err) {
		klog.Warningf("[worker] Fail to process item from eventQueue, err %v. Attempting to re-enqueque...", err)
		if err00 := cc.enqueueIfNotPresent(item); err00 != nil {
			klog.Errorf("[worker] Fatal error trying to re-enqueue item, err =%v", err00)
		} else {
			klog.Warning("[worker] Item re-enqueued.")
		}
		return
	}
}

func (cc *XController) syncQueueJob(ctx context.Context, qj *arbv1.AppWrapper) error {
	cacheAWJob, err := cc.getAppWrapper(qj.Namespace, qj.Name, "[syncQueueJob] get fresh appwrapper ")
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("[syncQueueJob] AppWrapper '%s/%s' not found in cache and will be deleted", qj.Namespace, qj.Name)
			// clean up app wrapper resources including quota
			if err := cc.Cleanup(ctx, qj); err != nil {
				klog.Errorf("Failed to delete resources associated with app wrapper: '%s/%s', err %v", qj.Namespace, qj.Name, err)
				// return error so operation can be retried
				return err
			}
			cc.qjqueue.Delete(qj)
			return nil
		}
		return err
	}
	klog.V(10).Infof("[syncQueueJob] Cache AW '%s/%s' &qj=%p Version=%s Status=%+v", qj.Namespace, qj.Name, qj, qj.ResourceVersion, qj.Status)

	// make sure qj has the latest information
	if larger(cacheAWJob.ResourceVersion, qj.ResourceVersion) {
		klog.V(5).Infof("[syncQueueJob] '%s/%s' found more recent copy from cache         &qj=%p         qj=%+v", qj.Namespace, qj.Name, qj, qj)
		klog.V(5).Infof("[syncQueueJob] '%s/%s' found more recent copy from cache &cacheAWJob=%p cacheAWJob=%+v", cacheAWJob.Namespace, cacheAWJob.Name, cacheAWJob, cacheAWJob)
		cacheAWJob.DeepCopyInto(qj)
	}

	// If it is Agent (not a dispatcher), update pod information
	podPhaseChanges := false
	if !cc.isDispatcher { // agent mode
		// Make a copy first to not update cache object and to use for comparing
		awNew := qj.DeepCopy()
		// we call sync to update pods running, pending,...
		if qj.Status.State == arbv1.AppWrapperStateActive {
			err := cc.UpdateQueueJobStatus(awNew)
			if err != nil {
				klog.Errorf("[syncQueueJob] Error updating pod status counts for AppWrapper job: %s, err=%+v", qj.Name, err)
				return err
			}
			klog.Infof("[syncQueueJob] Pod counts updated for app wrapper '%s/%s' Version=%s Status.CanRun=%t Status.State=%s, pod counts [Pending: %d, Running: %d, Succeded: %d, Failed %d]", awNew.Namespace, awNew.Name, awNew.ResourceVersion,
				awNew.Status.CanRun, awNew.Status.State, awNew.Status.Pending, awNew.Status.Running, awNew.Status.Succeeded, awNew.Status.Failed)

			// Update etcd conditions if AppWrapper Job has at least 1 running pod and transitioning from dispatched to running.
			if (awNew.Status.QueueJobState != arbv1.AppWrapperCondRunning) && (awNew.Status.Running > 0) {
				awNew.Status.QueueJobState = arbv1.AppWrapperCondRunning
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunning, v1.ConditionTrue, "PodsRunning", "")
				awNew.Status.Conditions = append(awNew.Status.Conditions, cond)
				awNew.Status.FilterIgnore = true // Update AppWrapperCondRunning
				err := cc.updateStatusInEtcdWithRetry(ctx, awNew, "[syncQueueJob] Update pod counts")
				if err != nil {
					klog.Error("[syncQueueJob] Error updating pod status counts for app wrapper job: '%s/%s', err=%+v.", qj.Namespace, qj.Name, err)
					return err
				}
				return nil
			}

			// For debugging?
			if !reflect.DeepEqual(awNew.Status, qj.Status) {
				podPhaseChanges = true
				// Using DeepCopy before DeepCopyInto as it seems that DeepCopyInto does not alloc a new memory object
				awNewStatus := awNew.Status.DeepCopy()
				awNewStatus.DeepCopyInto(&qj.Status)
				klog.V(4).Infof("[syncQueueJob] AW pod phase change(s) detected '%s/%s' &eventqueueaw=%p eventqueueawVersion=%s eventqueueawStatus=%+v; &newaw=%p newawVersion=%s newawStatus=%+v",
					qj.Namespace, qj.Name, qj, qj.ResourceVersion, qj.Status, awNew, awNew.ResourceVersion, awNew.Status)
			}
		}
	}

	err = cc.manageQueueJob(ctx, qj, podPhaseChanges)
	return err
}

// manageQueueJob is the core method responsible for managing the number of running
// pods according to what is specified in the job.Spec.
// Does NOT modify <activePods>.
func (cc *XController) manageQueueJob(ctx context.Context, qj *arbv1.AppWrapper, podPhaseChanges bool) error {

	if !cc.isDispatcher { // Agent Mode
		// Job is Complete only update pods if needed.
		if qj.Status.State == arbv1.AppWrapperStateCompleted || qj.Status.State == arbv1.AppWrapperStateRunningHoldCompletion {
			if podPhaseChanges {
				// Only update etcd if AW status has changed.  This can happen for periodic
				// updates of pod phase counts done in caller of this function.
				err := cc.updateStatusInEtcdWithRetry(ctx, qj, "manageQueueJob - podPhaseChanges")
				if err != nil {
					klog.Errorf("[manageQueueJob] Error updating status for podPhaseChanges for AppWrapper: '%s/%s',Status=%+v, err=%+v.", qj.Namespace, qj.Name, qj.Status, err)
					return err
				}
			}
			return nil
		}

		// Handle recovery condition
		if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued && !cc.qjqueue.IfExistUnschedulableQ(qj) && !cc.qjqueue.IfExistActiveQ(qj) {
			// One more check to ensure AW is not the current active scheduled object
			if !cc.IsActiveAppWrapper(qj.Name, qj.Namespace) {
				cc.qjqueue.AddIfNotPresent(qj)
				klog.V(6).Infof("[manageQueueJob] Recovered AppWrapper '%s/%s' - added to active queue, Status=%+v",
					qj.Namespace, qj.Name, qj.Status)
				return nil
			}
		}

		// add qj to Etcd for dispatch
		if qj.Status.CanRun && qj.Status.State != arbv1.AppWrapperStateActive &&
			qj.Status.State != arbv1.AppWrapperStateCompleted &&
			qj.Status.State != arbv1.AppWrapperStateRunningHoldCompletion {
			// keep conditions until the appwrapper is re-dispatched
			qj.Status.PendingPodConditions = nil

			qj.Status.State = arbv1.AppWrapperStateActive
			klog.V(4).Infof("[manageQueueJob] App wrapper '%s/%s' BeforeDispatchingToEtcd Version=%s Status=%+v", qj.Namespace, qj.Name, qj.ResourceVersion, qj.Status)
			dispatched := true
			dispatchFailureReason := "ItemCreationFailure."
			dispatchFailureMessage := ""
			if dispatched {
				// Handle generic resources
				for _, ar := range qj.Spec.AggrResources.GenericItems {
					klog.V(10).Infof("[manageQueueJob] before dispatch Generic.SyncQueueJob %s Version=%sStatus.CanRun=%t, Status.State=%s", qj.Name, qj.ResourceVersion, qj.Status.CanRun, qj.Status.State)
					_, err00 := cc.genericresources.SyncQueueJob(qj, &ar)
					if err00 != nil {
						if apierrors.IsInvalid(err00) {
							klog.Warningf("[manageQueueJob] Invalid generic item sent for dispatching by app wrapper='%s/%s' err=%v", qj.Namespace, qj.Name, err00)
						} else {
							klog.Errorf("[manageQueueJob] Error dispatching generic item for app wrapper='%s/%s' type=%v err=%v", qj.Namespace, qj.Name, err00)
						}
						dispatchFailureMessage = fmt.Sprintf("%s/%s creation failure: %+v", qj.Namespace, qj.Name, err00)
						dispatched = false
					}
				}
			}

			if dispatched { // set AppWrapperCondRunning if all resources are successfully dispatched
				qj.Status.QueueJobState = arbv1.AppWrapperCondDispatched
				index := getIndexOfMatchedCondition(qj, arbv1.AppWrapperCondDispatched, "AppWrapperRunnable")
				if index < 0 {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondDispatched, v1.ConditionTrue, "AppWrapperRunnable", "")
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondDispatched, v1.ConditionTrue, "AppWrapperRunnable", "")
					qj.Status.Conditions[index] = *cond.DeepCopy()
				}

				klog.V(4).Infof("[manageQueueJob] App wrapper '%s/%s' after DispatchingToEtcd Version=%s Status=%+v", qj.Namespace, qj.Name, qj.ResourceVersion, qj.Status)

			} else {
				klog.V(4).Infof("[manageQueueJob] App wrapper '%s/%s' failed dispatching Version=%s Status=%+v", qj.Namespace, qj.Name, qj.ResourceVersion, qj.Status)

				qj.Status.State = arbv1.AppWrapperStateFailed
				qj.Status.QueueJobState = arbv1.AppWrapperCondFailed
				qj.Status.CanRun = false
				if !isLastConditionDuplicate(qj, arbv1.AppWrapperCondFailed, v1.ConditionTrue, dispatchFailureReason, dispatchFailureMessage) {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondFailed, v1.ConditionTrue, dispatchFailureReason, dispatchFailureMessage)
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
				}
				// clean up app wrapper resources including quota
				if err00 := cc.Cleanup(ctx, qj); err00 != nil {
					klog.Errorf("Failed to delete resources associated with app wrapper: '%s/%s', err %v", qj.Namespace, qj.Name, err00)
					// return error so operation can be retried
					return err00
				}
				cc.qjqueue.Delete(qj)
			}

			qj.Status.FilterIgnore = true // update State & QueueJobState after dispatch
			err := cc.updateStatusInEtcdWithRetry(ctx, qj, "manageQueueJob - afterEtcdDispatching")
			if err != nil {
				klog.Errorf("[manageQueueJob] Error updating status 'afterEtcdDispatching' for  AppWrapper: '%s/%s',Status=%+v, err=%+v.", qj.Namespace, qj.Name, qj.Status, err)
				return err
			}
			return nil
		} else if qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateActive {
			klog.Infof("[manageQueueJob] Getting completion status for app wrapper '%s/%s' Version=%s Status.CanRun=%t Status.State=%s, pod counts [Pending: %d, Running: %d, Succeded: %d, Failed %d]", qj.Namespace, qj.Name, qj.ResourceVersion,
				qj.Status.CanRun, qj.Status.State, qj.Status.Pending, qj.Status.Running, qj.Status.Succeeded, qj.Status.Failed)

		} else if podPhaseChanges { // Continued bug fix
			// Only update etcd if AW status has changed.  This can happen for periodic
			// updates of pod phase counts done in caller of this function.
			err := cc.updateStatusInEtcdWithRetry(ctx, qj, "manageQueueJob - podPhaseChanges")
			if err != nil {
				klog.Errorf("[manageQueueJob] Error updating status 'podPhaseChanges' AppWrapper: '%s/%s',Status=%+v, err=%+v.", qj.Namespace, qj.Name, qj.Status, err)
				return err
			}
		}
		return nil
	} else { // Dispatcher Mode
		if !qj.Status.CanRun && (qj.Status.State != arbv1.AppWrapperStateEnqueued && qj.Status.State != arbv1.AppWrapperStateDeleted) {
			// if there are running resources for this job then delete them because the job was put in
			// pending state...
			klog.V(3).Infof("[manageQueueJob] [Dispatcher] Deleting AppWrapper resources because it will be preempted! %s", qj.Name)
			err00 := cc.Cleanup(ctx, qj)
			if err00 != nil {
				klog.Errorf("Failed to clean up resources for app wrapper '%s/%s', err =%v", qj.Namespace, qj.Name, err00)
				return err00
			}

			qj.Status.State = arbv1.AppWrapperStateEnqueued
			if cc.qjqueue.IfExistUnschedulableQ(qj) {
				klog.V(10).Infof("[manageQueueJob] [Dispatcher] leaving '%s/%s' to qjqueue.UnschedulableQ activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Namespace, qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			} else {
				klog.V(10).Infof("[manageQueueJob] [Dispatcher] before add to activeQ '%s/%s' activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Namespace, qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				qj.Status.QueueJobState = arbv1.AppWrapperCondQueueing
				qj.Status.FilterIgnore = true // Update Queueing status, add to qjqueue for ScheduleNext
				err := cc.updateStatusInEtcdWithRetry(ctx, qj, "manageQueueJob - setQueueing")
				if err != nil {
					klog.Errorf("[manageQueueJob] Error updating status 'setQueueing' for AppWrapper: '%s/%s',Status=%+v, err=%+v.", qj.Namespace, qj.Name, qj.Status, err)
					return err
				}
			}
			return nil
		}
		if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued {
			cc.qjqueue.AddIfNotPresent(qj)
			return nil
		}
		if qj.Status.CanRun && !qj.Status.IsDispatched {
			if klog.V(10).Enabled() {
				current_time := time.Now()
				klog.V(10).Infof("[manageQueueJob] [Dispatcher]  XQJ '%s/%s' has Overhead Before Dispatching: %s", qj.Namespace, qj.Name, current_time.Sub(qj.CreationTimestamp.Time))
				klog.V(10).Infof("[manageQueueJob] [Dispatcher]  '%s/%s', %s: WorkerBeforeDispatch", qj.Namespace, qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			}

			queuejobKey, _ := GetQueueJobKey(qj)
			if agentId, ok := cc.dispatchMap[queuejobKey]; ok {
				klog.V(10).Infof("[manageQueueJob] [Dispatcher]  Dispatched AppWrapper %s to Agent ID: %s.", qj.Name, agentId)
				cc.agentMap[agentId].CreateJob(ctx, qj)
				qj.Status.IsDispatched = true
			} else {
				klog.Errorf("[manageQueueJob] [Dispatcher]  AppWrapper %s not found in dispatcher mapping.", qj.Name)
			}
			if klog.V(10).Enabled() {
				current_time := time.Now()
				klog.V(10).Infof("[manageQueueJob] [Dispatcher]  XQJ %s has Overhead After Dispatching: %s", qj.Name, current_time.Sub(qj.CreationTimestamp.Time))
				klog.V(10).Infof("[manageQueueJob] [Dispatcher]  %s, %s: WorkerAfterDispatch", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			}
			err := cc.updateStatusInEtcdWithRetry(ctx, qj, "[manageQueueJob] [Dispatcher]  -- set dispatched true")
			if err != nil {
				klog.Errorf("Failed to update status of AppWrapper %s/%s: err=%v", qj.Namespace, qj.Name, err)
				return err
			}
		}
		return nil
	}
}

// Cleanup function
func (cc *XController) Cleanup(ctx context.Context, appwrapper *arbv1.AppWrapper) error {
	klog.V(3).Infof("[Cleanup] begin AppWrapper '%s/%s' Version=%s", appwrapper.Namespace, appwrapper.Name, appwrapper.ResourceVersion)
	var err *multierror.Error
	if !cc.isDispatcher {
		if appwrapper.Spec.AggrResources.GenericItems != nil {
			for _, ar := range appwrapper.Spec.AggrResources.GenericItems {
				genericResourceName, gvk, err00 := cc.genericresources.Cleanup(appwrapper, &ar)
				if err00 != nil && !CanIgnoreAPIError(err00) && !IsJsonSyntaxError(err00) {
					klog.Errorf("[Cleanup] Error deleting generic item %s, from app wrapper='%s/%s' err=%v.",
						genericResourceName, appwrapper.Namespace, appwrapper.Name, err00)
					err = multierror.Append(err, err00)
					continue
				}
				if gvk != nil {
					klog.V(3).Infof("[Cleanup] Deleted generic item '%s', GVK=%s.%s.%s from app wrapper='%s/%s'",
						genericResourceName, gvk.Group, gvk.Version, gvk.Kind, appwrapper.Namespace, appwrapper.Name)
				} else {
					klog.V(3).Infof("[Cleanup] Deleted generic item '%s' from app wrapper='%s/%s'",
						genericResourceName, appwrapper.Namespace, appwrapper.Name)
				}
			}
		}
	} else {
		if appwrapper.Status.IsDispatched {
			queuejobKey, _ := GetQueueJobKey(appwrapper)
			if obj, ok := cc.dispatchMap[queuejobKey]; ok {
				cc.agentMap[obj].DeleteJob(ctx, appwrapper)
			}
			appwrapper.Status.IsDispatched = false
		}
	}

	// Release quota if quota is enabled and quota manager instance exists
	if cc.serverOption.QuotaEnabled && cc.quotaManager != nil {
		cc.quotaManager.Release(appwrapper)
	}
	appwrapper.Status.Pending = 0
	appwrapper.Status.Running = 0
	appwrapper.Status.Succeeded = 0
	appwrapper.Status.Failed = 0
	klog.V(3).Infof("[Cleanup] end AppWrapper '%s/%s' Version=%s", appwrapper.Namespace, appwrapper.Name, appwrapper.ResourceVersion)

	return err.ErrorOrNil()
}
func (cc *XController) getAppWrapper(namespace string, name string, caller string) (*arbv1.AppWrapper, error) {
	klog.V(5).Infof("[getAppWrapper] getting a copy of '%s/%s' when called by '%s'.", namespace, name, caller)

	apiCacheAWJob, err := cc.appWrapperLister.AppWrappers(namespace).Get(name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("[getAppWrapper] getting a copy of '%s/%s' failed, when called  by '%s', err=%v", namespace, name, caller, err)
		}
		return nil, err
	}
	klog.V(5).Infof("[getAppWrapper] get a copy of '%s/%s' suceeded when called by '%s'", namespace, name, caller)
	return apiCacheAWJob.DeepCopy(), nil
}

type EtcdErrorClassifier struct {
}

func (c *EtcdErrorClassifier) Classify(err error) retrier.Action {
	if err == nil {
		return retrier.Succeed
	} else if apierrors.IsConflict(err) {
		return retrier.Retry
	} else {
		return retrier.Fail
	}
}

// IsActiveAppWrapper safely performs the comparison that was done inside the if block
// at line 1977 in the queuejob_controller_ex.go
// The code looked like this:
//
//	if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued &&
//		!cc.qjqueue.IfExistUnschedulableQ(qj) && !cc.qjqueue.IfExistActiveQ(qj) {
//		// One more check to ensure AW is not the current active schedule object
//		if cc.schedulingAW == nil ||
//			(strings.Compare(cc.schedulingAW.Namespace, qj.Namespace) != 0 &&
//				strings.Compare(cc.schedulingAW.Name, qj.Name) != 0) {
//			cc.qjqueue.AddIfNotPresent(qj)
//			klog.V(3).Infof("[manageQueueJob] Recovered AppWrapper %s%s - added to active queue, Status=%+v",
//				qj.Namespace, qj.Name, qj.Status)
//			return nil
//		}
//	}

func (cc *XController) IsActiveAppWrapper(name, namespace string) bool {
	cc.schedulingMutex.RLock()
	defer cc.schedulingMutex.RUnlock()
	if cc.schedulingAW == nil {
		klog.V(6).Info("[IsActiveAppWrapper] No active scheduling app wrapper set")
	} else {
		klog.V(6).Infof("[IsActiveAppWrapper] Active scheduling app wrapper is : '%s/%s'", cc.schedulingAW.Namespace, cc.schedulingAW.Name)
	}
	return cc.schedulingAW != nil &&
		(strings.Compare(cc.schedulingAW.Namespace, namespace) != 0 &&
			strings.Compare(cc.schedulingAW.Name, name) != 0)
}
func (qjm *XController) schedulingAWAtomicSet(qj *arbv1.AppWrapper) {
	qjm.schedulingMutex.Lock()
	qjm.schedulingAW = qj
	qjm.schedulingMutex.Unlock()
}

func IsJsonSyntaxError(err error) bool {
	var tt *jsons.SyntaxError
	if err == nil {
		return false
	} else if err.Error() == "Job resource template item not define as a PodTemplate" {
		return true
	} else if err.Error() == "name is required" {
		return true
	} else if errors.As(err, &tt) {
		return true
	} else {
		return false
	}
}
func CanIgnoreAPIError(err error) bool {
	return err == nil || apierrors.IsNotFound(err) || apierrors.IsInvalid(err)
}
