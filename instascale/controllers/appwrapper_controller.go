/*
Copyright 2022.

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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"errors"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned/clients"
	arbinformers "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion"
	v1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	machinev1 "github.com/openshift/api/machine/v1beta1"
	mapiclientset "github.com/openshift/client-go/machine/clientset/versioned"
	machineinformersv1beta1 "github.com/openshift/client-go/machine/informers/externalversions"
	"github.com/openshift/client-go/machine/listers/machine/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AppWrapperReconciler reconciles a AppWrapper object
type AppWrapperReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var nodeCache []string
var scaledAppwrapper []string
var (
	machinesetCache                = make(map[string]int32)
	appWrapperInstanceReplicaCache = make(map[string]map[string]int32)
	instanceTypeGPUMap             = make(map[int]string)
	demandMapPerInstanceType       = make(map[string]int)
	appwrapperToScaledMachineSet   = make(map[string]map[string]int32)
	machinesetToReplicaCount       = make(map[string]int32)
	appWrapperReplicaCache         = make(map[string]int32)
)

const (
	namespaceToList = "openshift-machine-api"
	machinesetName  = "ocp-sched-test-v2-f5hz2-worker-gpu-4-us-east-1a"
	minResyncPeriod = 10 * time.Minute
	kubeconfig      = ""
)

var msLister v1beta1.MachineSetLister
var msInformerHasSynced bool
var machineClient mapiclientset.Interface
var queueJobLister v1.AppWrapperLister
var arbclients *clientset.Clientset

//+kubebuilder:rbac:groups=instascale.ibm.com.instascale.ibm.com,resources=appwrappers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=instascale.ibm.com.instascale.ibm.com,resources=appwrappers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=instascale.ibm.com.instascale.ibm.com,resources=appwrappers/finalizers,verbs=update

//+kubebuilder:rbac:groups=apps,resources=machineset,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=machineset/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppWrapper object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *AppWrapperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	var appwrapper arbv1.AppWrapper
	if err := r.Get(ctx, req.NamespacedName, &appwrapper); err != nil {
		if apierrors.IsNotFound(err) {
			//ignore not-found errors, since we can get them on delete requests.
			return ctrl.Result{}, nil
		}
		klog.Error(err, "unable to fetch appwrapper")
	}
	klog.Infof("The appwrapper name is: %v", appwrapper.Name)

	kubeconfig := os.Getenv("KUBECONFIG")
	//kubeconfig := "/Users/abhishekmalvankar/aws/ocp-sched-test-v2/auth/kubeconfig"
	cb, err := NewClientBuilder(kubeconfig)
	if err != nil {
		klog.Fatalf("Error creating clients: %v", err)
	}
	restConfig, err := getRestConfig(kubeconfig)
	if err != nil {
		klog.Info("Failed to get rest config")
	}
	arbclients = clientset.NewForConfigOrDie(restConfig)
	machineClient = cb.MachineClientOrDie("machine-shared-informer")
	//TODO: resyncPeriod drives how often entries are printed??
	factory := machineinformersv1beta1.NewSharedInformerFactoryWithOptions(machineClient, resyncPeriod()(), machineinformersv1beta1.WithNamespace(""))
	informer := factory.Machine().V1beta1().MachineSets().Informer()
	msLister = factory.Machine().V1beta1().MachineSets().Lister()

	stopper := make(chan struct{})
	defer close(stopper)
	//Openshift libraries complain on below method, kubernetes is ok
	//defer runtime.HandleCrash()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		UpdateFunc: onUpdate,
		DeleteFunc: onDelete,
	})
	go informer.Run(stopper)
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		//Openshift libraries complain on below method, kubernetes is ok
		//runtime.HandleError(fmt.Errorf("TIMED OUT ON CACHE SYNC"))
		//returnt
		klog.Info("Wait for cache to sync")
	}
	//TODO: do we need dual sync??
	msInformerHasSynced = informer.HasSynced()
	go wait.Until(changeReplicasForEveryAppwrapper, 5*time.Second, stopper)
	addAppwrappersThatNeedScaling()
	<-stopper
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppWrapperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&arbv1.AppWrapper{}).
		Complete(r)
}

// ClientBuilder can create a variety of kubernetes client interface
// with its embeded rest.Config.
type ClientBuilder struct {
	config *rest.Config
}

// ProviderSpecFromRawExtension unmarshals the JSON-encoded spec
func ProviderSpecFromRawExtension(rawExtension *runtime.RawExtension) (*machinev1.AWSMachineProviderConfig, error) {
	if rawExtension == nil {
		return &machinev1.AWSMachineProviderConfig{}, nil
	}

	spec := new(machinev1.AWSMachineProviderConfig)
	if err := json.Unmarshal(rawExtension.Raw, &spec); err != nil {
		return nil, fmt.Errorf("error unmarshalling providerSpec: %v", err)
	}

	klog.V(5).Infof("Got provider spec from raw extension: %+v", spec)
	return spec, nil
}

func changeReplicasForEveryAppwrapper() {
	//We got to API server and update replicas based on value in cache
	//TODO: is cache always in sync??
	var scaledAppwrapper = ""
	for appWrapper, instanceReplicaMap := range appWrapperInstanceReplicaCache {
		if msInformerHasSynced {
			allMachineSet, err := msLister.MachineSets("").List(labels.Everything())
			if err != nil {
				klog.Infof("Error listing a machineset, %v", err)
			}
			klog.Infof("The AW needs below instance types to be scaled: %v", instanceReplicaMap)
			for _, aMachineSet := range allMachineSet {
				providerConfig, err := ProviderSpecFromRawExtension(aMachineSet.Spec.Template.Spec.ProviderSpec.Value)
				if err != nil {
					klog.Info(err)
				}
				if replicas, ok := instanceReplicaMap[providerConfig.InstanceType]; ok {
					klog.Infof("The providerspec is %v", providerConfig.InstanceType)
					klog.Infof("Got replicas %d", replicas)
					currentReplicas := *aMachineSet.Spec.Replicas
					klog.Infof("Current machine count for %s machineset is %v", aMachineSet.Name, *aMachineSet.Spec.Replicas)
					//make copy of object from lister and update, NOTE not going to API server
					copyOfaMachineSet := aMachineSet.DeepCopy()
					updatedReplicas := currentReplicas + replicas
					copyOfaMachineSet.Spec.Replicas = &updatedReplicas
					//if there are existing replicas then just increment
					if (*copyOfaMachineSet.Spec.Replicas) >= replicas {
						//scale-up will provide resources like CPU, Mem and GPU.
						//klog.Infof("Performing scale-up for: %s", string(appWrapper))
						klog.Infof("Updating machine count for %s machineset to %v for Appwrapper %s", copyOfaMachineSet.Name, *copyOfaMachineSet.Spec.Replicas, appWrapper)
						machineClient.MachineV1beta1().MachineSets(namespaceToList).Update(context.Background(), copyOfaMachineSet, metav1.UpdateOptions{
							TypeMeta:        metav1.TypeMeta{},
							DryRun:          []string{},
							FieldManager:    "",
							FieldValidation: "",
						})
						scaledAppwrapper = appWrapper
						appWrapperReplicaCache[scaledAppwrapper] = int32(replicas)
						appwrapperToScaledMachineSet[appWrapper] = map[string]int32{}
						appwrapperToScaledMachineSet[appWrapper][copyOfaMachineSet.Name] = *copyOfaMachineSet.Spec.Replicas
						if len(machinesetToReplicaCount) == 0 {
							machinesetToReplicaCount[copyOfaMachineSet.Name] = *copyOfaMachineSet.Spec.Replicas
						} else {
							existingReplicaCount := machinesetToReplicaCount[copyOfaMachineSet.Name]
							machinesetToReplicaCount[copyOfaMachineSet.Name] = existingReplicaCount + *copyOfaMachineSet.Spec.Replicas
						}
					}
					break
				}
			}

		}
		break
	}
	//delete already scaled appwrapper and put scale to zero
	delete(appWrapperInstanceReplicaCache, scaledAppwrapper)
}

func addAppwrappersThatNeedScaling() {
	//AppWrapper integration - starts
	kubeconfig := os.Getenv("KUBECONFIG")
	restConfig, err := getRestConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Error getting config: %v", err)
	}
	awJobClient, _, err := clients.NewClient(restConfig)
	if err != nil {
		klog.Fatalf("Error creating client: %v", err)
	}
	queueJobInformer := arbinformers.NewSharedInformerFactory(awJobClient, 0).AppWrapper().AppWrappers()
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
				AddFunc:    onAdd,
				UpdateFunc: onUpdate,
				DeleteFunc: onDelete,
			},
		})
	queueJobLister = queueJobInformer.Lister()
	queueJobSynced := queueJobInformer.Informer().HasSynced
	stopCh := make(chan struct{})
	defer close(stopCh)
	go queueJobInformer.Informer().Run(stopCh)
	cache.WaitForCacheSync(stopCh, queueJobSynced)
	queuedJobs, err := queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Fatalf("Error listing: %v", err)
	}

	klog.Infof("length of queued AW is: %d", len(queuedJobs))

	for _, newjob := range queuedJobs {
		replicas := newjob.Spec.SchedSpec.MinAvailable
		klog.Infof("The status is: %s", newjob.Status.State)
		klog.Infof("The condition is: %s", newjob.Status.Conditions)
		klog.Infof("The replicas needed: %d", replicas)
	}
	<-stopCh
	// //AppWrapper integration - Ends
}

// NewClientBuilder returns a *ClientBuilder with the given kubeconfig.
func NewClientBuilder(kubeconfig string) (*ClientBuilder, error) {
	config, err := getRestConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return &ClientBuilder{
		config: config,
	}, nil
}

func getRestConfig(kubeconfig string) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if kubeconfig != "" {
		klog.V(10).Infof("Loading kube client config from path %q", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		klog.V(4).Infof("Using in-cluster kube client config")
		config, err = rest.InClusterConfig()
		if err == rest.ErrNotInCluster {
			return nil, errors.New("Not running in-cluster? Try using --kubeconfig")
		}
	}
	return config, err
}

// onAdd is the function executed when the kubernetes informer notified the
// presence of a new kubernetes node in the cluster
func onAdd(obj interface{}) {
	node, ok := obj.(*machinev1.MachineSet)
	if ok {
		name := node.GetName()
		//add machineset names to nodeCache string array
		//TODO: do we still need the node cache??
		nodeCache = append(nodeCache, name)
	}
	aw, ok := obj.(*arbv1.AppWrapper)
	if ok {
		klog.Infof("Added new Appwrapper named %s", aw.Name)
	}

}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func onUpdate(old, new interface{}) {
	node, ok := new.(*machinev1.MachineSet)
	if ok {
		//newMachineset := new.(*machinev1.MachineSet)
		machinesetName := node.GetName()
		newReplicas := *node.Spec.Replicas
		machinesetCache[machinesetName] = newReplicas
	}

	aw, ok := new.(*arbv1.AppWrapper)
	if ok {
		status := aw.Status.State
		//assuming conditions are always ordered
		allconditions := aw.Status.Conditions
		if len(allconditions) > 0 {
			aConditions := aw.Status.Conditions[len(aw.Status.Conditions)-1]
			//TODO: MachineSetInformer resyncs it calls the scaleUp for the same appwrapper since the condition is not updated
			if status == "Pending" && strings.Contains(aConditions.Message, "Insufficient") && !contains(scaledAppwrapper, aw.Name) {
				klog.Infof("Found AW that does not have enough resources to run %v, %v", status, aConditions)
				scaledAppwrapper = append(scaledAppwrapper, aw.Name)
				scaleUp(aw)
			}
		}
		if status == "Completed" {
			klog.Info("Job completed, deleting AppWrapper to trigger scaling down")
			foreground := metav1.DeletePropagationForeground
			arbclients.ArbV1().AppWrappers(aw.Namespace).Delete(aw.Name, &metav1.DeleteOptions{
				PropagationPolicy: &foreground,
			})
		}

	}

}

func scaleUp(aw *arbv1.AppWrapper) {
	//status := aw.Status.State
	//TODO: Delay is needed, so running for loop to get conditions array filled by AW controller
	// var allconditions []arbv1.AppWrapperCondition
	// awRes, _ := obj.(*arbv1.AppWrapperResource)
	//Init Machineset
	instanceTypeGPUMap[1] = "p3.2xlarge"
	instanceTypeGPUMap[4] = "p3.8xlarge"
	instanceTypeGPUMap[8] = "p3.16xlarge"

	for _, genericItem := range aw.Spec.AggrResources.GenericItems {
		itemsList, _ := GetListOfPodResourcesFromOneGenericItem(&genericItem)
		klog.Infof("The custompod resources in item list are: %v", itemsList)
		for i := 0; i < len(itemsList); i++ {
			if itemsList[i].GPU == 0 {
				//TODO need to consider CPU resource for scaling
				klog.Info("Found CPU only container or pod")
				//hard-coding CPU instance type to one
				demandMapPerInstanceType["m4.large"] = 1
			} else {
				klog.Info("Found GPU container or pod")
				//klog.Infof("The length of instance GPU map is: %d", len(instanceTypeGPUMap))
				keys := make([]int, 0, len(instanceTypeGPUMap))
				for k := range instanceTypeGPUMap {
					keys = append(keys, k)
				}
				sort.Ints(keys)
				var foundMatchingInstanceType bool
				for _, k := range keys {
					//klog.Infof("GPU from  yaml %d and GPU from instance %d", itemsList[i].GPU, numGpuPerInstance)
					if itemsList[i].GPU <= int64(k) {
						instanceName := instanceTypeGPUMap[k]
						klog.Infof("Need instance type: %s", instanceName)
						if val, ok := demandMapPerInstanceType[instanceName]; ok {
							//klog.Infof("current val: %d", demandMapPerInstanceType[instanceName])
							demandMapPerInstanceType[instanceName] = val + 1
							//klog.Infof("after append val: %d", demandMapPerInstanceType[instanceName])
						} else {
							demandMapPerInstanceType[instanceName] = 1
							//klog.Infof("added value - init")
						}
						//Need one instance type, stop if found one
						foundMatchingInstanceType = true
						break
					}

				}
				if !foundMatchingInstanceType {
					// TODO: should we queue AW and never scale?
					klog.Info("Found a pod or container for which no machineset exists to satisfy resource requirement")
					break
				}
			}
		}
		//Demand
		for instanceName, numReplicas := range demandMapPerInstanceType {
			klog.Infof("The key : %s and value: %d", instanceName, numReplicas)
			appWrapperInstanceReplicaCache[aw.Name+"-"+instanceName] = map[string]int32{}
			appWrapperInstanceReplicaCache[aw.Name+"-"+instanceName][instanceName] = int32(numReplicas)
		}
	}
}

//This function is changed from genericresource.go file as we force to look for resources under custompodresources.
func GetListOfPodResourcesFromOneGenericItem(awr *arbv1.AppWrapperGenericResource) (resource []*clusterstateapi.Resource, er error) {
	var podResourcesList []*clusterstateapi.Resource

	podTotalresource := clusterstateapi.EmptyResource()
	var replicas int
	var res *clusterstateapi.Resource
	if awr.GenericTemplate.Raw != nil {
		//_, replicas, _ := hasFields(awr.GenericTemplate)
		podresources := awr.CustomPodResources
		for _, item := range podresources {
			res, replicas = getPodResourcesWithReplicas(item)
			podTotalresource = podTotalresource.Add(res)
		}
		klog.V(8).Infof("[GetListOfPodResourcesFromOneGenericItem] Requested total allocation resource from 1 pod `%v`.\n", podTotalresource)

		// Addd individual pods to results
		var replicaCount int = int(replicas)
		for i := 0; i < replicaCount; i++ {
			podResourcesList = append(podResourcesList, podTotalresource)
		}
	}

	return podResourcesList, nil
}

// MachineClientOrDie returns the machine api client interface for machine api objects.
func (cb *ClientBuilder) MachineClientOrDie(name string) mapiclientset.Interface {
	return mapiclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

func resyncPeriod() func() time.Duration {
	return func() time.Duration {
		factor := rand.Float64() + 1
		return time.Duration(float64(minResyncPeriod.Nanoseconds()) * factor)
	}
}

func getPodResourcesWithReplicas(pod arbv1.CustomPodResourceTemplate) (resource *clusterstateapi.Resource, count int) {
	replicas := pod.Replicas
	req := clusterstateapi.NewResource(pod.Requests)
	limit := clusterstateapi.NewResource(pod.Limits)
	tolerance := 0.001

	// Use limit if request is 0
	if diff := math.Abs(req.MilliCPU - float64(0.0)); diff < tolerance {
		req.MilliCPU = limit.MilliCPU
	}

	if diff := math.Abs(req.Memory - float64(0.0)); diff < tolerance {
		req.Memory = limit.Memory
	}

	if req.GPU <= 0 {
		req.GPU = limit.GPU
	}
	// req.MilliCPU = req.MilliCPU
	// req.Memory = req.Memory
	// req.GPU = req.GPU
	return req, replicas
}

func onDelete(obj interface{}) {
	aw, ok := obj.(*arbv1.AppWrapper)
	if ok {
		klog.Infof("Appwrapper deleted scale-down machineset: %s ", aw.Name)
		scaleDown(aw)
	}
}

func scaleDown(aw *arbv1.AppWrapper) {
	//machinesetToReplicas := appwrapperToScaledMachineSet[aw.Name]
	for aWPlusInstanceName, machinesetToReplicas := range appwrapperToScaledMachineSet {
		//klog.Infof("The AW name is %s and AWinstance name is %s", aw.Name, aWPlusInstanceName)
		if strings.Contains(aWPlusInstanceName, aw.Name) {
			allMachineSet, err := msLister.MachineSets("").List(labels.Everything())
			if err != nil {
				klog.Infof("Error listing a machineset, %v", err)
			}
			for machinesetName, TotalReplicasScaledByAllAppwrappers := range machinesetToReplicas {
				for _, aMachineSet := range allMachineSet {
					if aMachineSet.Name == machinesetName {
						copyOfaMachineSet := aMachineSet.DeepCopy()
						scaledReplicasForAnAppwrapper := appWrapperReplicaCache[aWPlusInstanceName]
						var scaleDown = int32(0)
						if TotalReplicasScaledByAllAppwrappers > scaledReplicasForAnAppwrapper {
							scaleDown = TotalReplicasScaledByAllAppwrappers - scaledReplicasForAnAppwrapper
						}
						klog.Infof("Replicas scale-down by controller for an Appwrapper instance %s is %d", aw.Name, scaleDown)
						copyOfaMachineSet.Spec.Replicas = &scaleDown
						machineClient.MachineV1beta1().MachineSets(namespaceToList).Update(context.Background(), copyOfaMachineSet, metav1.UpdateOptions{
							TypeMeta:        metav1.TypeMeta{},
							DryRun:          []string{},
							FieldManager:    "",
							FieldValidation: "",
						})
					}
				}
			}

		}
	}

}
