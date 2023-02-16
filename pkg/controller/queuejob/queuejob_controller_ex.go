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
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota/quotamanager"
	dto "github.com/prometheus/client_model/go"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/metrics/adapter"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota"
	qmutils "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota/quotamanager/util"
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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"
	resconfigmap "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/configmap" // ConfigMap
	resdeployment "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/deployment"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/genericresource"
	resnamespace "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/namespace"                         // NP
	resnetworkpolicy "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/networkpolicy"                 // NetworkPolicy
	respersistentvolume "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/persistentvolume"           // PV
	respersistentvolumeclaim "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/persistentvolumeclaim" // PVC
	respod "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/pod"
	ressecret "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/secret" // Secret
	resservice "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/service"
	resstatefulset "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/statefulset"
	"k8s.io/apimachinery/pkg/labels"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clientset "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned/clients"
	arbinformers "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion"
	informersv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion/v1"
	listersv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejobdispatch"

	jsons "encoding/json"

	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	clusterstatecache "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/cache"
)

const (
	// QueueJobNameLabel label string for queuejob name
	QueueJobNameLabel string = "appwrapper-name"

	// ControllerUIDLabel label string for queuejob controller uid
	ControllerUIDLabel string = "controller-uid"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = arbv1.SchemeGroupVersion.WithKind("AppWrapper")

//XController the AppWrapper Controller type
type XController struct {
	config       *rest.Config
	serverOption *options.ServerOption

	queueJobInformer informersv1.AppWrapperInformer
	// resources registered for the AppWrapper
	qjobRegisteredResources queuejobresources.RegisteredResources
	// controllers for these resources
	qjobResControls map[arbv1.ResourceType]queuejobresources.Interface

	// Captures all available resources in the cluster
	genericresources *genericresource.GenericResources

	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset

	// A store of jobs
	queueJobLister listersv1.AppWrapperLister
	queueJobSynced func() bool

	// QueueJobs that need to be initialized
	// Add labels and selectors to AppWrapper
	initQueue *cache.FIFO

	// QueueJobs that need to sync up after initialization
	updateQueue *cache.FIFO

	// eventQueue that need to sync up
	eventQueue *cache.FIFO

	//QJ queue that needs to be allocated
	qjqueue SchedulingQueue

	// our own local cache, used for computing total amount of resources
	cache clusterstatecache.Cache

	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager

	// is dispatcher or deployer?
	isDispatcher bool

	// Agent map: agentID -> JobClusterAgent
	agentMap  map[string]*queuejobdispatch.JobClusterAgent
	agentList []string

	// Map for AppWrapper -> JobClusterAgent
	dispatchMap map[string]string

	// Metrics API Server
	metricsAdapter *adapter.MetricsAdpater

	// EventQueueforAgent
	agentEventQueue *cache.FIFO

	// Quota Manager
	quotaManager quota.QuotaManagerInterface

	// Active Scheduling AppWrapper
	schedulingAW *arbv1.AppWrapper
}

type JobAndClusterAgent struct {
	queueJobKey      string
	queueJobAgentKey string
}

func NewJobAndClusterAgent(qjKey string, qaKey string) *JobAndClusterAgent {
	return &JobAndClusterAgent{
		queueJobKey:      qjKey,
		queueJobAgentKey: qaKey,
	}
}

//RegisterAllQueueJobResourceTypes - gegisters all resources
func RegisterAllQueueJobResourceTypes(regs *queuejobresources.RegisteredResources) {
	respod.Register(regs)
	resservice.Register(regs)
	resdeployment.Register(regs)
	resstatefulset.Register(regs)
	respersistentvolume.Register(regs)
	respersistentvolumeclaim.Register(regs)
	resnamespace.Register(regs)
	resconfigmap.Register(regs)
	ressecret.Register(regs)
	resnetworkpolicy.Register(regs)
}

func GetQueueJobAgentKey(obj interface{}) (string, error) {
	qa, ok := obj.(*queuejobdispatch.JobClusterAgent)
	if !ok {
		return "", fmt.Errorf("not a AppWrapperAgent")
	}
	return fmt.Sprintf("%s;%s", qa.AgentId, qa.DeploymentName), nil
}

func GetQueueJobKey(obj interface{}) (string, error) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		return "", fmt.Errorf("not a AppWrapper")
	}

	return fmt.Sprintf("%s/%s", qj.Namespace, qj.Name), nil
}

//NewJobController create new AppWrapper Controller
func NewJobController(config *rest.Config, serverOption *options.ServerOption) *XController {
	cc := &XController{
		config:          config,
		serverOption:    serverOption,
		clients:         kubernetes.NewForConfigOrDie(config),
		arbclients:      clientset.NewForConfigOrDie(config),
		eventQueue:      cache.NewFIFO(GetQueueJobKey),
		agentEventQueue: cache.NewFIFO(GetQueueJobKey),
		initQueue:       cache.NewFIFO(GetQueueJobKey),
		updateQueue:     cache.NewFIFO(GetQueueJobKey),
		qjqueue:         NewSchedulingQueue(),
		cache:           clusterstatecache.New(config),
	}
	cc.metricsAdapter = adapter.New(serverOption, config, cc.cache)

	cc.genericresources = genericresource.NewAppWrapperGenericResource(config)

	cc.qjobResControls = map[arbv1.ResourceType]queuejobresources.Interface{}
	RegisterAllQueueJobResourceTypes(&cc.qjobRegisteredResources)

	//initialize pod sub-resource control
	resControlPod, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePod, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type Pod not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypePod] = resControlPod

	// initialize service sub-resource control
	resControlService, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeService, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type Service not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeService] = resControlService

	// initialize PV sub-resource control
	resControlPersistentVolume, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePersistentVolume, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type PersistentVolume not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypePersistentVolume] = resControlPersistentVolume

	resControlPersistentVolumeClaim, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePersistentVolumeClaim, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type PersistentVolumeClaim not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypePersistentVolumeClaim] = resControlPersistentVolumeClaim

	resControlNamespace, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeNamespace, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type Namespace not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeNamespace] = resControlNamespace

	resControlConfigMap, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeConfigMap, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type ConfigMap not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeConfigMap] = resControlConfigMap

	resControlSecret, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeSecret, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type Secret not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeSecret] = resControlSecret

	resControlNetworkPolicy, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeNetworkPolicy, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type NetworkPolicy not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeNetworkPolicy] = resControlNetworkPolicy

	// initialize deployment sub-resource control
	resControlDeployment, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeDeployment, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type Service not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeDeployment] = resControlDeployment

	// initialize SS sub-resource
	resControlSS, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeStatefulSet, config)
	if err != nil {
		klog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		klog.Errorf("queuejob resource type StatefulSet not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeStatefulSet] = resControlSS

	queueJobClient, _, err := clients.NewClient(cc.config)
	if err != nil {
		panic(err)
	}
	cc.queueJobInformer = arbinformers.NewSharedInformerFactory(queueJobClient, 0).AppWrapper().AppWrappers()
	cc.queueJobInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *arbv1.AppWrapper:
					klog.V(10).Infof("[Informer] Filter Name=%s Version=%s Local=%t FilterIgnore=%t Sender=%s &qj=%p qj=%+v", t.Name, t.ResourceVersion, t.Status.Local, t.Status.FilterIgnore, t.Status.Sender, t, t)
					// todo: This is a current workaround for duplicate message bug.
					//if t.Status.Local == true { // ignore duplicate message from cache
					//	return false
					//}
					//t.Status.Local = true // another copy of this will be recognized as duplicate
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
	cc.queueJobLister = cc.queueJobInformer.Lister()
	cc.queueJobSynced = cc.queueJobInformer.Informer().HasSynced

	//create sub-resource reference manager
	cc.refManager = queuejobresources.NewLabelRefManager()

	// Setup Quota
	if serverOption.QuotaEnabled {
		dispatchedAWDemands, dispatchedAWs := cc.getDispatchedAppWrappers()
		cc.quotaManager, _ = quotamanager.NewQuotaManager(dispatchedAWDemands, dispatchedAWs, cc.queueJobLister,
			config, serverOption)
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

	//create agents and agentMap
	cc.agentMap = map[string]*queuejobdispatch.JobClusterAgent{}
	cc.agentList = []string{}
	for _, agentconfig := range strings.Split(serverOption.AgentConfigs, ",") {
		agentData := strings.Split(agentconfig, ":")
		cc.agentMap["/root/kubernetes/"+agentData[0]] = queuejobdispatch.NewJobClusterAgent(agentconfig, cc.agentEventQueue)
		cc.agentList = append(cc.agentList, "/root/kubernetes/"+agentData[0])
	}

	if cc.isDispatcher && len(cc.agentMap) == 0 {
		klog.Errorf("Dispatcher mode: no agent information")
		return nil
	}

	//create (empty) dispatchMap
	cc.dispatchMap = map[string]string{}

	// Initialize current scheuling active AppWrapper
	cc.schedulingAW = nil
	return cc
}

func (qjm *XController) PreemptQueueJobs() {
	qjobs := qjm.GetQueueJobsEligibleForPreemption()
	for _, aw := range qjobs {
		if aw.Status.State == arbv1.AppWrapperStateCompleted || aw.Status.State == arbv1.AppWrapperStateDeleted || aw.Status.State == arbv1.AppWrapperStateFailed {
			continue
		}

		var updateNewJob *arbv1.AppWrapper
		var message string
		newjob, e := qjm.queueJobLister.AppWrappers(aw.Namespace).Get(aw.Name)
		if e != nil {
			continue
		}
		newjob.Status.CanRun = false
		cleanAppWrapper := false

		if (aw.Status.Running + aw.Status.Succeeded) < int32(aw.Spec.SchedSpec.MinAvailable) {
			index := getIndexOfMatchedCondition(aw, arbv1.AppWrapperCondPreemptCandidate, "MinPodsNotRunning")
			if index < 0 {
				message = fmt.Sprintf("Insufficient number of Running and Completed pods, minimum=%d, running=%d, completed=%d.", aw.Spec.SchedSpec.MinAvailable, aw.Status.Running, aw.Status.Succeeded)
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "MinPodsNotRunning", message)
				newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
			} else {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "MinPodsNotRunning", "")
				newjob.Status.Conditions[index] = *cond.DeepCopy()
			}

			if aw.Spec.SchedSpec.Requeuing.GrowthType == "exponential" {
				newjob.Spec.SchedSpec.Requeuing.TimeInSeconds += aw.Spec.SchedSpec.Requeuing.TimeInSeconds
			} else if aw.Spec.SchedSpec.Requeuing.GrowthType == "linear" {
				newjob.Spec.SchedSpec.Requeuing.TimeInSeconds += aw.Spec.SchedSpec.Requeuing.InitialTimeInSeconds
			}

			if aw.Spec.SchedSpec.Requeuing.MaxTimeInSeconds > 0 {
				if aw.Spec.SchedSpec.Requeuing.MaxTimeInSeconds <= newjob.Spec.SchedSpec.Requeuing.TimeInSeconds {
					newjob.Spec.SchedSpec.Requeuing.TimeInSeconds = aw.Spec.SchedSpec.Requeuing.MaxTimeInSeconds
				}
			}

			if newjob.Spec.SchedSpec.Requeuing.MaxNumRequeuings > 0 && newjob.Spec.SchedSpec.Requeuing.NumRequeuings == newjob.Spec.SchedSpec.Requeuing.MaxNumRequeuings {
				newjob.Status.State = arbv1.AppWrapperStateDeleted
				cleanAppWrapper = true
			} else {
				newjob.Spec.SchedSpec.Requeuing.NumRequeuings += 1
			}

			updateNewJob = newjob.DeepCopy()
		} else {
			//If pods failed scheduling generate new preempt condition
			message = fmt.Sprintf("Pods failed scheduling failed=%v, running=%v.", len(aw.Status.PendingPodConditions), aw.Status.Running)
			index := getIndexOfMatchedCondition(newjob, arbv1.AppWrapperCondPreemptCandidate, "PodsFailedScheduling")
			//ignore co-scheduler failed scheduling events. This is a temp
			//work around until co-scheduler version 0.22.X perf issues are resolved.
			if index < 0 {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "PodsFailedScheduling", message)
				newjob.Status.Conditions = append(newjob.Status.Conditions, cond)
			} else {
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "PodsFailedScheduling", message)
				newjob.Status.Conditions[index] = *cond.DeepCopy()
			}

			updateNewJob = newjob.DeepCopy()
		}
		if err := qjm.updateEtcd(updateNewJob, "PreemptQueueJobs - CanRun: false"); err != nil {
			klog.Errorf("Failed to update status of AppWrapper %v/%v: %v", aw.Namespace, aw.Name, err)
		}
		if cleanAppWrapper {
			klog.V(4).Infof("[PreemptQueueJobs] Deleting AppWrapper %s/%s due to maximum number of requeuings exceeded.", aw.Name, aw.Namespace)
			go qjm.Cleanup(aw)
		} else {
			klog.V(4).Infof("[PreemptQueueJobs] Adding preempted AppWrapper %s/%s to backoff queue.", aw.Name, aw.Namespace)
			go qjm.backoff(aw, "PreemptionTriggered", string(message))
		}
	}
}

func (qjm *XController) preemptAWJobs(preemptAWs []*arbv1.AppWrapper) {
	if preemptAWs == nil {
		return
	}

	for _, aw := range preemptAWs {
		apiCacheAWJob, e := qjm.queueJobLister.AppWrappers(aw.Namespace).Get(aw.Name)
		if e != nil {
			klog.Errorf("[preemptQWJobs] Failed to get AppWrapper to from API Cache %v/%v: %v",
				aw.Namespace, aw.Name, e)
			continue
		}
		apiCacheAWJob.Status.CanRun = false
		if err := qjm.updateEtcd(apiCacheAWJob, "preemptAWJobs - CanRun: false"); err != nil {
			klog.Errorf("Failed to update status of AppWrapper %v/%v: %v",
				apiCacheAWJob.Namespace, apiCacheAWJob.Name, err)
		}
	}
}

func (qjm *XController) GetQueueJobsEligibleForPreemption() []*arbv1.AppWrapper {
	qjobs := make([]*arbv1.AppWrapper, 0)

	queueJobs, err := qjm.queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Errorf("List of queueJobs %+v", qjobs)
		return qjobs
	}

	if !qjm.isDispatcher { // Agent Mode
		for _, value := range queueJobs {
			replicas := value.Spec.SchedSpec.MinAvailable

			// Skip if AW Pending or just entering the system and does not have a state yet.
			if (value.Status.State == arbv1.AppWrapperStateEnqueued) || (value.Status.State == "") {
				continue
			}

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
				lastCondition := value.Status.Conditions[numConditions - 1]
				var condition arbv1.AppWrapperCondition

				if dispatchedConditionExists && dispatchedCondition.LastTransitionMicroTime.After(lastCondition.LastTransitionMicroTime.Time) {
					condition = dispatchedCondition
				} else {
					condition = lastCondition
				}

				requeuingTimeInSeconds := value.Spec.SchedSpec.Requeuing.TimeInSeconds
				minAge := condition.LastTransitionMicroTime.Add(time.Duration(requeuingTimeInSeconds) * time.Second)
				currentTime := time.Now()

				if currentTime.Before(minAge) {
					continue
				}

				if value.Spec.SchedSpec.Requeuing.InitialTimeInSeconds == 0 {
					value.Spec.SchedSpec.Requeuing.InitialTimeInSeconds = value.Spec.SchedSpec.Requeuing.TimeInSeconds
				}

				if replicas > 0 {
					klog.V(3).Infof("AppWrapper %s is eligible for preemption Running: %v - minAvailable: %v , Succeeded: %v !!! \n", value.Name, value.Status.Running, replicas, value.Status.Succeeded)
					qjobs = append(qjobs, value)
				}
			} else {
				// Preempt when schedulingSpec stanza is not set but pods fails scheduling.
				// ignore co-scheduler pods
				if len(value.Status.PendingPodConditions) > 0 {
					klog.V(3).Infof("AppWrapper %s is eligible for preemption Running: %v , Succeeded: %v due to failed scheduling !!! \n", value.Name, value.Status.Running, value.Status.Succeeded)
					qjobs = append(qjobs, value)
				}
			}
		}
	}

	return qjobs
}

func GetPodTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.PodTemplateSpec, error) {
	rtScheme := runtime.NewScheme()
	v1.AddToScheme(rtScheme)

	jsonSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, rtScheme, rtScheme)

	podGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("PodTemplate")

	obj, _, err := jsonSerializer.Decode(qjobRes.Template.Raw, &podGVK, nil)
	if err != nil {
		return nil, err
	}

	template, ok := obj.(*v1.PodTemplate)
	if !ok {
		return nil, fmt.Errorf("Resource template not define as a PodTemplate")
	}

	return &template.Template, nil

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

//Gets all objects owned by AW from API server, check user supplied status and set whole AW status
func (qjm *XController) getAppWrapperCompletionStatus(caw *arbv1.AppWrapper) arbv1.AppWrapperState {

	// Get all pods and related resources
	countCompletionRequired := 0
	for _, genericItem := range caw.Spec.AggrResources.GenericItems {
		if len(genericItem.CompletionStatus) > 0 {
			objectName := genericItem.GenericTemplate
			var unstruct unstructured.Unstructured
			unstruct.Object = make(map[string]interface{})
			var blob interface{}
			if err := jsons.Unmarshal(objectName.Raw, &blob); err != nil {
				klog.Errorf("[getAppWrapperCompletionStatus] Error unmarshalling, err=%#v", err)
			}
			unstruct.Object = blob.(map[string]interface{}) //set object to the content of the blob after Unmarshalling
			name := ""
			if md, ok := unstruct.Object["metadata"]; ok {
				metadata := md.(map[string]interface{})
				if objectName, ok := metadata["name"]; ok {
					name = objectName.(string)
				}
			}
			if len(name) == 0 {
				klog.Warningf("[getAppWrapperCompletionStatus] object name not present for appwrapper: %v in namespace: %v", caw.Name, caw.Namespace)
			}
			klog.Infof("[getAppWrapperCompletionStatus] Checking items completed for appwrapper: %v in namespace: %v", caw.Name, caw.Namespace)

			status := qjm.genericresources.IsItemCompleted(&genericItem, caw.Namespace, caw.Name, name)
			if !status {
				//early termination because a required item is not completed
				return caw.Status.State
			}

			//only consider count completion required for valid items
			countCompletionRequired = countCompletionRequired + 1

		}
	}
	klog.V(4).Infof("[getAppWrapperCompletionStatus] countCompletionRequired %v, podsRunning %v, podsPending %v", countCompletionRequired, caw.Status.Running, caw.Status.Pending)

	//Set new status only when completion required flag is present in genericitems array
	if countCompletionRequired > 0 {
		if caw.Status.Running == 0 && caw.Status.Pending == 0 {
			return arbv1.AppWrapperStateCompleted
		}

		if caw.Status.Pending > 0 || caw.Status.Running > 0 {
			return arbv1.AppWrapperStateRunningHoldCompletion
		}
	}
	//return previous condition
	return caw.Status.State
}

func (qjm *XController) GetAggregatedResources(cqj *arbv1.AppWrapper) *clusterstateapi.Resource {
	//todo: deprecate resource controllers
	allocated := clusterstateapi.EmptyResource()
	for _, resctrl := range qjm.qjobResControls {
		qjv := resctrl.GetAggregatedResources(cqj)
		allocated = allocated.Add(qjv)
	}

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

	//Sort keys of map
	priorityKeyValues := make([]float64, len(preemptableAWs))
	i := 0
	for key, _ := range preemptableAWs {
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

	if foundEnoughResources == false {
		klog.V(10).Infof("[getProposedPreemptions] Not enought preemptable jobs to dispatch %s.", requestingJob.Name)
	}

	return proposedPreemptions
}

func (qjm *XController) getDispatchedAppWrappers() (map[string]*clusterstateapi.Resource, map[string]*arbv1.AppWrapper) {
	awrRetVal := make(map[string]*clusterstateapi.Resource)
	awsRetVal := make(map[string]*arbv1.AppWrapper)
	// Setup and break down an informer to get a list of appwrappers bofore controllerinitialization completes
	appwrapperJobClient, _, err := clients.NewClient(qjm.config)
	if err != nil {
		klog.Errorf("[getDispatchedAppWrappers] Failure creating client for initialization informer err=%#v", err)
		return awrRetVal, awsRetVal
	}
	queueJobInformer := arbinformers.NewSharedInformerFactory(appwrapperJobClient, 0).AppWrapper().AppWrappers()
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
		if aw.Status.CanRun == true {
			id := qmutils.CreateId(aw.Namespace, aw.Name)
			awrRetVal[id] = qjm.GetAggregatedResources(aw)
			awsRetVal[id] = aw
		}
	}
	klog.V(10).Infof("[getDispatchedAppWrappers] List of runnable AppWrappers dispatched or to be dispatched: %+v",
		awrRetVal)
	return awrRetVal, awsRetVal
}

func (qjm *XController) getAggregatedAvailableResourcesPriority(unallocatedClusterResources *clusterstateapi.
	Resource, targetpr float64, requestingJob *arbv1.AppWrapper, agentId string) (*clusterstateapi.Resource, []*arbv1.AppWrapper) {
	r := unallocatedClusterResources.Clone()
	// Track preemption resources
	preemptable := clusterstateapi.EmptyResource()
	preemptableAWs := make(map[float64][]string)
	preemptableAWsMap := make(map[string]*arbv1.AppWrapper)
	// Resources that can fit but have not dispatched.
	pending := clusterstateapi.EmptyResource()
	klog.V(3).Infof("[getAggAvaiResPri] Idle cluster resources %+v", r)

	queueJobs, err := qjm.queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Errorf("[getAggAvaiResPri] Unable to obtain the list of queueJobs %+v", err)
		return r, nil
	}

	for _, value := range queueJobs {
		klog.V(10).Infof("[getAggAvaiResPri] %s: Evaluating job: %s to calculate aggregated resources.", time.Now().String(), value.Name)
		if value.Name == requestingJob.Name {
			klog.V(11).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since it is the job being processed.", time.Now().String(), value.Name)
			continue
		} else if !value.Status.CanRun {
			klog.V(11).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since it can not run.", time.Now().String(), value.Name)
			continue
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

				preemptableAWs[value.Status.SystemPriority] = append(preemptableAWs[value.Status.SystemPriority], queueJobKey)
				preemptableAWsMap[queueJobKey] = value
				klog.V(10).Infof("[getAggAvaiResPri] %s: Added %s to candidate preemptable job with priority %f.", time.Now().String(), value.Name, value.Status.SystemPriority)
			}

			for _, resctrl := range qjm.qjobResControls {
				qjv := resctrl.GetAggregatedResources(value)
				preemptable = preemptable.Add(qjv)
			}
			for _, genericItem := range value.Spec.AggrResources.GenericItems {
				qjv, _ := genericresource.GetResources(&genericItem)
				preemptable = preemptable.Add(qjv)
			}

			continue
		} else if qjm.isDispatcher {
			// Dispatcher job does not currently track pod states.  This is
			// a workaround until implementation of pod state is complete.
			// Currently calculation for available resources only considers priority.
			klog.V(10).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since priority %f is >= %f of requesting job: %s.", time.Now().String(),
				value.Name, value.Status.SystemPriority, targetpr, requestingJob.Name)
			continue
		} else if value.Status.State == arbv1.AppWrapperStateEnqueued {
			// Don't count the resources that can run but not yet realized (job orchestration pending or partially running).
			for _, resctrl := range qjm.qjobResControls {
				qjv := resctrl.GetAggregatedResources(value)
				pending = pending.Add(qjv)
				klog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v but state is still pending.", qjv, resctrl, value.Name, value.Status.CanRun)
			}
			for _, genericItem := range value.Spec.AggrResources.GenericItems {
				qjv, _ := genericresource.GetResources(&genericItem)
				pending = pending.Add(qjv)
				klog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v but state is still pending.", qjv, genericItem, value.Name, value.Status.CanRun)
			}
			continue
		} else if value.Status.State == arbv1.AppWrapperStateActive {
			if value.Status.Pending > 0 {
				//Don't count partially running jobs with pods still pending.
				for _, resctrl := range qjm.qjobResControls {
					qjv := resctrl.GetAggregatedResources(value)
					pending = pending.Add(qjv)
					klog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but %v pod(s) are pending.", qjv, resctrl, value.Name, value.Status.CanRun, value.Status.State, value.Status.Pending)
				}
				for _, genericItem := range value.Spec.AggrResources.GenericItems {
					qjv, _ := genericresource.GetResources(&genericItem)
					pending = pending.Add(qjv)
					klog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but %v pod(s) are pending.", qjv, genericItem, value.Name, value.Status.CanRun, value.Status.State, value.Status.Pending)
				}
			} else {
				// TODO: Hack to handle race condition when Running jobs have not yet updated the pod counts (In-Flight AW Jobs)
				// This hack uses the golang struct implied behavior of defining the object without a value.  In this case
				// of using 'int32' novalue and value of 0 are the same.
				if value.Status.Pending == 0 && value.Status.Running == 0 && value.Status.Succeeded == 0 && value.Status.Failed == 0 {

					// In some cases the object wrapped in the appwrapper never creates pod.  This likely  happens
					// in a custom resource that does some processing and errors occur before creating the pod or
					// even there is not a problem within the CR controler but when the K8s quota is hit not
					// allowing pods to get create due the admission controller.  This check will now put a timeout
					// on reserving these resources that are "in-flight")
					dispatchedCond := qjm.getLatestStatusConditionType(value, arbv1.AppWrapperCondDispatched)

					// If pod counts for AW have not updated within the timeout window, account for
					// this object's resources to give the object controller more time to start creating
					// pods.  This matters when resources are scare.  Once the timeout expires,
					// resources for this object will not be held and other AW may be dispatched which
					// could consume resources initially allocated for this object.  This is to handle
					// object controllers (essentially custom resource controllers) that do not work as
					// expected by creating pods.
					if qjm.waitForPodCountUpdates(dispatchedCond) {
						for _, resctrl := range qjm.qjobResControls {
							qjv := resctrl.GetAggregatedResources(value)
							pending = pending.Add(qjv)
							klog.V(4).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but no pod counts in the state have been defined.", qjv, resctrl, value.Name, value.Status.CanRun, value.Status.State)
						}
						for _, genericItem := range value.Spec.AggrResources.GenericItems {
							qjv, _ := genericresource.GetResources(&genericItem)
							pending = pending.Add(qjv)
							klog.V(4).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but no pod counts in the state have been defined.", qjv, genericItem, value.Name, value.Status.CanRun, value.Status.State)
						}
					} else {
						klog.V(4).Infof("[getAggAvaiResPri] Resources will no longer be reserved for %s/%s due to timeout of %d ms for pod creating.", value.Name, value.Namespace, qjm.serverOption.DispatchResourceReservationTimeout)
					}
				}
			}
			continue
		} else {
			//Do nothing
		}
	}

	proposedPremptions := qjm.getProposedPreemptions(requestingJob, r, preemptableAWs, preemptableAWsMap)

	klog.V(6).Infof("[getAggAvaiResPri] Schedulable idle cluster resources: %+v, subtracting dispatched resources: %+v and adding preemptable cluster resources: %+v", r, pending, preemptable)
	r = r.Add(preemptable)
	r, _ = r.NonNegSub(pending)

	klog.V(3).Infof("[getAggAvaiResPri] %+v available resources to schedule", r)
	return r, proposedPremptions
}

func (qjm *XController) chooseAgent(qj *arbv1.AppWrapper) string {

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

		//Now evaluate quota
		if qjm.serverOption.QuotaEnabled {
			if qjm.quotaManager != nil {
				if fits, preemptAWs, _ := qjm.quotaManager.Fits(qj, qjAggrResources, proposedPreemptions); fits {
					klog.V(2).Infof("[chooseAgent] AppWrapper %s has enough quota.\n", qj.Name)
					qjm.preemptAWJobs(preemptAWs)
					return agentId
				} else {
					klog.V(2).Infof("[chooseAgent] AppWrapper %s  does not have enough quota\n", qj.Name)
				}
			} else {
				klog.Errorf("[chooseAgent] Quota evaluation is enable but not initialize.  AppWrapper %s/%s does not have enough quota\n", qj.Name, qj.Namespace)
			}
		} else {
			//Quota is not enabled to return selected agent
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
func (qjm *XController) ScheduleNext() {
	// get next QJ from the queue
	// check if we have enough compute resources for it
	// if we have enough compute resources then we set the AllocatedReplicas to the total
	// amount of resources asked by the job
	qj, err := qjm.qjqueue.Pop()
	qjm.schedulingAW = qj
	if err != nil {
		klog.V(3).Infof("[ScheduleNext] Cannot pop QueueJob from qjqueue! err=%#v", err)
		return // Try to pop qjqueue again
	} else {
		klog.V(3).Infof("[ScheduleNext] activeQ.Pop %s *Delay=%.6f seconds RemainingLength=%d &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.Length(), qj, qj.ResourceVersion, qj.Status)
	}

	apiCacheAWJob, e := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
	// apiQueueJob's ControllerFirstTimestamp is only microsecond level instead of nanosecond level
	if e != nil {
		klog.Errorf("[ScheduleNext] Unable to get AW %s from API cache &aw=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
		return
	}
	// make sure qj has the latest information
	if larger(apiCacheAWJob.ResourceVersion, qj.ResourceVersion) {
		klog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache          &qj=%p          qj=%+v", qj.Name, qj, qj)
		klog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiCacheAWJob.Name, apiCacheAWJob, apiCacheAWJob)
		apiCacheAWJob.DeepCopyInto(qj)
	}

	// Re-compute SystemPriority for DynamicPriority policy
	if qjm.serverOption.DynamicPriority {
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
		if klog.V(10).Enabled() {
			pq := qjm.qjqueue.(*PriorityQueue)
			if qjm.qjqueue.Length() > 0 {
				for key, element := range pq.activeQ.data.items {
					qjtemp := element.obj.(*arbv1.AppWrapper)
					klog.V(10).Infof("[ScheduleNext] AfterCalc: qjqLength=%d Key=%s index=%d Priority=%.1f SystemPriority=%.1f QueueJobState=%s",
						qjm.qjqueue.Length(), key, element.index, float64(qjtemp.Spec.Priority), qjtemp.Status.SystemPriority, qjtemp.Status.QueueJobState)
				}
			}
		}

		// Retrieve HeadOfLine after priority update
		qj, err = qjm.qjqueue.Pop()
		qjm.schedulingAW = qj
		if err != nil {
			klog.V(3).Infof("[ScheduleNext] Cannot pop QueueJob from qjqueue! err=%#v", err)
		} else {
			klog.V(3).Infof("[ScheduleNext] activeQ.Pop_afterPriorityUpdate %s *Delay=%.6f seconds RemainingLength=%d &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.Length(), qj, qj.ResourceVersion, qj.Status)
		}
	}

	if qj.Status.CanRun {
		klog.V(10).Infof("[ScheduleNext] AppWrapper job: %s from prioirty queue is already scheduled. Ignoring request: Status=%+v\n", qj.Name, qj.Status)
		return
	}
	apiCacheAppWrapper, err := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
	if err != nil {
		klog.Errorf("[ScheduleNext] Fail get AppWrapper job: %s from the api cache, err=%#v", qj.Name, err)
		return
	}
	if apiCacheAppWrapper.Status.CanRun {
		klog.V(10).Infof("[ScheduleNext] AppWrapper job: %s from API is already scheduled. Ignoring request: Status=%+v\n", apiCacheAppWrapper.Name, apiCacheAppWrapper.Status)
		return
	}

	qj.Status.QueueJobState = arbv1.AppWrapperCondHeadOfLine
	qjm.addOrUpdateCondition(qj, arbv1.AppWrapperCondHeadOfLine, v1.ConditionTrue, "FrontOfQueue.", "")

	qj.Status.FilterIgnore = true // update QueueJobState only
	qjm.updateEtcd(qj, "ScheduleNext - setHOL")
	qjm.qjqueue.AddUnschedulableIfNotPresent(qj) // working on qj, avoid other threads putting it back to activeQ

	klog.V(10).Infof("[ScheduleNext] after Pop qjqLength=%d qj %s Version=%s activeQ=%t Unsched=%t Status=%+v", qjm.qjqueue.Length(), qj.Name, qj.ResourceVersion, qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj.Status)
	if qjm.isDispatcher {
		klog.V(2).Infof("[ScheduleNext] [Dispatcher Mode] Dispatch Next QueueJob: %s\n", qj.Name)
	} else {
		klog.V(2).Infof("[ScheduleNext] [Agent Mode] Deploy Next QueueJob: %s Status=%+v\n", qj.Name, qj.Status)
	}

	dispatchFailedReason := "AppWrapperNotRunnable."
	dispatchFailedMessage := ""
	if qjm.isDispatcher { // Dispatcher Mode
		agentId := qjm.chooseAgent(qj)
		if agentId != "" { // A proper agent is found.
			// Update states (CanRun=True) of XQJ in API Server
			// Add XQJ -> Agent Map
			apiQueueJob, err := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
			if err != nil {
				klog.Errorf("[ScheduleNext] Fail get AppWrapper job: %s from the api cache, err=%#v", qj.Name, err)
				return
			}
			// make sure qj has the latest information
			if larger(apiQueueJob.ResourceVersion, qj.ResourceVersion) {
				klog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache          &qj=%p          qj=%+v", qj.Name, qj, qj)
				klog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiQueueJob.Name, apiQueueJob, apiQueueJob)
				apiQueueJob.DeepCopyInto(qj)
			}

			//apiQueueJob.Status.CanRun = true
			qj.Status.CanRun = true
			queueJobKey, _ := GetQueueJobKey(qj)
			qjm.dispatchMap[queueJobKey] = agentId
			klog.V(10).Infof("[TTime] %s, %s: ScheduleNextBeforeEtcd", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			qjm.updateEtcd(qj, "ScheduleNext - setCanRun")
			if err := qjm.eventQueue.Add(qj); err != nil { // unsuccessful add to eventQueue, add back to activeQ
				klog.Errorf("[ScheduleNext] Fail to add %s to eventQueue, activeQ.Add_toSchedulingQueue &qj=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
				qjm.qjqueue.MoveToActiveQueueIfExists(qj)
			} else { // successful add to eventQueue, remove from qjqueue
				if qjm.qjqueue.IfExist(qj) {
					klog.V(10).Infof("[ScheduleNext] AppWrapper %s will be deleted from priority queue and sent to event queue", qj.Name)
				}
				qjm.qjqueue.Delete(qj)
			}

			//if _, err := qjm.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(apiQueueJob); err != nil {
			//	klog.Errorf("Failed to update status of AppWrapper %v/%v: %v", qj.Namespace, qj.Name, err)
			//}
			klog.V(10).Infof("[TTime] %s, %s: ScheduleNextAfterEtcd", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			return
		} else {
			dispatchFailedMessage = "Cannot find an cluster with enough resources to dispatch AppWrapper."
			klog.V(2).Infof("[Controller: Dispatcher Mode] %s %s\n", dispatchFailedReason, dispatchFailedMessage)
			go qjm.backoff(qj, dispatchFailedReason, dispatchFailedMessage)
		}
	} else { // Agent Mode
		aggqj := qjm.GetAggregatedResources(qj)

		// HeadOfLine logic
		HOLStartTime := time.Now()
		forwarded := false
		// Try to forward to eventQueue for at most HeadOfLineHoldingTime
		for !forwarded {
			priorityindex := qj.Status.SystemPriority
			// Support for Non-Preemption
			if !qjm.serverOption.Preemption {
				priorityindex = -math.MaxFloat64
			}
			// Disable Preemption under DynamicPriority.  Comment out if allow DynamicPriority and Preemption at the same time.
			if qjm.serverOption.DynamicPriority {
				priorityindex = -math.MaxFloat64
			}
			resources, proposedPreemptions := qjm.getAggregatedAvailableResourcesPriority(
				qjm.cache.GetUnallocatedResources(), priorityindex, qj, "")
			klog.V(2).Infof("[ScheduleNext] XQJ %s with resources %v to be scheduled on aggregated idle resources %v", qj.Name, aggqj, resources)

			if aggqj.LessEqual(resources) && qjm.nodeChecks(qjm.cache.GetUnallocatedHistograms(), qj) {
				//Now evaluate quota
				fits := true
				klog.V(10).Infof("[ScheduleNext] HOL available resourse successful check for %s at %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v due to quota limits", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				if qjm.serverOption.QuotaEnabled {
					if qjm.quotaManager != nil {
						quotaFits, preemptAWs, msg := qjm.quotaManager.Fits(qj, aggqj, proposedPreemptions)
						if quotaFits {
							klog.V(4).Infof("[ScheduleNext] HOL quota evaluation successful %s for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v due to quota limits", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
							// Set any jobs that are marked for preemption
							qjm.preemptAWJobs(preemptAWs)
						} else { // Not enough free quota to dispatch appwrapper
							dispatchFailedMessage = "Insufficient quota to dispatch AppWrapper."
							if len(msg) > 0 {
								dispatchFailedReason += " "
								dispatchFailedReason += msg
							}
							klog.V(3).Infof("[ScheduleNext] HOL Blocking by %s for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v msg=%s, due to quota limits",
								qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status, msg)
						}
						fits = quotaFits
					} else {
						fits = false
						//Quota manager not initialized
						dispatchFailedMessage = "Quota evaluation is enable but not initialize. Insufficient quota to dispatch AppWrapper."
						klog.Errorf("[ScheduleNext] Quota evaluation is enable but not initialize.  AppWrapper %s/%s does not have enough quota\n", qj.Name, qj.Namespace)
					}
				} else {
					klog.V(10).Infof("[ScheduleNext] HOL quota evaluation not enabled for %s at %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v due to quota limits", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				}

				// If quota evalauation is set or quota evaluation not enabled set the appwrapper to be dispatched
				if fits {
					// aw is ready to go!
					apiQueueJob, e := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
					// apiQueueJob's ControllerFirstTimestamp is only microsecond level instead of nanosecond level
					if e != nil {
						klog.Errorf("[ScheduleNext] Unable to get AW %s from API cache &aw=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
						return
					}
					// make sure qj has the latest information
					if larger(apiQueueJob.ResourceVersion, qj.ResourceVersion) {
						klog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache          &qj=%p          qj=%+v", qj.Name, qj, qj)
						klog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiQueueJob.Name, apiQueueJob, apiQueueJob)
						apiQueueJob.DeepCopyInto(qj)
					}
					desired := int32(0)
					for i, ar := range qj.Spec.AggrResources.Items {
						desired += ar.Replicas
						qj.Spec.AggrResources.Items[i].AllocatedReplicas = ar.Replicas
					}
					qj.Status.CanRun = true
					qj.Status.FilterIgnore = true // update CanRun & Spec.  no need to trigger event
					// Handle k8s watch race condition
					if err := qjm.updateEtcd(qj, "ScheduleNext - setCanRun"); err == nil {
						// add to eventQueue for dispatching to Etcd
						if err = qjm.eventQueue.Add(qj); err != nil { // unsuccessful add to eventQueue, add back to activeQ
							klog.Errorf("[ScheduleNext] Fail to add %s to eventQueue, activeQ.Add_toSchedulingQueue &qj=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
							qjm.qjqueue.MoveToActiveQueueIfExists(qj)
						} else { // successful add to eventQueue, remove from qjqueue
							qjm.qjqueue.Delete(qj)
							forwarded = true
							klog.V(3).Infof("[ScheduleNext] %s Delay=%.6f seconds eventQueue.Add_afterHeadOfLine activeQ=%t, Unsched=%t &aw=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
						}
					} //updateEtcd
				} //fits
			} else { // Not enough free resources to dispatch HOL
				dispatchFailedMessage = "Insufficient resources to dispatch AppWrapper."
				klog.V(3).Infof("[ScheduleNext] HOL Blocking by %s for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			}
			// stop trying to dispatch after HeadOfLineHoldingTime
			if forwarded || time.Now().After(HOLStartTime.Add(time.Duration(qjm.serverOption.HeadOfLineHoldingTime)*time.Second)) {
				break
			} else { // Try to dispatch again after one second
				time.Sleep(time.Second * 1)
			}
		}
		if !forwarded { // start thread to backoff
			klog.V(3).Infof("[ScheduleNext] HOL backoff %s after waiting for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			go qjm.backoff(qj, dispatchFailedReason, dispatchFailedMessage)
		}
	}
}

// Update AppWrappers in etcd
// todo: This is a current workaround for duplicate message bug.
func (cc *XController) updateEtcd(qj *arbv1.AppWrapper, at string) error {
	//apiCacheAWJob, e := cc.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
	//
	//if (e != nil) {
	//	klog.Errorf("[updateEtcd] Failed to update status of AppWrapper %s, namespace: %s at %s err=%v",
	//		apiCacheAWJob.Name, apiCacheAWJob.Namespace, at, e)
	//	return e
	//}

	//TODO: Remove next line
	var apiCacheAWJob *arbv1.AppWrapper
	//TODO: Remove next line
	apiCacheAWJob = qj
	apiCacheAWJob.Status.Sender = "before " + at // set Sender string to indicate code location
	apiCacheAWJob.Status.Local = false           // for Informer FilterFunc to pickup
	if _, err := cc.arbclients.ArbV1().AppWrappers(apiCacheAWJob.Namespace).Update(apiCacheAWJob); err != nil {
		klog.Errorf("[updateEtcd] Failed to update status of AppWrapper %s, namespace: %s at %s err=%v",
			apiCacheAWJob.Name, apiCacheAWJob.Namespace, at, err)
		return err
		//	} else {  // qjj should be the same as qj except with newer ResourceVersion
		//		qj.ResourceVersion = qjj.ResourceVersion  // update new ResourceVersion from etcd
	}
	klog.V(10).Infof("[updateEtcd] AppWrapperUpdate success %s at %s &qj=%p qj=%+v",
		apiCacheAWJob.Name, at, apiCacheAWJob, apiCacheAWJob)
	//qj.Status.Local  = true           // for Informer FilterFunc to ignore duplicate
	//qj.Status.Sender = "after  "+ at  // set Sender string to indicate code location
	return nil
}

func (cc *XController) updateStatusInEtcd(qj *arbv1.AppWrapper, at string) error {
	var apiCacheAWJob *arbv1.AppWrapper
	apiCacheAWJob = qj
	if _, err := cc.arbclients.ArbV1().AppWrappers(apiCacheAWJob.Namespace).UpdateStatus(apiCacheAWJob); err != nil {
		klog.Errorf("[updateEtcd] Failed to update status of AppWrapper %s, namespace: %s at %s err=%v",
			apiCacheAWJob.Name, apiCacheAWJob.Namespace, at, err)
		return err
	}
	klog.V(10).Infof("[updateEtcd] AppWrapperUpdate success %s at %s &qj=%p qj=%+v",
		apiCacheAWJob.Name, at, apiCacheAWJob, apiCacheAWJob)
	return nil
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
		klog.V(10).Infof("[waitForPodCountUpdates] Dispatch duration time %d microseconds has reached timeout value of %d microseconds",
			timeSinceDispatched.Microseconds(), timeoutMicroSeconds)
	}

	klog.V(10).Infof("[waitForPodCountUpdates] Dispatch duration time %d microseconds has not reached timeout value of %d microseconds",
		timeSinceDispatched.Microseconds(), timeoutMicroSeconds)
	return true
}

func (qjm *XController) getLatestStatusConditionType(aw *arbv1.AppWrapper, condType arbv1.AppWrapperConditionType) *arbv1.AppWrapperCondition {
	var latestConditionBasedOnType arbv1.AppWrapperCondition
	if aw.Status.Conditions != nil && len(aw.Status.Conditions) > 0 {
		// Find the latest matching condition based on type not related other condition fields
		for _, condition := range aw.Status.Conditions {
			// Matching condition?
			if condition.Type == condType {
				//First time match?
				if (arbv1.AppWrapperCondition{} == latestConditionBasedOnType) {
					latestConditionBasedOnType = condition
				} else {
					// Compare current condition to last match and get keep the later condition
					currentCondLastUpdate := condition.LastUpdateMicroTime
					currentCondLastUpdatePtr := &currentCondLastUpdate
					lastCondLastUpdate := latestConditionBasedOnType.LastUpdateMicroTime
					lastCondLastUpdatePtr := &lastCondLastUpdate
					if lastCondLastUpdatePtr.Before(currentCondLastUpdatePtr) {
						latestConditionBasedOnType = condition
					}
				}
			} // Condition type match check
		} // Loop through conditions of AW
	} // AW has conditions?

	// If no matching condition found return nil otherwise return matching latest condition
	if (arbv1.AppWrapperCondition{} == latestConditionBasedOnType) {
		klog.V(10).Infof("[getLatestStatusConditionType] No disptach condition found for AppWrapper=%s/%s.",
			aw.Name, aw.Namespace)
		return nil
	} else {
		return &latestConditionBasedOnType
	}
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

func (qjm *XController) backoff(q *arbv1.AppWrapper, reason string, message string) {
	var workingAW *arbv1.AppWrapper
	apiCacheAWJob, e := qjm.queueJobLister.AppWrappers(q.Namespace).Get(q.Name)
	// Update condition
	if e == nil {
		workingAW = apiCacheAWJob
		apiCacheAWJob.Status.QueueJobState = arbv1.AppWrapperCondBackoff
		workingAW.Status.FilterIgnore = true // update QueueJobState only, no work needed
		qjm.addOrUpdateCondition(workingAW, arbv1.AppWrapperCondBackoff, v1.ConditionTrue, reason, message)
		//qjm.updateEtcd(workingAW, "backoff - Rejoining")
		qjm.updateStatusInEtcd(workingAW, "backoff - Rejoining")
	} else {
		workingAW = q
		klog.Errorf("[backoff] Failed to retrieve cached object for %s/%s.  Continuing with possible stale object without updating conditions.", workingAW.Namespace, workingAW.Name)

	}
	qjm.qjqueue.AddUnschedulableIfNotPresent(workingAW)
	klog.V(3).Infof("[backoff] %s move to unschedulableQ before sleep for %d seconds. activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", workingAW.Name,
		qjm.serverOption.BackoffTime, qjm.qjqueue.IfExistActiveQ((workingAW)), qjm.qjqueue.IfExistUnschedulableQ((workingAW)), workingAW, workingAW.ResourceVersion, workingAW.Status)
	time.Sleep(time.Duration(qjm.serverOption.BackoffTime) * time.Second)
	qjm.qjqueue.MoveToActiveQueueIfExists(workingAW)

	klog.V(3).Infof("[backoff] %s activeQ.Add after sleep for %d seconds. activeQ=%t Unsched=%t &aw=%p Version=%s Status=%+v", workingAW.Name,
		qjm.serverOption.BackoffTime, qjm.qjqueue.IfExistActiveQ((workingAW)), qjm.qjqueue.IfExistUnschedulableQ((workingAW)), workingAW, workingAW.ResourceVersion, workingAW.Status)
}

// Run start AppWrapper Controller
func (cc *XController) Run(stopCh chan struct{}) {
	// initialized
	createAppWrapperKind(cc.config)

	go cc.queueJobInformer.Informer().Run(stopCh)

	go cc.qjobResControls[arbv1.ResourceTypePod].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeService].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeDeployment].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeStatefulSet].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypePersistentVolume].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypePersistentVolumeClaim].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeNamespace].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeConfigMap].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeSecret].Run(stopCh)
	go cc.qjobResControls[arbv1.ResourceTypeNetworkPolicy].Run(stopCh)

	cache.WaitForCacheSync(stopCh, cc.queueJobSynced)

	// update snapshot of ClientStateCache every second
	cc.cache.Run(stopCh)

	// go wait.Until(cc.ScheduleNext, 2*time.Second, stopCh)
	go wait.Until(cc.ScheduleNext, 0, stopCh)
	// start preempt thread based on preemption of pods

	// TODO - scheduleNext...Job....
	// start preempt thread based on preemption of pods
	go wait.Until(cc.PreemptQueueJobs, 60*time.Second, stopCh)

	// This thread is used as a heartbeat to calculate runtime spec in the status
	go wait.Until(cc.UpdateQueueJobs, 5*time.Second, stopCh)

	if cc.isDispatcher {
		go wait.Until(cc.UpdateAgent, 2*time.Second, stopCh) // In the Agent?
		for _, jobClusterAgent := range cc.agentMap {
			go jobClusterAgent.Run(stopCh)
		}
		go wait.Until(cc.agentEventQueueWorker, time.Second, stopCh) // Update Agent Worker
	}

	// go wait.Until(cc.worker, time.Second, stopCh)
	go wait.Until(cc.worker, 0, stopCh)
}

func (qjm *XController) UpdateAgent() {
	klog.V(3).Infof("[Controller] Update AggrResources for All Agents\n")
	for _, jobClusterAgent := range qjm.agentMap {
		jobClusterAgent.UpdateAggrResources()
	}
}

func (qjm *XController) UpdateQueueJobs() {
	firstTime := metav1.NowMicro()
	// retrieve queueJobs from local cache.  no guarantee queueJobs contain up-to-date information
	queueJobs, err := qjm.queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Errorf("[UpdateQueueJobs] List of queueJobs err=%+v", err)
		return
	}
	for _, newjob := range queueJobs {
		// UpdateQueueJobs can be the first to see a new AppWrapper job, under heavy load
		if newjob.Status.QueueJobState == "" {
			newjob.Status.ControllerFirstTimestamp = firstTime
			newjob.Status.SystemPriority = float64(newjob.Spec.Priority)
			newjob.Status.QueueJobState = arbv1.AppWrapperCondInit
			newjob.Status.Conditions = []arbv1.AppWrapperCondition{
				arbv1.AppWrapperCondition{
					Type:                    arbv1.AppWrapperCondInit,
					Status:                  v1.ConditionTrue,
					LastUpdateMicroTime:     metav1.NowMicro(),
					LastTransitionMicroTime: metav1.NowMicro(),
				},
			}
			klog.V(3).Infof("[UpdateQueueJobs] %s 0Delay=%.6f seconds CreationTimestamp=%s ControllerFirstTimestamp=%s",
				newjob.Name, time.Now().Sub(newjob.Status.ControllerFirstTimestamp.Time).Seconds(), newjob.CreationTimestamp, newjob.Status.ControllerFirstTimestamp)
		}
		klog.V(10).Infof("[UpdateQueueJobs] %s: qjqueue=%t &qj=%p Version=%s Status=%+v", newjob.Name, qjm.qjqueue.IfExist(newjob), newjob, newjob.ResourceVersion, newjob.Status)
		// check eventQueue, qjqueue in program sequence to make sure job is not in qjqueue
		if _, exists, _ := qjm.eventQueue.Get(newjob); exists {
			continue
		} // do not enqueue if already in eventQueue
		if qjm.qjqueue.IfExist(newjob) {
			continue
		} // do not enqueue if already in qjqueue

		err = qjm.enqueueIfNotPresent(newjob)
		if err != nil {
			klog.Errorf("[UpdateQueueJobs] Fail to enqueue %s to eventQueue, ignore.  *Delay=%.6f seconds &qj=%p Version=%s Status=%+v err=%#v", newjob.Name, time.Now().Sub(newjob.Status.ControllerFirstTimestamp.Time).Seconds(), newjob, newjob.ResourceVersion, newjob.Status, err)
		} else {
			klog.V(4).Infof("[UpdateQueueJobs] %s *Delay=%.6f seconds eventQueue.Add_byUpdateQueueJobs &qj=%p Version=%s Status=%+v", newjob.Name, time.Now().Sub(newjob.Status.ControllerFirstTimestamp.Time).Seconds(), newjob, newjob.ResourceVersion, newjob.Status)
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
			arbv1.AppWrapperCondition{
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

	klog.V(10).Infof("[Informer-addQJ] %s Delay=%.6f seconds CreationTimestamp=%s ControllerFirstTimestamp=%s",
		qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.CreationTimestamp, qj.Status.ControllerFirstTimestamp)

	klog.V(4).Infof("[Informer-addQJ] enqueue %s &qj=%p Version=%s Status=%+v", qj.Name, qj, qj.ResourceVersion, qj.Status)
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
		klog.V(4).Infof("[Informer-updateQJ] %s *Delay=%.6f seconds BadOldObject enqueue &newQJ=%p Version=%s Status=%+v", newQJ.Name, time.Now().Sub(newQJ.Status.ControllerFirstTimestamp.Time).Seconds(), newQJ, newQJ.ResourceVersion, newQJ.Status)
		//cc.enqueue(newQJ)
		return
	}
	// AppWrappers may come out of order.  Ignore old ones.
	if (oldQJ.Name == newQJ.Name) && (larger(oldQJ.ResourceVersion, newQJ.ResourceVersion)) {
		klog.V(10).Infof("[Informer-updateQJ]  %s ignored OutOfOrder arrival &oldQJ=%p oldQJ=%+v", oldQJ.Name, oldQJ, oldQJ)
		klog.V(10).Infof("[Informer-updateQJ] %s ignored OutOfOrder arrival &newQJ=%p newQJ=%+v", newQJ.Name, newQJ, newQJ)
		return
	}

	if equality.Semantic.DeepEqual(newQJ.Status, oldQJ.Status) {
		klog.V(10).Infof("[Informer-updateQJ] No change to status field of AppWrapper: %s, oldAW=%+v, newAW=%+v.", newQJ.Name, oldQJ.Status, newQJ.Status)
	}

	klog.V(3).Infof("[Informer-updateQJ] %s *Delay=%.6f seconds normal enqueue &newQJ=%p Version=%s Status=%+v", newQJ.Name, time.Now().Sub(newQJ.Status.ControllerFirstTimestamp.Time).Seconds(), newQJ, newQJ.ResourceVersion, newQJ.Status)
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

func (cc *XController) deleteQueueJob(obj interface{}) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		klog.Errorf("[Informer-deleteQJ] obj is not AppWrapper. obj=%+v", obj)
		return
	}
	current_ts := metav1.NewTime(time.Now())
	klog.V(10).Infof("[Informer-deleteQJ] %s *Delay=%.6f seconds before enqueue &qj=%p Version=%s Status=%+v Deletion Timestame=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj, qj.ResourceVersion, qj.Status, qj.GetDeletionTimestamp())
	accessor, err := meta.Accessor(qj)
	if err != nil {
		klog.V(10).Infof("[Informer-deleteQJ] Error obtaining the accessor for AW job: %s", qj.Name)
		qj.SetDeletionTimestamp(&current_ts)
	} else {
		accessor.SetDeletionTimestamp(&current_ts)
	}
	klog.V(3).Infof("[Informer-deleteQJ] %s enqueue deletion, deletion ts = %v", qj.Name, qj.GetDeletionTimestamp())
	cc.enqueue(qj)
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
		if err := cc.updateQueueJobStatus(queuejob); err != nil {
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

func (cc *XController) updateQueueJobStatus(queueJobFromAgent *arbv1.AppWrapper) error {
	queueJobInEtcd, err := cc.queueJobLister.AppWrappers(queueJobFromAgent.Namespace).Get(queueJobFromAgent.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if cc.isDispatcher {
				cc.Cleanup(queueJobFromAgent)
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
	_, err = cc.arbclients.ArbV1().AppWrappers(queueJobInEtcd.Namespace).Update(queueJobInEtcd)
	if err != nil {
		return err
	}
	return nil
}

func (cc *XController) worker() {
	defer func() {
		if pErr := recover(); pErr != nil {
			klog.Errorf("[worker] Panic occurred error: %v, stacktrace: %s", pErr, string(debug.Stack()))
		}
	}()
	if _, err := cc.eventQueue.Pop(func(obj interface{}) error {
		var queuejob *arbv1.AppWrapper
		switch v := obj.(type) {
		case *arbv1.AppWrapper:
			queuejob = v
		default:
			klog.Errorf("[worker] eventQueue.Pop un-supported type. obj=%+v", obj)
			return nil
		}
		klog.V(10).Infof("[worker] %s *Delay=%.6f seconds eventQueue.Pop_begin &newQJ=%p Version=%s Status=%+v", queuejob.Name, time.Now().Sub(queuejob.Status.ControllerFirstTimestamp.Time).Seconds(), queuejob, queuejob.ResourceVersion, queuejob.Status)

		if queuejob == nil {
			if acc, err := meta.Accessor(obj); err != nil {
				klog.Warningf("[worker] Failed to get AppWrapper for %v/%v", acc.GetNamespace(), acc.GetName())
			}

			return nil
		}

		// sync AppWrapper
		if err := cc.syncQueueJob(queuejob); err != nil {
			klog.Errorf("[worker] Failed to sync AppWrapper %s, err %#v", queuejob.Name, err)
			// If any error, requeue it.
			return err
		}

		klog.V(10).Infof("[worker] Ending %s Delay=%.6f seconds &newQJ=%p Version=%s Status=%+v", queuejob.Name, time.Now().Sub(queuejob.Status.ControllerFirstTimestamp.Time).Seconds(), queuejob, queuejob.ResourceVersion, queuejob.Status)
		return nil
	}); err != nil {
		klog.Errorf("[worker] Fail to pop item from eventQueue, err %#v", err)
		return
	}
}

func (cc *XController) syncQueueJob(qj *arbv1.AppWrapper) error {
	cacheAWJob, err := cc.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
	if err != nil {
		klog.V(10).Infof("[syncQueueJob] AppWrapper %s not found in cache: info=%+v", qj.Name, err)
		// Implicit detection of deletion
		if apierrors.IsNotFound(err) {
			//if (cc.isDispatcher) {
			cc.Cleanup(qj)
			cc.qjqueue.Delete(qj)
			//}
			return nil
		}
		return err
	}
	klog.V(10).Infof("[syncQueueJob] Cache AW %s &qj=%p Version=%s Status=%+v", qj.Name, qj, qj.ResourceVersion, qj.Status)

	// make sure qj has the latest information
	if larger(qj.ResourceVersion, qj.ResourceVersion) {
		klog.V(10).Infof("[syncQueueJob] %s found more recent copy from cache       &qj=%p       qj=%+v", qj.Name, qj, qj)
		klog.V(10).Infof("[syncQueueJobJ] %s found more recent copy from cache &cacheAWJob=%p cacheAWJob=%+v", cacheAWJob.Name, cacheAWJob, cacheAWJob)
		cacheAWJob.DeepCopyInto(qj)
	}

	// If it is Agent (not a dispatcher), update pod information
	podPhaseChanges := false
	if !cc.isDispatcher {
		//Make a copy first to not update cache object and to use for comparing
		awNew := qj.DeepCopy()
		// we call sync to update pods running, pending,...
		if qj.Status.State == arbv1.AppWrapperStateActive {
			err := cc.qjobResControls[arbv1.ResourceTypePod].UpdateQueueJobStatus(awNew)
			if err != nil {
				klog.Errorf("[syncQueueJob] Error updating pod status counts for AppWrapper job: %s, err=%+v", qj.Name, err)
			}
			klog.V(10).Infof("[syncQueueJob] AW popped from event queue %s &qj=%p Version=%s Status=%+v", awNew.Name, awNew, awNew.ResourceVersion, awNew.Status)

			// Update etcd conditions if AppWrapper Job has at least 1 running pod and transitioning from dispatched to running.
			if (awNew.Status.QueueJobState != arbv1.AppWrapperCondRunning) && (awNew.Status.Running > 0) {
				awNew.Status.QueueJobState = arbv1.AppWrapperCondRunning
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunning, v1.ConditionTrue, "PodsRunning", "")
				awNew.Status.Conditions = append(awNew.Status.Conditions, cond)
				awNew.Status.FilterIgnore = true // Update AppWrapperCondRunning
				cc.updateEtcd(awNew, "[syncQueueJob] setRunning")
			}

			//For debugging?
			if !reflect.DeepEqual(awNew.Status, qj.Status) {
				podPhaseChanges = true
				// Using DeepCopy before DeepCopyInto as it seems that DeepCopyInto does not alloc a new memory object
				awNewStatus := awNew.Status.DeepCopy()
				awNewStatus.DeepCopyInto(&qj.Status)
				//awNew.Status.DeepCopy().DeepCopyInto(&qj.Status)
				klog.V(10).Infof("[syncQueueJob] AW pod phase change(s) detected %s &eventqueueaw=%p eventqueueawVersion=%s eventqueueawStatus=%+v; &newaw=%p newawVersion=%s newawStatus=%+v",
					qj.Name, qj, qj.ResourceVersion, qj.Status, awNew, awNew.ResourceVersion, awNew.Status)
			}
		}
	}

	return cc.manageQueueJob(qj, podPhaseChanges)
	//return cc.manageQueueJob(cacheAWJob)
}

// manageQueueJob is the core method responsible for managing the number of running
// pods according to what is specified in the job.Spec.
// Does NOT modify <activePods>.
func (cc *XController) manageQueueJob(qj *arbv1.AppWrapper, podPhaseChanges bool) error {
	var err error
	startTime := time.Now()
	defer func() {
		klog.V(10).Infof("[worker-manageQJ] Ending %s manageQJ time=%s &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(startTime), qj, qj.ResourceVersion, qj.Status)
	}()

	if !cc.isDispatcher { // Agent Mode

		if qj.DeletionTimestamp != nil {

			klog.V(4).Infof("[manageQueueJob] AW job=%s/%s set for deletion.", qj.Name, qj.Namespace)

			// cleanup resources for running job
			err = cc.Cleanup(qj)
			if err != nil {
				return err
			}
			//empty finalizers and delete the queuejob again
			accessor, err := meta.Accessor(qj)
			if err != nil {
				return err
			}
			accessor.SetFinalizers(nil)

			// we delete the job from the queue if it is there
			cc.qjqueue.Delete(qj)

			return nil
		}
		//Job is Complete only update pods if needed.
		if qj.Status.State == arbv1.AppWrapperStateCompleted || qj.Status.State == arbv1.AppWrapperStateRunningHoldCompletion {
			if podPhaseChanges {
				// Only update etcd if AW status has changed.  This can happen for periodic
				// updates of pod phase counts done in caller of this function.
				if err := cc.updateEtcd(qj, "manageQueueJob - podPhaseChanges"); err != nil {
					klog.Errorf("[manageQueueJob] Error updating etc for AW job=%s Status=%+v err=%+v", qj.Name, qj.Status, err)
				}
			}
			return nil
		}

		// First execution of qj to set Status.State = Enqueued
		if !qj.Status.CanRun && (qj.Status.State != arbv1.AppWrapperStateEnqueued && qj.Status.State != arbv1.AppWrapperStateDeleted) {
			// if there are running resources for this job then delete them because the job was put in
			// pending state...

			// If this the first time seeing this AW, no need to delete.
			stateLen := len(qj.Status.State)
			if stateLen > 0 {
				klog.V(2).Infof("[manageQueueJob] Deleting resources for AppWrapper Job %s because it was preempted, status=%+v\n", qj.Name, qj.Status)
				err = cc.Cleanup(qj)
				klog.V(8).Infof("[manageQueueJob] Validation after deleting resources for AppWrapper Job %s because it was be preempted, status=%+v\n", qj.Name, qj.Status)
				if err != nil {
					klog.Errorf("[manageQueueJob] Fail to delete resources for AppWrapper Job %s, err=%#v", qj.Name, err)
					return err
				}
			}

			qj.Status.State = arbv1.AppWrapperStateEnqueued
			//  add qj to qjqueue only when it is not in UnschedulableQ
			if cc.qjqueue.IfExistUnschedulableQ(qj) {
				klog.V(10).Infof("[worker-manageQJ] leaving %s to qjqueue.UnschedulableQ activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			} else {
				klog.V(10).Infof("[worker-manageQJ] before add to activeQ %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				index := getIndexOfMatchedCondition(qj, arbv1.AppWrapperCondQueueing, "AwaitingHeadOfLine")
				if index < 0 {
					qj.Status.QueueJobState = arbv1.AppWrapperCondQueueing
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondQueueing, v1.ConditionTrue, "AwaitingHeadOfLine", "")
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondQueueing, v1.ConditionTrue, "AwaitingHeadOfLine", "")
					qj.Status.Conditions[index] = *cond.DeepCopy()
				}

				qj.Status.FilterIgnore = true // Update Queueing status, add to qjqueue for ScheduleNext
				cc.updateEtcd(qj, "manageQueueJob - setQueueing")
				klog.V(10).Infof("[worker-manageQJ] before add to activeQ %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				if err = cc.qjqueue.AddIfNotPresent(qj); err != nil {
					klog.Errorf("[worker-manageQJ] Fail to add %s to activeQueue. Back to eventQueue activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v err=%#v",
						qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status, err)
					cc.enqueue(qj)
				} else {
					klog.V(3).Infof("[worker-manageQJ] %s 1Delay=%.6f seconds activeQ.Add_success activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v",
						qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				}
			}
			return nil
		} // End of first execution of qj to add to qjqueue for ScheduleNext

		//Handle recovery condition
		if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued &&
			!cc.qjqueue.IfExistUnschedulableQ(qj) && !cc.qjqueue.IfExistActiveQ(qj) {
			// One more check to ensure AW is not the current active schedule object
			if cc.schedulingAW == nil ||
				(strings.Compare(cc.schedulingAW.Namespace, qj.Namespace) != 0 &&
					strings.Compare(cc.schedulingAW.Name, qj.Name) != 0) {
				cc.qjqueue.AddIfNotPresent(qj)
				klog.V(3).Infof("[manageQueueJob] Recovered AppWrapper %s%s - added to active queue, Status=%+v",
					qj.Namespace, qj.Name, qj.Status)
				return nil
			}
		}

		// add qj to Etcd for dispatch
		if qj.Status.CanRun && qj.Status.State != arbv1.AppWrapperStateActive &&
			qj.Status.State != arbv1.AppWrapperStateCompleted &&
			qj.Status.State != arbv1.AppWrapperStateRunningHoldCompletion {
			//keep conditions until the appwrapper is re-dispatched
			qj.Status.PendingPodConditions = nil

			qj.Status.State = arbv1.AppWrapperStateActive
			// Bugfix to eliminate performance problem of overloading the event queue.}

			if qj.Spec.AggrResources.Items != nil {
				for i := range qj.Spec.AggrResources.Items {
					err := cc.refManager.AddTag(&qj.Spec.AggrResources.Items[i], func() string {
						return strconv.Itoa(i)
					})
					if err != nil {
						return err
					}
				}
			}
			klog.V(3).Infof("[worker-manageQJ] %s 3Delay=%.6f seconds BeforeDispatchingToEtcd Version=%s Status=%+v",
				qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.ResourceVersion, qj.Status)
			dispatched := true
			dispatchFailureReason := "ItemCreationFailure."
			dispatchFailureMessage := ""
			for _, ar := range qj.Spec.AggrResources.Items {
				klog.V(10).Infof("[worker-manageQJ] before dispatch [%v].SyncQueueJob %s &qj=%p Version=%s Status=%+v", ar.Type, qj.Name, qj, qj.ResourceVersion, qj.Status)
				// Call Resource Controller of ar.Type to issue REST call to Etcd for resource creation
				err00 := cc.qjobResControls[ar.Type].SyncQueueJob(qj, &ar)
				if err00 != nil {
					dispatchFailureMessage = fmt.Sprintf("%s/%s creation failure: %+v", qj.Namespace, qj.Name, err00)
					klog.V(3).Infof("[worker-manageQJ] Error dispatching job=%s type=%v Status=%+v err=%+v", qj.Name, ar.Type, qj.Status, err00)
					dispatched = false
					break
				}
			}
			// Handle generic resources
			for _, ar := range qj.Spec.AggrResources.GenericItems {
				klog.V(10).Infof("[worker-manageQJ] before dispatch Generic.SyncQueueJob %s &qj=%p Version=%s Status=%+v", qj.Name, qj, qj.ResourceVersion, qj.Status)
				_, err00 := cc.genericresources.SyncQueueJob(qj, &ar)
				if err00 != nil {
					dispatchFailureMessage = fmt.Sprintf("%s/%s creation failure: %+v", qj.Namespace, qj.Name, err00)
					klog.Errorf("[worker-manageQJ] Error dispatching job=%s Status=%+v err=%+v", qj.Name, qj.Status, err00)
					dispatched = false
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

				klog.V(3).Infof("[worker-manageQJ] %s 4Delay=%.6f seconds AllResourceDispatchedToEtcd Version=%s Status=%+v",
					qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.ResourceVersion, qj.Status)

			} else {
				qj.Status.State = arbv1.AppWrapperStateFailed
				qj.Status.QueueJobState = arbv1.AppWrapperCondFailed
				if !isLastConditionDuplicate(qj, arbv1.AppWrapperCondFailed, v1.ConditionTrue, dispatchFailureReason, dispatchFailureMessage) {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondFailed, v1.ConditionTrue, dispatchFailureReason, dispatchFailureMessage)
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
				}
				cc.Cleanup(qj)
			}

			// TODO(k82cn): replaced it with `UpdateStatus`
			qj.Status.FilterIgnore = true // update State & QueueJobState after dispatch
			if err := cc.updateEtcd(qj, "manageQueueJob - afterEtcdDispatching"); err != nil {
				klog.Errorf("[manageQueueJob] Error updating etc for AW job=%s Status=%+v err=%+v", qj.Name, qj.Status, err)
				return err
			}

		} else if qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateActive {
			//set appwrapper status to Complete or RunningHoldCompletion
			derivedAwStatus := cc.getAppWrapperCompletionStatus(qj)

			//Set Appwrapper state to complete if all items in Appwrapper
			//are completed
			if derivedAwStatus == arbv1.AppWrapperStateRunningHoldCompletion {
				qj.Status.State = derivedAwStatus
				var updateQj *arbv1.AppWrapper
				index := getIndexOfMatchedCondition(qj, arbv1.AppWrapperCondRunningHoldCompletion, "SomeItemsCompleted")
				if index < 0 {
					qj.Status.QueueJobState = arbv1.AppWrapperCondRunningHoldCompletion
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunningHoldCompletion, v1.ConditionTrue, "SomeItemsCompleted", "")
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
					qj.Status.FilterIgnore = true // Update AppWrapperCondRunningHoldCompletion
					updateQj = qj.DeepCopy()
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunningHoldCompletion, v1.ConditionTrue, "SomeItemsCompleted", "")
					qj.Status.Conditions[index] = *cond.DeepCopy()
					updateQj = qj.DeepCopy()
				}
				cc.updateEtcd(updateQj, "[syncQueueJob] setRunningHoldCompletion")
			}
			//Set appwrapper status to complete
			if derivedAwStatus == arbv1.AppWrapperStateCompleted {
				qj.Status.State = derivedAwStatus
				qj.Status.CanRun = false
				var updateQj *arbv1.AppWrapper
				index := getIndexOfMatchedCondition(qj, arbv1.AppWrapperCondCompleted, "PodsCompleted")
				if index < 0 {
					qj.Status.QueueJobState = arbv1.AppWrapperCondCompleted
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondCompleted, v1.ConditionTrue, "PodsCompleted", "")
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
					qj.Status.FilterIgnore = true // Update AppWrapperCondCompleted
					updateQj = qj.DeepCopy()
				} else {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondCompleted, v1.ConditionTrue, "PodsCompleted", "")
					qj.Status.Conditions[index] = *cond.DeepCopy()
					updateQj = qj.DeepCopy()
				}
				cc.updateEtcd(updateQj, "[syncQueueJob] setCompleted")
			}

			// Bugfix to eliminate performance problem of overloading the event queue.
		} else if podPhaseChanges { // Continued bug fix
			// Only update etcd if AW status has changed.  This can happen for periodic
			// updates of pod phase counts done in caller of this function.
			if err := cc.updateEtcd(qj, "manageQueueJob - podPhaseChanges"); err != nil {
				klog.Errorf("[manageQueueJob] Error updating etc for AW job=%s Status=%+v err=%+v", qj.Name, qj.Status, err)
			}
		}
		// Finish adding qj to Etcd for dispatch

	} else { // Dispatcher Mode

		if qj.DeletionTimestamp != nil {
			// cleanup resources for running job
			err = cc.Cleanup(qj)
			if err != nil {
				return err
			}
			//empty finalizers and delete the queuejob again
			accessor, err := meta.Accessor(qj)
			if err != nil {
				return err
			}
			accessor.SetFinalizers(nil)

			cc.qjqueue.Delete(qj)

			return nil
		}

		if !qj.Status.CanRun && (qj.Status.State != arbv1.AppWrapperStateEnqueued && qj.Status.State != arbv1.AppWrapperStateDeleted) {
			// if there are running resources for this job then delete them because the job was put in
			// pending state...
			klog.V(3).Infof("[worker-manageQJ] Deleting AppWrapper resources because it will be preempted! %s", qj.Name)
			err = cc.Cleanup(qj)
			if err != nil {
				return err
			}

			qj.Status.State = arbv1.AppWrapperStateEnqueued
			if cc.qjqueue.IfExistUnschedulableQ(qj) {
				klog.V(10).Infof("[worker-manageQJ] leaving %s to qjqueue.UnschedulableQ activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			} else {
				klog.V(10).Infof("[worker-manageQJ] before add to activeQ %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				qj.Status.QueueJobState = arbv1.AppWrapperCondQueueing
				qj.Status.FilterIgnore = true // Update Queueing status, add to qjqueue for ScheduleNext
				cc.updateEtcd(qj, "manageQueueJob - setQueueing")
				if err = cc.qjqueue.AddIfNotPresent(qj); err != nil {
					klog.Errorf("[worker-manageQJ] Fail to add %s to activeQueue. Back to eventQueue activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v err=%#v",
						qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status, err)
					cc.enqueue(qj)
				} else {
					klog.V(4).Infof("[worker-manageQJ] %s 1Delay=%.6f seconds activeQ.Add_success activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v",
						qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				}
			}

			//_, err = cc.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(qj)
			//if err != nil {
			//	return err
			//}
			return nil
		}

		// if !qj.Status.CanRun && qj.Status.State == arbv1.QueueJobStateEnqueued {
		if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued {
			cc.qjqueue.AddIfNotPresent(qj)
			return nil
		}

		if qj.Status.CanRun && !qj.Status.IsDispatched {
			if klog.V(10).Enabled() {
				current_time := time.Now()
				klog.V(10).Infof("[worker-manageQJ] XQJ %s has Overhead Before Dispatching: %s", qj.Name, current_time.Sub(qj.CreationTimestamp.Time))
				klog.V(10).Infof("[TTime] %s, %s: WorkerBeforeDispatch", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			}

			queuejobKey, _ := GetQueueJobKey(qj)
			// agentId:=cc.dispatchMap[queuejobKey]
			// if agentId!=nil {
			if agentId, ok := cc.dispatchMap[queuejobKey]; ok {
				klog.V(10).Infof("[Dispatcher Controller] Dispatched AppWrapper %s to Agent ID: %s.", qj.Name, agentId)
				cc.agentMap[agentId].CreateJob(qj)
				qj.Status.IsDispatched = true
			} else {
				klog.Errorf("[Dispatcher Controller] AppWrapper %s not found in dispatcher mapping.", qj.Name)
			}
			if klog.V(10).Enabled() {
				current_time := time.Now()
				klog.V(10).Infof("[Dispatcher Controller] XQJ %s has Overhead After Dispatching: %s", qj.Name, current_time.Sub(qj.CreationTimestamp.Time))
				klog.V(10).Infof("[TTime] %s, %s: WorkerAfterDispatch", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			}

			if _, err := cc.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(qj); err != nil {
				klog.Errorf("Failed to update status of AppWrapper %v/%v: %v",
					qj.Namespace, qj.Name, err)
				return err
			}
		}

	}
	return err
}

//Cleanup function
func (cc *XController) Cleanup(appwrapper *arbv1.AppWrapper) error {
	klog.V(3).Infof("[Cleanup] begin AppWrapper %s Version=%s Status=%+v\n", appwrapper.Name, appwrapper.ResourceVersion, appwrapper.Status)

	if !cc.isDispatcher {
		if appwrapper.Spec.AggrResources.Items != nil {
			// we call clean-up for each controller
			for _, ar := range appwrapper.Spec.AggrResources.Items {
				err00 := cc.qjobResControls[ar.Type].Cleanup(appwrapper, &ar)
				if err00 != nil {
					klog.Errorf("[Cleanup] Error deleting item %s from job=%s Status=%+v err=%+v.",
						ar.Type, appwrapper.Name, appwrapper.Status, err00)
				}
			}
		}
		if appwrapper.Spec.AggrResources.GenericItems != nil {
			for _, ar := range appwrapper.Spec.AggrResources.GenericItems {
				genericResourceName, gvk, err00 := cc.genericresources.Cleanup(appwrapper, &ar)
				if err00 != nil {
					klog.Errorf("[Cleanup] Error deleting generic item %s, GVK=%s.%s.%s from job=%s Status=%+v err=%+v.",
						genericResourceName, gvk.Group, gvk.Version, gvk.Kind, appwrapper.Name, appwrapper.Status, err00)
				}
			}
		}

	} else {
		// klog.Infof("[Dispatcher] Cleanup: State=%s\n", appwrapper.Status.State)
		//if ! appwrapper.Status.CanRun && appwrapper.Status.IsDispatched {
		if appwrapper.Status.IsDispatched {
			queuejobKey, _ := GetQueueJobKey(appwrapper)
			if obj, ok := cc.dispatchMap[queuejobKey]; ok {
				cc.agentMap[obj].DeleteJob(appwrapper)
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
	klog.V(10).Infof("[Cleanup] end AppWrapper %s Version=%s Status=%+v\n", appwrapper.Name, appwrapper.ResourceVersion, appwrapper.Status)

	return nil
}
