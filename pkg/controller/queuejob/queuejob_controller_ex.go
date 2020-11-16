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

package queuejob

import (
	"fmt"
	"github.com/IBM/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	"github.com/golang/glog"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/metrics/adapter"
	"math"
	"math/rand"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"strconv"
	"time"

	"k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/genericresource"
	resconfigmap "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/configmap" // ConfigMap
	resdeployment "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/deployment"
	resnamespace "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/namespace"                         // NP
	resnetworkpolicy "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/networkpolicy"                 // NetworkPolicy
	respersistentvolume "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/persistentvolume"           // PV
	respersistentvolumeclaim "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/persistentvolumeclaim" // PVC
	respod "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/pod"
	ressecret "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/secret" // Secret
	resservice "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/service"
	resstatefulset "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources/statefulset"
	"k8s.io/apimachinery/pkg/labels"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned/clients"
	arbinformers "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion"
	informersv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion/v1"
	listersv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"

	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobdispatch"

	clusterstatecache "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/cache"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
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
	config           *rest.Config
	serverOption     *options.ServerOption

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
	agentMap map[string]*queuejobdispatch.JobClusterAgent
	agentList []string

	// Map for AppWrapper -> JobClusterAgent
	dispatchMap map[string]string

	// Metrics API Server
	metricsAdapter *adapter.MetricsAdpater

	// EventQueueforAgent
	agentEventQueue *cache.FIFO
}

type JobAndClusterAgent struct{
	queueJobKey string
	queueJobAgentKey string
}

func NewJobAndClusterAgent(qjKey string, qaKey string) *JobAndClusterAgent {
	return &JobAndClusterAgent{
		queueJobKey: qjKey,
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
		config:			config,
		serverOption:		serverOption,
		clients:		kubernetes.NewForConfigOrDie(config),
		arbclients:  		clientset.NewForConfigOrDie(config),
		eventQueue:  		cache.NewFIFO(GetQueueJobKey),
		agentEventQueue:	cache.NewFIFO(GetQueueJobKey),
		initQueue: 		cache.NewFIFO(GetQueueJobKey),
		updateQueue:		cache.NewFIFO(GetQueueJobKey),
		qjqueue:		NewSchedulingQueue(),
		cache: 			clusterstatecache.New(config),
	}
	cc.metricsAdapter =  adapter.New(config, cc.cache)

	cc.genericresources = genericresource.NewAppWrapperGenericResource(config)

	cc.qjobResControls = map[arbv1.ResourceType]queuejobresources.Interface{}
	RegisterAllQueueJobResourceTypes(&cc.qjobRegisteredResources)

	//initialize pod sub-resource control
	resControlPod, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePod, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type Pod not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypePod] = resControlPod

	// initialize service sub-resource control
	resControlService, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeService, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type Service not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeService] = resControlService

	// initialize PV sub-resource control
	resControlPersistentVolume, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePersistentVolume, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type PersistentVolume not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypePersistentVolume] = resControlPersistentVolume

	resControlPersistentVolumeClaim, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypePersistentVolumeClaim, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type PersistentVolumeClaim not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypePersistentVolumeClaim] = resControlPersistentVolumeClaim

	resControlNamespace, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeNamespace, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type Namespace not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeNamespace] = resControlNamespace

	resControlConfigMap, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeConfigMap, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type ConfigMap not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeConfigMap] = resControlConfigMap

	resControlSecret, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeSecret, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type Secret not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeSecret] = resControlSecret

	resControlNetworkPolicy, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeNetworkPolicy, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type NetworkPolicy not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeNetworkPolicy] = resControlNetworkPolicy



	// initialize deployment sub-resource control
	resControlDeployment, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeDeployment, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type Service not found")
		return nil
	}
	cc.qjobResControls[arbv1.ResourceTypeDeployment] = resControlDeployment

	// initialize SS sub-resource
	resControlSS, found, err := cc.qjobRegisteredResources.InitQueueJobResource(arbv1.ResourceTypeStatefulSet, config)
	if err != nil {
		glog.Errorf("fail to create queuejob resource control")
		return nil
	}
	if !found {
		glog.Errorf("queuejob resource type StatefulSet not found")
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
					glog.V(10).Infof("[Informer] Filter Name=%s Version=%s Local=%t FilterIgnore=%t Sender=%s &qj=%p qj=%+v", t.Name, t.ResourceVersion, t.Status.Local, t.Status.FilterIgnore, t.Status.Sender, t, t)
					// todo: This is a current workaround for duplicate message bug.
					if t.Status.Local == true { // ignore duplicate message from cache
						return false
					}
					t.Status.Local = true // another copy of this will be recognized as duplicate
					return !t.Status.FilterIgnore  // ignore update messages
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

	// Set dispatcher mode or agent mode
	cc.isDispatcher=serverOption.Dispatcher
	if cc.isDispatcher {
		glog.Infof("[Controller] Dispatcher mode")
 	}	else {
		glog.Infof("[Controller] Agent mode")
	}

	//create agents and agentMap
	cc.agentMap=map[string]*queuejobdispatch.JobClusterAgent{}
	cc.agentList=[]string{}
	for _, agentconfig := range strings.Split(serverOption.AgentConfigs,",") {
		agentData := strings.Split(agentconfig,":")
		cc.agentMap["/root/kubernetes/" + agentData[0]]=queuejobdispatch.NewJobClusterAgent(agentconfig, cc.agentEventQueue)
		cc.agentList=append(cc.agentList, "/root/kubernetes/" + agentData[0])
	}

	if cc.isDispatcher && len(cc.agentMap)==0 {
		glog.Errorf("Dispatcher mode: no agent information")
		return nil
	}

	//create (empty) dispatchMap
	cc.dispatchMap=map[string]string{}

	return cc
}

func (qjm *XController) PreemptQueueJobs() {
	qjobs := qjm.GetQueueJobsEligibleForPreemption()
	for _, q := range qjobs {
		newjob, e := qjm.queueJobLister.AppWrappers(q.Namespace).Get(q.Name)
		if e != nil {
			continue
		}
		newjob.Status.CanRun = false

		message := fmt.Sprintf("Insufficient number of Running pods, minimum=%d, running=%v.", q.Spec.SchedSpec.MinAvailable, q.Status.Running)
		cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondPreemptCandidate, v1.ConditionTrue, "MinPodsNotRunning", message)
		newjob.Status.Conditions = append(newjob.Status.Conditions, cond)

		if _, err := qjm.arbclients.ArbV1().AppWrappers(q.Namespace).Update(newjob); err != nil {
			glog.Errorf("Failed to update status of AppWrapper %v/%v: %v",
				q.Namespace, q.Name, err)
		}
	}
}

func (qjm *XController) GetQueueJobsEligibleForPreemption() []*arbv1.AppWrapper {
	qjobs := make([]*arbv1.AppWrapper, 0)

	queueJobs, err := qjm.queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		glog.Errorf("List of queueJobs %+v", qjobs)
		return qjobs
	}

	if !qjm.isDispatcher {		// Agent Mode
		for _, value := range queueJobs {
			replicas := value.Spec.SchedSpec.MinAvailable

			if int(value.Status.Succeeded) == replicas {
				if (replicas>0) {
					qjm.arbclients.ArbV1().AppWrappers(value.Namespace).Delete(value.Name, &metav1.DeleteOptions{})
					continue
				}
			}

			// Skip if AW Pending or just entering the system and does not have a state yet.
			if (value.Status.State == arbv1.AppWrapperStateEnqueued) || (value.Status.State == ""){
				continue
			}

			if int(value.Status.Running) < replicas {

				//Check to see if if this AW job has been dispatched for a time window before preempting
				conditionsLen := len(value.Status.Conditions)
				var dispatchConditionExists bool
				dispatchConditionExists = false
				var condition arbv1.AppWrapperCondition
				// Get the last time the AppWrapper was dispatched
				for i := (conditionsLen - 1); i > 0; i-- {
					condition = value.Status.Conditions[i]
					if (condition.Type != arbv1.AppWrapperCondDispatched) {
						continue
					}
					dispatchConditionExists = true
					break
				}

				// Now check for 0 running pods and for the minimum age and then
				// skip preempt if current time is not beyond minimum age
				minAge := condition.LastTransitionMicroTime.Add(60 * time.Second)
				if (value.Status.Running <= 0) && (dispatchConditionExists && (time.Now().Before(minAge))) {
					continue
				}

				if (replicas > 0) {
					glog.V(3).Infof("AppWrapper %s is eligible for preemption %v - %v , %v !!! \n", value.Name, value.Status.Running, replicas, value.Status.Succeeded)
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

func (qjm *XController) GetAggregatedResources(cqj *arbv1.AppWrapper) *clusterstateapi.Resource {
	//todo: deprecate resource controllers
	allocated := clusterstateapi.EmptyResource()
        for _, resctrl := range qjm.qjobResControls {
                qjv     := resctrl.GetAggregatedResources(cqj)
                allocated = allocated.Add(qjv)
        }

	for _, genericItem := range cqj.Spec.AggrResources.GenericItems {
		qjv, _ := genericresource.GetResources(&genericItem)
		allocated = allocated.Add(qjv)
	}

        return allocated
}


func (qjm *XController) getAggregatedAvailableResourcesPriority(targetpr float64, cqj string) *clusterstateapi.Resource {
	r := qjm.cache.GetUnallocatedResources()
	preemptable := clusterstateapi.EmptyResource()
	// Resources that can fit but have not dispatched.
	pending := clusterstateapi.EmptyResource()

	glog.V(3).Infof("[getAggAvaiResPri] Idle cluster resources %+v", r)

	queueJobs, err := qjm.queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		glog.Errorf("[getAggAvaiResPri] Unable to obtain the list of queueJobs %+v", err)
		return r
	}

	for _, value := range queueJobs {
		if value.Name == cqj {
			glog.V(11).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since it is the job being processed.", time.Now().String(), value.Name)
			continue
		} else if !value.Status.CanRun {
			glog.V(11).Infof("[getAggAvaiResPri] %s: Skipping adjustments for %s since it can not run.", time.Now().String(), value.Name)
			continue
		} else if value.Status.SystemPriority < targetpr {
			for _, resctrl := range qjm.qjobResControls {
				qjv := resctrl.GetAggregatedResources(value)
				preemptable = preemptable.Add(qjv)
			}
			for _, genericItem := range value.Spec.AggrResources.GenericItems {
				qjv, _ := genericresource.GetResources(&genericItem)
				preemptable = preemptable.Add(qjv)
			}

			continue
		} else if value.Status.State == arbv1.AppWrapperStateEnqueued {
			// Don't count the resources that can run but not yet realized (job orchestration pending or partially running).
			for _, resctrl := range qjm.qjobResControls {
				qjv := resctrl.GetAggregatedResources(value)
				pending = pending.Add(qjv)
				glog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v but state is still pending.", qjv, resctrl, value.Name, value.Status.CanRun)
			}
			for _, genericItem := range value.Spec.AggrResources.GenericItems {
				qjv, _ := genericresource.GetResources(&genericItem)
				pending = pending.Add(qjv)
				glog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v but state is still pending.", qjv, genericItem, value.Name, value.Status.CanRun)
			}
			continue
		} else if value.Status.State == arbv1.AppWrapperStateActive {
			if value.Status.Pending > 0 {
				//Don't count partially running jobs with pods still pending.
				for _, resctrl := range qjm.qjobResControls {
					qjv := resctrl.GetAggregatedResources(value)
					pending = pending.Add(qjv)
					glog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but %v pod(s) are pending.", qjv, resctrl, value.Name, value.Status.CanRun, value.Status.State, value.Status.Pending)
				}
				for _, genericItem := range value.Spec.AggrResources.GenericItems {
					qjv, _ := genericresource.GetResources(&genericItem)
					pending = pending.Add(qjv)
					glog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but %v pod(s) are pending.", qjv, genericItem, value.Name, value.Status.CanRun, value.Status.State, value.Status.Pending)
				}
			} else {
				// TODO: Hack to handle race condition when Running jobs have not yet updated the pod counts
				// This hack uses the golang struct implied behavior of defining the object without a value.  In this case
				// of using 'int32' novalue and value of 0 are the same.
				if value.Status.Pending == 0 && value.Status.Running == 0 && value.Status.Succeeded == 0 && value.Status.Failed == 0 {
					for _, resctrl := range qjm.qjobResControls {
						qjv := resctrl.GetAggregatedResources(value)
						pending = pending.Add(qjv)
						glog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but no pod counts in the state have been defined.", qjv, resctrl, value.Name, value.Status.CanRun, value.Status.State)
					}
					for _, genericItem := range value.Spec.AggrResources.GenericItems {
						qjv, _ := genericresource.GetResources(&genericItem)
						pending = pending.Add(qjv)
						glog.V(10).Infof("[getAggAvaiResPri] Subtract all resources %+v in resctrlType=%T for job %s which can-run is set to: %v and status set to: %s but no pod counts in the state have been defined.", qjv, genericItem, value.Name, value.Status.CanRun, value.Status.State)
					}
				}
			}
			continue
		} else {
			//Do nothing
		}
	}

	glog.V(6).Infof("[getAggAvaiResPri] Schedulable idle cluster resources: %+v, subtracting dispatched resources: %+v and adding preemptable cluster resources: %+v", r, pending, preemptable)

	r = r.Add(preemptable)
	r = r.NonNegSub(pending)

	glog.V(3).Infof("[getAggAvaiResPri] %+v available resources to schedule", r)
	return r
}

func (qjm *XController) chooseAgent(qj *arbv1.AppWrapper) string{

	qjAggrResources := qjm.GetAggregatedResources(qj)

	glog.V(2).Infof("[Controller: Dispatcher Mode] Aggr Resources of XQJ %s: %v\n", qj.Name, qjAggrResources)

	agentId := qjm.agentList[rand.Int() % len(qjm.agentList)]
	glog.V(2).Infof("[Controller: Dispatcher Mode] Agent %s is chosen randomly\n", agentId)
	resources := qjm.agentMap[agentId].AggrResources
	glog.V(2).Infof("[Controller: Dispatcher Mode] Aggr Resources of Agent %s: %v\n", agentId, resources)
	if qjAggrResources.LessEqual(resources) {
		glog.V(2).Infof("[Controller: Dispatcher Mode] Agent %s has enough resources\n", agentId)
	return agentId
	}
	glog.V(2).Infof("[Controller: Dispatcher Mode] Agent %s does not have enough resources\n", agentId)

	return ""
}


// Thread to find queue-job(QJ) for next schedule
func (qjm *XController) ScheduleNext() {
	// get next QJ from the queue
	// check if we have enough compute resources for it
	// if we have enough compute resources then we set the AllocatedReplicas to the total
	// amount of resources asked by the job
	qj, err := qjm.qjqueue.Pop()
	if err != nil {
		glog.V(3).Infof("[ScheduleNext] Cannot pop QueueJob from qjqueue! err=%#v", err)
		return // Try to pop qjqueue again
	} else {
		glog.V(3).Infof("[ScheduleNext] activeQ.Pop %s *Delay=%.6f seconds RemainingLength=%d &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.Length(), qj, qj.ResourceVersion, qj.Status)
	}

	apiCacheAWJob, e := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
	// apiQueueJob's ControllerFirstTimestamp is only microsecond level instead of nanosecond level
	if e != nil {
		glog.Errorf("[ScheduleNext] Unable to get AW %s from API cache &aw=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
		return
	}
	// make sure qj has the latest information
	if larger(apiCacheAWJob.ResourceVersion, qj.ResourceVersion) {
		glog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache          &qj=%p          qj=%+v", qj.Name, qj, qj)
		glog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiCacheAWJob.Name, apiCacheAWJob, apiCacheAWJob)
		apiCacheAWJob.DeepCopyInto(qj)
	}

	// Re-compute SystemPriority for DynamicPriority policy
	if qjm.serverOption.DynamicPriority {
		//  Create newHeap to temporarily store qjqueue jobs for updating SystemPriority
		tempQ := newHeap(cache.MetaNamespaceKeyFunc, HigherSystemPriorityQJ)
		qj.Status.SystemPriority = qj.Spec.Priority + qj.Spec.PrioritySlope * (time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time)).Seconds()
		tempQ.Add(qj)
		for qjm.qjqueue.Length() > 0 {
			qjtemp, _ := qjm.qjqueue.Pop()
			qjtemp.Status.SystemPriority = qjtemp.Spec.Priority + qjtemp.Spec.PrioritySlope * (time.Now().Sub(qjtemp.Status.ControllerFirstTimestamp.Time)).Seconds()
			tempQ.Add(qjtemp)
		}
		// move AppWrappers back to activeQ and sort based on SystemPriority
		for tempQ.data.Len() > 0 {
			qjtemp, _ := tempQ.Pop()
			qjm.qjqueue.AddIfNotPresent(qjtemp.(*arbv1.AppWrapper))
		}
		// Print qjqueue.ativeQ for debugging
		if glog.V(10) {
			pq := qjm.qjqueue.(*PriorityQueue)
			if qjm.qjqueue.Length() > 0 {
				for key, element := range pq.activeQ.data.items {
					qjtemp := element.obj.(*arbv1.AppWrapper)
					glog.V(10).Infof("[ScheduleNext] AfterCalc: qjqLength=%d Key=%s index=%d Priority=%.1f SystemPriority=%.1f QueueJobState=%s",
						qjm.qjqueue.Length(), key, element.index, qjtemp.Spec.Priority, qjtemp.Status.SystemPriority, qjtemp.Status.QueueJobState)
				}
			}
		}

		// Retrive HeadOfLine after priority update
		qj, err = qjm.qjqueue.Pop()
		if err != nil {
			glog.V(3).Infof("[ScheduleNext] Cannot pop QueueJob from qjqueue! err=%#v", err)
		} else {
			glog.V(3).Infof("[ScheduleNext] activeQ.Pop_afterPriorityUpdate %s *Delay=%.6f seconds RemainingLength=%d &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.Length(), qj, qj.ResourceVersion, qj.Status)
		}
	}

	qj.Status.QueueJobState = arbv1.AppWrapperCondHeadOfLine
	cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondHeadOfLine, v1.ConditionTrue, "FrontOfQueue.", "")
	qj.Status.Conditions = append(qj.Status.Conditions, cond)

	qj.Status.FilterIgnore = true   // update QueueJobState only
	qjm.updateEtcd(qj, "ScheduleNext - setHOL")
	qjm.qjqueue.AddUnschedulableIfNotPresent(qj)  // working on qj, avoid other threads putting it back to activeQ
	glog.V(10).Infof("[ScheduleNext] after Pop qjqLength=%d qj %s Version=%s activeQ=%t Unsched=%t Status=%+v", qjm.qjqueue.Length(), qj.Name, qj.ResourceVersion, qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj.Status)
	if(qjm.isDispatcher) {
		glog.V(2).Infof("[ScheduleNext] [Dispatcher Mode] Dispatch Next QueueJob: %s\n", qj.Name)
	}else{
		glog.V(2).Infof("[ScheduleNext] [Agent Mode] Deploy Next QueueJob: %s Status=%+v\n", qj.Name, qj.Status)
	}

	if qj.Status.CanRun {
		return
	}

	dispatchFailedReason := "AppWrapperNotRunnable."
	dispatchFailedMessage := ""
	if qjm.isDispatcher {			// Dispatcher Mode
		agentId:=qjm.chooseAgent(qj)
		if agentId != "" {			// A proper agent is found.
														// Update states (CanRun=True) of XQJ in API Server
														// Add XQJ -> Agent Map
			apiQueueJob, err := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
			if err != nil {
				return
			}
			apiQueueJob.Status.CanRun = true
			// qj.Status.CanRun = true
			glog.V(10).Infof("[TTime] %s, %s: ScheduleNextBeforeEtcd", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			if _, err := qjm.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(apiQueueJob); err != nil {
				glog.Errorf("Failed to update status of AppWrapper %v/%v: %v",
																	qj.Namespace, qj.Name, err)
			}
			queueJobKey,_:=GetQueueJobKey(qj)
			qjm.dispatchMap[queueJobKey]=agentId
			glog.V(10).Infof("[TTime] %s, %s: ScheduleNextAfterEtcd", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			return
		} else {
			dispatchFailedMessage = "Cannot find an cluster with enough resources to dispatch AppWrapper."
			glog.V(2).Infof("[Controller: Dispatcher Mode] %s %s\n", dispatchFailedReason, dispatchFailedMessage)
			go qjm.backoff(qj, dispatchFailedReason, dispatchFailedMessage)
		}
	} else {						// Agent Mode
		aggqj := qjm.GetAggregatedResources(qj)

		// HeadOfLine logic
		HOLStartTime := time.Now()
		forwarded := false
		// Try to forward to eventQueue for at most HeadOfLineHoldingTime
		for !forwarded {
			priorityindex := qj.Status.SystemPriority
			// Support for Non-Preemption
			if !qjm.serverOption.Preemption     { priorityindex = -math.MaxFloat64 }
			// Disable Preemption under DynamicPriority.  Comment out if allow DynamicPriority and Preemption at the same time.
			if qjm.serverOption.DynamicPriority { priorityindex = -math.MaxFloat64 }
			resources := qjm.getAggregatedAvailableResourcesPriority(priorityindex, qj.Name)
			glog.V(2).Infof("[ScheduleNext] XQJ %s with resources %v to be scheduled on aggregated idle resources %v", qj.Name, aggqj, resources)

			if aggqj.LessEqual(resources) {
				// qj is ready to go!
				apiQueueJob, e := qjm.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
				// apiQueueJob's ControllerFirstTimestamp is only microsecond level instead of nanosecond level
				if e != nil {
					glog.Errorf("[ScheduleNext] Unable to get AW %s from API cache &aw=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
					return
				}
				// make sure qj has the latest information
				if larger(apiQueueJob.ResourceVersion, qj.ResourceVersion) {
					glog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache          &qj=%p          qj=%+v", qj.Name, qj, qj)
					glog.V(10).Infof("[ScheduleNext] %s found more recent copy from cache &apiQueueJob=%p apiQueueJob=%+v", apiQueueJob.Name, apiQueueJob, apiQueueJob)
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
					if err = qjm.enqueue(qj); err != nil { // unsuccessful add to eventQueue, add back to activeQ
						glog.Errorf("[ScheduleNext] Fail to add %s to eventQueue, activeQ.Add_toSchedulingQueue &qj=%p Version=%s Status=%+v err=%#v", qj.Name, qj, qj.ResourceVersion, qj.Status, err)
						qjm.qjqueue.MoveToActiveQueueIfExists(qj)
					} else { // successful add to eventQueue, remove from qjqueue
						qjm.qjqueue.Delete(qj)
						forwarded = true
						glog.V(3).Infof("[ScheduleNext] %s Delay=%.6f seconds eventQueue.Add_afterHeadOfLine activeQ=%t, Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
					}
				}
			} else { // Not enough free resources to dispatch HOL
				dispatchFailedMessage = "Insufficient resources to dispatch AppWrapper."
				glog.V(3).Infof("[ScheduleNext] HOL Blocking by %s for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			}
			// stop trying to dispatch after HeadOfLineHoldingTime
			if (forwarded || time.Now().After(HOLStartTime.Add(time.Duration(qjm.serverOption.HeadOfLineHoldingTime)*time.Second))) {
				break
			} else { // Try to dispatch again after one second
				time.Sleep(time.Second * 1)
			}
		}
		if !forwarded { // start thread to backoff
			glog.V(3).Infof("[ScheduleNext] HOL backoff %s after waiting for %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(HOLStartTime), qjm.qjqueue.IfExistActiveQ(qj), qjm.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			go qjm.backoff(qj, dispatchFailedReason, dispatchFailedMessage)
		}
	}
}

// Update AppWrappers in etcd
// todo: This is a current workaround for duplicate message bug.
func (cc *XController) updateEtcd(qj *arbv1.AppWrapper, at string) error {
	qj.Status.Sender = "before "+ at  // set Sender string to indicate code location
	qj.Status.Local  = false          // for Informer FilterFunc to pickup
	if qjj, err := cc.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(qj); err != nil {
		glog.Errorf("[updateEtcd] Failed to update status of AppWrapper %s, namespace: %s at %s err=%v", qj.Name, qj.Namespace, at, err)
		return err
	} else {  // qjj should be the same as qj except with newer ResourceVersion
		qj.ResourceVersion = qjj.ResourceVersion  // update new ResourceVersion from etcd
	}
	glog.V(10).Infof("[updateEtcd] AppWrapperUpdate success %s at %s &qj=%p qj=%+v", qj.Name, at, qj, qj)
	qj.Status.Local  = true           // for Informer FilterFunc to ignore duplicate
	qj.Status.Sender = "after  "+ at  // set Sender string to indicate code location
	return nil
}

func (qjm *XController) backoff(q *arbv1.AppWrapper, reason string, message string) {
	var workingAW *arbv1.AppWrapper
	apiCacheAWJob, e := qjm.queueJobLister.AppWrappers(q.Namespace).Get(q.Name)
	// Update condition
	if (e == nil) {
		workingAW = apiCacheAWJob
		apiCacheAWJob.Status.QueueJobState = arbv1.AppWrapperCondBackoff
		cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondBackoff, v1.ConditionTrue, reason, message)
		workingAW.Status.Conditions = append(workingAW.Status.Conditions, cond)
		workingAW.Status.FilterIgnore = true // update QueueJobState only, no work needed
		qjm.updateEtcd(workingAW, "backoff - Rejoining")
	} else {
		workingAW = q
		glog.Errorf("[backoff] Failed to retrieve cached object for %s/%s.  Continuing with possible stale object without updating conditions.", workingAW.Namespace,workingAW.Name)
	}
	qjm.qjqueue.AddUnschedulableIfNotPresent(workingAW)
	glog.V(3).Infof("[backoff] %s move to unschedulableQ before sleep for %d seconds. activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", workingAW.Name,
		qjm.serverOption.BackoffTime, qjm.qjqueue.IfExistActiveQ((workingAW)), qjm.qjqueue.IfExistUnschedulableQ((workingAW)), workingAW, workingAW.ResourceVersion, workingAW.Status)
 	time.Sleep(time.Duration(qjm.serverOption.BackoffTime) * time.Second)
	qjm.qjqueue.MoveToActiveQueueIfExists(workingAW)

	// Update condition after backoff
	apiCacheAWJob, e = qjm.queueJobLister.AppWrappers(q.Namespace).Get(q.Name)
	if (e == nil) {
		workingAW = apiCacheAWJob
		workingAW.Status.QueueJobState = arbv1.AppWrapperCondQueueing
		returnCond := GenerateAppWrapperCondition(arbv1.AppWrapperCondQueueing, v1.ConditionTrue, "BackoffTimerExpired.", "")
		workingAW.Status.Conditions = append(workingAW.Status.Conditions, returnCond)
		workingAW.Status.FilterIgnore = true  // update QueueJobState only, no work needed
		qjm.updateEtcd(workingAW, "backoff - Queueing")
	}
	glog.V(3).Infof("[backoff] %s activeQ.Add after sleep for %d seconds. activeQ=%t Unsched=%t &aw=%p Version=%s Status=%+v", workingAW.Name,
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
		go wait.Until(cc.UpdateAgent, 2*time.Second, stopCh)			// In the Agent?
		for _, jobClusterAgent := range cc.agentMap {
			go jobClusterAgent.Run(stopCh)
		}
		go wait.Until(cc.agentEventQueueWorker, time.Second, stopCh)		// Update Agent Worker
	}

	// go wait.Until(cc.worker, time.Second, stopCh)
	go wait.Until(cc.worker, 0, stopCh)
}

func (qjm *XController) UpdateAgent() {
	glog.V(3).Infof("[Controller] Update AggrResources for All Agents\n")
	for _, jobClusterAgent := range qjm.agentMap {
		jobClusterAgent.UpdateAggrResources()
	}
}

func (qjm *XController) UpdateQueueJobs() {
	firstTime := metav1.NowMicro()
	// retrive queueJobs from local cache.  no guarantee queueJobs contain up-to-date information
	queueJobs, err := qjm.queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		glog.Errorf("[UpdateQueueJobs] List of queueJobs err=%+v", err)
		return
	}
	for _, newjob := range queueJobs {
		// UpdateQueueJobs can be the first to see a new AppWrapper job, under heavy load
		if newjob.Status.QueueJobState == "" {
			newjob.Status.ControllerFirstTimestamp = firstTime
			newjob.Status.SystemPriority = newjob.Spec.Priority
			newjob.Status.QueueJobState = arbv1.AppWrapperCondInit
			newjob.Status.Conditions =  []arbv1.AppWrapperCondition{
				arbv1.AppWrapperCondition{
					Type:    arbv1.AppWrapperCondInit,
					Status:  v1.ConditionTrue,
					LastUpdateMicroTime:  metav1.NowMicro(),
					LastTransitionMicroTime: metav1.NowMicro(),
				},
			}
			glog.V(3).Infof("[UpdateQueueJobs] %s 0Delay=%.6f seconds CreationTimestamp=%s ControllerFirstTimestamp=%s",
				newjob.Name, time.Now().Sub(newjob.Status.ControllerFirstTimestamp.Time).Seconds(), newjob.CreationTimestamp, newjob.Status.ControllerFirstTimestamp)
		}
		glog.V(10).Infof("[UpdateQueueJobs] %s: qjqueue=%t &qj=%p Version=%s Status=%+v", newjob.Name, qjm.qjqueue.IfExist(newjob), newjob, newjob.ResourceVersion, newjob.Status)
		// check eventQueue, qjqueue in program sequence to make sure job is not in qjqueue
		if _, exists, _ := qjm.eventQueue.Get(newjob); exists { continue } // do not enqueue if already in eventQueue
		if qjm.qjqueue.IfExist(newjob) { continue } // do not enqueue if already in qjqueue

		err = qjm.enqueueIfNotPresent(newjob)
		if err != nil {
			glog.Errorf("[UpdateQueueJobs] Fail to enqueue %s to eventQueue, ignore.  *Delay=%.6f seconds &qj=%p Version=%s Status=%+v err=%#v", newjob.Name, time.Now().Sub(newjob.Status.ControllerFirstTimestamp.Time).Seconds(), newjob, newjob.ResourceVersion, newjob.Status, err)
		} else {
			glog.V(4).Infof("[UpdateQueueJobs] %s *Delay=%.6f seconds eventQueue.Add_byUpdateQueueJobs &qj=%p Version=%s Status=%+v", newjob.Name, time.Now().Sub(newjob.Status.ControllerFirstTimestamp.Time).Seconds(), newjob, newjob.ResourceVersion, newjob.Status)
		}
  	}
}

func (cc *XController) addQueueJob(obj interface{}) {
	firstTime := metav1.NowMicro()
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		glog.Errorf("[Informer-addQJ] object is not AppWrapper. object=%+v", obj)
		return
	}
	glog.V(6).Infof("[Informer-addQJ] %s &qj=%p  qj=%+v", qj.Name, qj, qj)
	if qj.Status.QueueJobState == "" {
		qj.Status.ControllerFirstTimestamp = firstTime
		qj.Status.SystemPriority = qj.Spec.Priority
		qj.Status.QueueJobState  = arbv1.AppWrapperCondInit
		qj.Status.Conditions =  []arbv1.AppWrapperCondition{
			arbv1.AppWrapperCondition{
				Type:    arbv1.AppWrapperCondInit,
				Status:  v1.ConditionTrue,
				LastUpdateMicroTime:  metav1.NowMicro(),
				LastTransitionMicroTime: metav1.NowMicro(),
			},
		}
	} else {
		glog.Warningf("[Informer-addQJ] Received and add by the informer for AppWrapper job %s which already has been seen and initialized current state %s with timestamp: %s, elapsed time of %.6f",
						qj.Name, qj.Status.State, qj.Status.ControllerFirstTimestamp, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds())
	}

	glog.V(10).Infof("[Informer-addQJ] %s Delay=%.6f seconds CreationTimestamp=%s ControllerFirstTimestamp=%s",
		qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.CreationTimestamp, qj.Status.ControllerFirstTimestamp)

	glog.V(4).Infof("[Informer-addQJ] enqueue %s &qj=%p Version=%s Status=%+v", qj.Name, qj, qj.ResourceVersion, qj.Status)
	cc.enqueue(qj)
}

func (cc *XController) updateQueueJob(oldObj, newObj interface{}) {
	newQJ, ok := newObj.(*arbv1.AppWrapper)
	if !ok {
		glog.Errorf("[Informer-updateQJ] new object is not AppWrapper. object=%+v", newObj)
		return
	}
	oldQJ, ok := oldObj.(*arbv1.AppWrapper)
	if !ok {
		glog.Errorf("[Informer-updateQJ] old object is not AppWrapper.  enqueue(newQJ).  oldObj=%+v", oldObj)
		glog.V(4).Infof("[Informer-updateQJ] %s *Delay=%.6f seconds BadOldObject enqueue &newQJ=%p Version=%s Status=%+v", newQJ.Name, time.Now().Sub(newQJ.Status.ControllerFirstTimestamp.Time).Seconds(), newQJ, newQJ.ResourceVersion, newQJ.Status)
		cc.enqueue(newQJ)
		return
	}
	// AppWrappers may come out of order.  Ignore old ones.
	if (oldQJ.Name == newQJ.Name) && (larger(oldQJ.ResourceVersion, newQJ.ResourceVersion)) {
		glog.V(10).Infof("[Informer-updateQJ]  %s ignored OutOfOrder arrival &oldQJ=%p oldQJ=%+v", oldQJ.Name, oldQJ, oldQJ)
		glog.V(10).Infof("[Informer-updateQJ] %s ignored OutOfOrder arrival &newQJ=%p newQJ=%+v", newQJ.Name, newQJ, newQJ)
		return
	}
	glog.V(3).Infof("[Informer-updateQJ] %s *Delay=%.6f seconds normal enqueue &newQJ=%p Version=%s Status=%+v", newQJ.Name, time.Now().Sub(newQJ.Status.ControllerFirstTimestamp.Time).Seconds(), newQJ, newQJ.ResourceVersion, newQJ.Status)
	cc.enqueue(newQJ)
}

// a, b arbitrary length numerical string.  returns true if a larger than b
func larger (a, b string) bool {
	if len(a) > len(b) { return true  } // Longer string is larger
	if len(a) < len(b) { return false } // Longer string is larger
	return a > b // Equal length, lexicographic order
}

func (cc *XController) deleteQueueJob(obj interface{}) {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		glog.Errorf("[Informer-deleteQJ] obj is not AppWrapper. obj=%+v", obj)
		return
	}
	glog.V(3).Infof("[Informer-deleteQJ] %s *Delay=%.6f seconds before enqueue &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj, qj.ResourceVersion, qj.Status)
	cc.enqueue(qj)
}

func (cc *XController) enqueue(obj interface{}) error {
	qj, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		return fmt.Errorf("[enqueue] obj is not AppWrapper. obj=%+v", obj)
	}

	err := cc.eventQueue.Add(qj)  // add to FIFO queue if not in, update object & keep position if already in FIFO queue
	if err != nil {
		glog.Errorf("[enqueue] Fail to enqueue %s to eventQueue, ignore.  *Delay=%.6f seconds &qj=%p Version=%s Status=%+v err=%#v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj, qj.ResourceVersion, qj.Status, err)
	} else {
		glog.V(10).Infof("[enqueue] %s *Delay=%.6f seconds eventQueue.Add_byEnqueue &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj, qj.ResourceVersion, qj.Status)
	}
	return err
}

func (cc *XController) enqueueIfNotPresent(obj interface{}) error {
	aw, ok := obj.(*arbv1.AppWrapper)
	if !ok {
		return fmt.Errorf("[enqueueIfNotPresent] obj is not AppWrapper. obj=%+v", obj)
	}

	err := cc.eventQueue.AddIfNotPresent(aw)  // add to FIFO queue if not in, update object & keep position if already in FIFO queue
	return err
}

func (cc *XController) agentEventQueueWorker() {
	if _, err := cc.agentEventQueue.Pop(func(obj interface{}) error {
		var queuejob *arbv1.AppWrapper
		switch v := obj.(type) {
		case *arbv1.AppWrapper:
			queuejob = v
		default:
			glog.Errorf("Un-supported type of %v", obj)
			return nil
		}

		if queuejob == nil {
			if acc, err := meta.Accessor(obj); err != nil {
				glog.Warningf("Failed to get AppWrapper for %v/%v", acc.GetNamespace(), acc.GetName())
			}

			return nil
		}
		glog.V(3).Infof("[Controller: Dispatcher Mode] XQJ Status Update from AGENT: Name:%s, Status: %+v\n", queuejob.Name, queuejob.Status)


		// sync AppWrapper
		if err := cc.updateQueueJobStatus(queuejob); err != nil {
			glog.Errorf("Failed to sync AppWrapper %s, err %#v", queuejob.Name, err)
			// If any error, requeue it.
			return err
		}

		return nil
	}); err != nil {
		glog.Errorf("Fail to pop item from updateQueue, err %#v", err)
		return
	}
}

func (cc *XController) updateQueueJobStatus(queueJobFromAgent *arbv1.AppWrapper) error {
	queueJobInEtcd, err := cc.queueJobLister.AppWrappers(queueJobFromAgent.Namespace).Get(queueJobFromAgent.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if (cc.isDispatcher) {
				cc.Cleanup(queueJobFromAgent)
				cc.qjqueue.Delete(queueJobFromAgent)
			}
			return nil
		}
		return err
	}
	if(len(queueJobFromAgent.Status.State)==0 || queueJobInEtcd.Status.State == queueJobFromAgent.Status.State) {
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
	if _, err := cc.eventQueue.Pop(func(obj interface{}) error {
		var queuejob *arbv1.AppWrapper
		switch v := obj.(type) {
		case *arbv1.AppWrapper:
			queuejob = v
		default:
			glog.Errorf("[worker] eventQueue.Pop un-supported type. obj=%+v", obj)
			return nil
		}
		glog.V(10).Infof("[worker] %s *Delay=%.6f seconds eventQueue.Pop_begin &newQJ=%p Version=%s Status=%+v", queuejob.Name, time.Now().Sub(queuejob.Status.ControllerFirstTimestamp.Time).Seconds(), queuejob, queuejob.ResourceVersion, queuejob.Status)

		if queuejob == nil {
			if acc, err := meta.Accessor(obj); err != nil {
				glog.Warningf("[worker] Failed to get AppWrapper for %v/%v", acc.GetNamespace(), acc.GetName())
			}

			return nil
		}

		// sync AppWrapper
		if err := cc.syncQueueJob(queuejob); err != nil {
			glog.Errorf("[worker] Failed to sync AppWrapper %s, err %#v", queuejob.Name, err)
			// If any error, requeue it.
			return err
		}

		glog.V(10).Infof("[worker] Ending %s Delay=%.6f seconds &newQJ=%p Version=%s Status=%+v", queuejob.Name, time.Now().Sub(queuejob.Status.ControllerFirstTimestamp.Time).Seconds(), queuejob, queuejob.ResourceVersion, queuejob.Status)
		return nil
	}); err != nil {
		glog.Errorf("[worker] Fail to pop item from eventQueue, err %#v", err)
		return
	}
}

func (cc *XController) syncQueueJob(qj *arbv1.AppWrapper) error {
	queueJob, err := cc.queueJobLister.AppWrappers(qj.Namespace).Get(qj.Name)
	// queueJob's ControllerFirstTimestamp is only microsecond level instead of nanosecond level
	if err != nil {
		if apierrors.IsNotFound(err) {
			if (cc.isDispatcher) {
				cc.Cleanup(qj)
				cc.qjqueue.Delete(qj)
			}
			return nil
		}
		return err
	}
	// make sure qj has the latest information
	if larger(queueJob.ResourceVersion, qj.ResourceVersion) {
		glog.V(10).Infof("[worker-syncQJ] %s found more recent copy from cache       &qj=%p       qj=%+v", qj.Name, qj, qj)
		glog.V(10).Infof("[worker-syncQJ] %s found more recent copy from cache &queueJob=%p queueJob=%+v", queueJob.Name, queueJob, queueJob)

		queueJob.DeepCopyInto(qj)
	}

	// If it is Agent (not a dispatcher), update pod information
	if(!cc.isDispatcher){
		// we call sync for each controller
		// update pods running, pending,...
		if (qj.Status.State == arbv1.AppWrapperStateActive) {
			cc.qjobResControls[arbv1.ResourceTypePod].UpdateQueueJobStatus(qj)

			// Update etcd conditions if AppWrapper Job has at least 1 running pod and transitioning from dispatched to running.
			if (qj.Status.QueueJobState != arbv1.AppWrapperCondRunning ) && (qj.Status.Running > 0) {
				qj.Status.QueueJobState = arbv1.AppWrapperCondRunning
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondRunning, v1.ConditionTrue, "PodsRunning", "")
				qj.Status.Conditions = append(qj.Status.Conditions, cond)
				qj.Status.FilterIgnore = true  // Update AppWrapperCondRunning
				cc.updateEtcd(qj, "[syncQueueJob] setRunning")
			}
		}
	}

	return cc.manageQueueJob(qj)
}

// manageQueueJob is the core method responsible for managing the number of running
// pods according to what is specified in the job.Spec.
// Does NOT modify <activePods>.
func (cc *XController) manageQueueJob(qj *arbv1.AppWrapper) error {
	var err error
	startTime := time.Now()
	defer func() {
		glog.V(10).Infof("[worker-manageQJ] Ending %s manageQJ time=%s &qj=%p Version=%s Status=%+v", qj.Name, time.Now().Sub(startTime), qj, qj.ResourceVersion, qj.Status)
	}()

	if(!cc.isDispatcher) { // Agent Mode

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

			// we delete the job from the queue if it is there
			cc.qjqueue.Delete(qj)

			return nil
			//var result arbv1.AppWrapper
			//return cc.arbclients.Put().
			//	Namespace(qj.Namespace).Resource(arbv1.QueueJobPlural).
			//	Name(qj.Name).Body(qj).Do().Into(&result)
		}

		// First execution of qj to set Status.State = Enqueued
		if !qj.Status.CanRun && (qj.Status.State != arbv1.AppWrapperStateEnqueued && qj.Status.State != arbv1.AppWrapperStateDeleted) {
			// if there are running resources for this job then delete them because the job was put in
			// pending state...
			glog.V(2).Infof("[Agent Mode] Deleting resources for XQJ %s because it will be preempted (newjob)\n", qj.Name)
			err = cc.Cleanup(qj)
			if err != nil {
				return err
			}

			qj.Status.State = arbv1.AppWrapperStateEnqueued
			//  add qj to qjqueue only when it is not in UnschedulableQ
			if cc.qjqueue.IfExistUnschedulableQ(qj) {
				glog.V(10).Infof("[worker-manageQJ] leaving %s to qjqueue.UnschedulableQ activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
			} else {
				glog.V(10).Infof("[worker-manageQJ] before add to activeQ %s activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v", qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				qj.Status.QueueJobState = arbv1.AppWrapperCondQueueing
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondQueueing, v1.ConditionTrue, "AwaitingHeadOfLine", "")
				qj.Status.Conditions = append(qj.Status.Conditions, cond)

				qj.Status.FilterIgnore = true // Update Queueing status, add to qjqueue for ScheduleNext
				cc.updateEtcd(qj, "[manageQueueJob]setQueueing")
				if err = cc.qjqueue.AddIfNotPresent(qj); err != nil {
					glog.Errorf("[worker-manageQJ] Fail to add %s to activeQueue. Back to eventQueue activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v err=%#v",
						qj.Name, cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status, err)
					cc.enqueue(qj)
				} else {
					glog.V(3).Infof("[worker-manageQJ] %s 1Delay=%.6f seconds activeQ.Add_success activeQ=%t Unsched=%t &qj=%p Version=%s Status=%+v",
						qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), cc.qjqueue.IfExistActiveQ(qj), cc.qjqueue.IfExistUnschedulableQ(qj), qj, qj.ResourceVersion, qj.Status)
				}
			}
			return nil
		} // End of first execution of qj to add to qjqueue for ScheduleNext

		// add qj to Etcd for dispatch
		if qj.Status.CanRun && qj.Status.State != arbv1.AppWrapperStateActive {
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
			glog.V(3).Infof("[worker-manageQJ] %s 3Delay=%.6f seconds BeforeDispatchingToEtcd Version=%s Status=%+v",
				qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.ResourceVersion, qj.Status)
			dispatched := true
			dispatchFailureReason := "ItemCreationFailure."
			dispatchFailureMessage := ""
			for _, ar := range qj.Spec.AggrResources.Items {
				glog.V(10).Infof("[worker-manageQJ] before dispatch [%v].SyncQueueJob %s &qj=%p Version=%s Status=%+v", ar.Type, qj.Name, qj, qj.ResourceVersion, qj.Status)
				// Call Resource Controller of ar.Type to issue REST call to Etcd for resource creation
				err00 := cc.qjobResControls[ar.Type].SyncQueueJob(qj, &ar)
				if err00 != nil {
					dispatchFailureMessage = fmt.Sprintf("%s/%s creation failure: %+v", qj.Namespace, qj.Name, err00)
					glog.V(3).Infof("[worker-manageQJ] Error dispatching job=%s type=%v Status=%+v err=%+v", qj.Name, ar.Type, qj.Status, err00)
					dispatched = false
					break
				}
			}
			// Handle generic resources
			for _, ar := range qj.Spec.AggrResources.GenericItems {
				glog.V(10).Infof("[worker-manageQJ] before dispatch Generic.SyncQueueJob %s &qj=%p Version=%s Status=%+v", qj.Name, qj, qj.ResourceVersion, qj.Status)
				_, err00 := cc.genericresources.SyncQueueJob(qj, &ar)
				if err00 != nil {
					dispatchFailureMessage = fmt.Sprintf("%s/%s creation failure: %+v", qj.Namespace, qj.Name, err00)
					glog.Errorf("[worker-manageQJ] Error dispatching job=%s Status=%+v err=%+v", qj.Name, qj.Status, err00)
					dispatched = false
				}
			}

			if dispatched { // set AppWrapperCondRunning if all resources are successfully dispatched
				qj.Status.QueueJobState = arbv1.AppWrapperCondDispatched
				cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondDispatched, v1.ConditionTrue, "AppWrapperRunnable", "")
				qj.Status.Conditions = append(qj.Status.Conditions, cond)

				glog.V(3).Infof("[worker-manageQJ] %s 4Delay=%.6f seconds AllResourceDispatchedToEtcd Version=%s Status=%+v",
					qj.Name, time.Now().Sub(qj.Status.ControllerFirstTimestamp.Time).Seconds(), qj.ResourceVersion, qj.Status)
			} else {
				qj.Status.State = arbv1.AppWrapperStateFailed
				qj.Status.QueueJobState = arbv1.AppWrapperCondFailed
				if ( !isLastConditionDuplicate(qj,arbv1.AppWrapperCondFailed, v1.ConditionTrue, dispatchFailureReason, dispatchFailureMessage) ) {
					cond := GenerateAppWrapperCondition(arbv1.AppWrapperCondFailed, v1.ConditionTrue, dispatchFailureReason, dispatchFailureMessage)
					qj.Status.Conditions = append(qj.Status.Conditions, cond)
				}
				cc.Cleanup(qj)
			}

			// TODO(k82cn): replaced it with `UpdateStatus`
			qj.Status.FilterIgnore = true  // update State & QueueJobState after dispatch
			if err := cc.updateEtcd(qj, "[manageQueueJob]afterEtcdDispatching"); err != nil {
				return err
			}
		} // Bugfix to eliminate performance problem of overloading the event queue.
		// Finish adding qj to Etcd for dispatch

	}	else { 				// Dispatcher Mode

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
			glog.V(3).Infof("Deleting queuejob resources because it will be preempted! %s", qj.Name)
			err = cc.Cleanup(qj)
			if err != nil {
				return err
			}

			qj.Status.State = arbv1.AppWrapperStateEnqueued
			_, err = cc.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(qj)
			if err != nil {
				return err
			}
			return nil
		}

		// if !qj.Status.CanRun && qj.Status.State == arbv1.QueueJobStateEnqueued {
		if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued {
			cc.qjqueue.AddIfNotPresent(qj)
			return nil
		}

		if qj.Status.CanRun && !qj.Status.IsDispatched{

			if glog.V(10) {
			current_time:=time.Now()
			glog.V(10).Infof("[Dispatcher Controller] XQJ %s has Overhead Before Dispatching: %s", qj.Name,current_time.Sub(qj.CreationTimestamp.Time))
			glog.V(10).Infof("[TTime] %s, %s: WorkerBeforeDispatch", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
		  }

			qj.Status.IsDispatched = true
			queuejobKey, _:=GetQueueJobKey(qj)
			// obj:=cc.dispatchMap[queuejobKey]
			// if obj!=nil {
			if obj, ok:=cc.dispatchMap[queuejobKey]; ok {
				cc.agentMap[obj].CreateJob(qj)
			}
			if glog.V(10) {
			current_time:=time.Now()
			glog.V(10).Infof("[Dispatcher Controller] XQJ %s has Overhead After Dispatching: %s", qj.Name,current_time.Sub(qj.CreationTimestamp.Time))
			glog.V(10).Infof("[TTime] %s, %s: WorkerAfterDispatch", qj.Name, time.Now().Sub(qj.CreationTimestamp.Time))
			}
			if _, err := cc.arbclients.ArbV1().AppWrappers(qj.Namespace).Update(qj); err != nil {
				glog.Errorf("Failed to update status of AppWrapper %v/%v: %v",
					qj.Namespace, qj.Name, err)
				return err
			}
		}

	}
	return err
}

//Cleanup function
func (cc *XController) Cleanup(queuejob *arbv1.AppWrapper) error {
	glog.V(4).Infof("[Cleanup] begin AppWrapper %s Version=%s Status=%+v\n", queuejob.Name, queuejob.ResourceVersion, queuejob.Status)

	if !cc.isDispatcher {
		if queuejob.Spec.AggrResources.Items != nil {
			// we call clean-up for each controller
			for _, ar := range queuejob.Spec.AggrResources.Items {
				cc.qjobResControls[ar.Type].Cleanup(queuejob, &ar)
			}
		}
	} else {
		// glog.Infof("[Dispatcher] Cleanup: State=%s\n", queuejob.Status.State)
		if queuejob.Status.CanRun && queuejob.Status.IsDispatched {
			queuejobKey, _:=GetQueueJobKey(queuejob)
			if obj, ok:=cc.dispatchMap[queuejobKey]; ok {
				cc.agentMap[obj].DeleteJob(queuejob)
			}
		}
	}

	queuejob.Status.Pending      = 0
	queuejob.Status.Running      = 0
	queuejob.Status.Succeeded    = 0
	queuejob.Status.Failed       = 0
	glog.V(10).Infof("[Cleanup] end   AppWrapper %s Version=%s Status=%+v\n", queuejob.Name, queuejob.ResourceVersion, queuejob.Status)

	return nil
}
