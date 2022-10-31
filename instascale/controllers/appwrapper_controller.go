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
	"fmt"
	"strconv"

	"os"
	"strings"
	"time"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned/clients"
	arbinformers "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/informers/controller-externalversion"
	v1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"
	mapiclientset "github.com/openshift/client-go/machine/clientset/versioned"
	machineinformersv1beta1 "github.com/openshift/client-go/machine/informers/externalversions"
	"github.com/openshift/client-go/machine/listers/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
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

//var nodeCache []string
var scaledAppwrapper []string

const (
	namespaceToList = "openshift-machine-api"
	minResyncPeriod = 10 * time.Minute
	kubeconfig      = ""
)

var maxScaleNodesAllowed int
var msLister v1beta1.MachineSetLister
var msInformerHasSynced bool
var machineClient mapiclientset.Interface
var queueJobLister v1.AppWrapperLister

//var arbclients *clientset.Clientset
var kubeClient *kubernetes.Clientset

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
	//klog.Infof("The appwrapper name is: %v", appwrapper.Name)

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
	//arbclients = clientset.NewForConfigOrDie(restConfig)
	machineClient = cb.MachineClientOrDie("machine-shared-informer")
	kubeClient, _ = kubernetes.NewForConfig(restConfig)
	factory := machineinformersv1beta1.NewSharedInformerFactoryWithOptions(machineClient, resyncPeriod()(), machineinformersv1beta1.WithNamespace(""))
	informer := factory.Machine().V1beta1().MachineSets().Informer()
	msLister = factory.Machine().V1beta1().MachineSets().Lister()
	nodesTobeadded, err := kubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "instascale-config", metav1.GetOptions{})
	if err != nil {
		klog.Infof("Config map named instascale-config is not available in namespace kube-system")
	}
	for _, v := range nodesTobeadded.Data {
		if maxScaleNodesAllowed, err = strconv.Atoi(v); err != nil {
			klog.Infof("Error configuring value of configmap %v using value  3", maxScaleNodesAllowed)
			maxScaleNodesAllowed = 3
		}
	}
	klog.Infof("Got config map named: %v that configures max nodes in cluster to value %v", nodesTobeadded.Name, maxScaleNodesAllowed)
	stopper := make(chan struct{})
	defer close(stopper)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    onAdd,
		UpdateFunc: onUpdate,
		DeleteFunc: onDelete,
	})
	go informer.Run(stopper)
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		klog.Info("Wait for cache to sync")
	}
	//TODO: do we need dual sync??
	msInformerHasSynced = informer.HasSynced()
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

func addAppwrappersThatNeedScaling() {
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
	// queuedJobs, err := queueJobLister.AppWrappers("").List(labels.Everything())
	// if err != nil {
	// 	klog.Fatalf("Error listing: %v", err)
	// }

	//klog.Infof("length of queued AW is: %d", len(queuedJobs))

	<-stopCh
}

func canScale(demandPerInstanceType map[string]int) bool {
	//init for 3 since cluster has 3 master nodes
	var totalNodes int32 = 3
	allMachineSet, err := msLister.MachineSets("").List(labels.Everything())
	if err != nil {
		klog.Infof("Error listing a machineset, %v", err)
	}
	for _, aMachine := range allMachineSet {
		totalNodes += *aMachine.Spec.Replicas
	}
	for _, count := range demandPerInstanceType {
		totalNodes += int32(count)
	}
	klog.Infof("The nodes allowed: %v and total nodes in cluster after node scale-out %v", maxScaleNodesAllowed, totalNodes)
	return totalNodes < int32(maxScaleNodesAllowed)
}

// onAdd is the function executed when the kubernetes informer notified the
// presence of a new kubernetes node in the cluster
func onAdd(obj interface{}) {
	aw, ok := obj.(*arbv1.AppWrapper)
	if ok {
		klog.Infof("Found Appwrapper named %s that has status %v", aw.Name, aw.Status.State)
		if aw.Status.State == arbv1.AppWrapperStateEnqueued || aw.Status.State == "" {
			//scaledAppwrapper = append(scaledAppwrapper, aw.Name)
			demandPerInstanceType := discoverInstanceTypes(aw)
			if canScale(demandPerInstanceType) {
				scaleUp(aw, demandPerInstanceType)
			} else {
				klog.Infof("Cannot scale up replicas max replicas allowed is %v", maxScaleNodesAllowed)
			}
		}

	}

}

func onUpdate(old, new interface{}) {
	aw, ok := new.(*arbv1.AppWrapper)
	if ok {
		status := aw.Status.State
		if status == "Completed" {
			klog.Info("Job completed, deleting resources owned")
			//can delete AW if needed
			// foreground := metav1.DeletePropagationForeground
			// arbclients.ArbV1().AppWrappers(aw.Namespace).Delete(aw.Name, &metav1.DeleteOptions{
			// 	PropagationPolicy: &foreground,
			// })

			deleteMachineSet(aw)
		}
		if contains(scaledAppwrapper, aw.Name) {
			//klog.Infof("Already scaled appwrapper %v", aw.Name)
			return
		}
		// if reUseMachineSet() {
		// 	return
		// }

		pending, aw := IsAwPending()
		if pending {
			demandPerInstanceType := discoverInstanceTypes(aw)
			if canScale(demandPerInstanceType) {
				scaleUp(aw, demandPerInstanceType)
			} else {
				klog.Infof("Cannot scale up replicas max replicas allowed is %v", maxScaleNodesAllowed)
			}
		}

	}

}

func discoverInstanceTypes(aw *arbv1.AppWrapper) map[string]int {
	demandMapPerInstanceType := make(map[string]int)
	var instanceRequired []string
	for k, v := range aw.Labels {
		if k == "orderedinstance" {
			instanceRequired = strings.Split(v, "_")
		}
	}
	//instanceRequired := res[len(res)-1]
	//klog.Infof("Extracting instance name from AW name %v and using %v", aw.Name, instanceRequired)

	for _, genericItem := range aw.Spec.AggrResources.GenericItems {
		for idx, val := range genericItem.CustomPodResources {
			instanceName := instanceRequired[idx]
			demandMapPerInstanceType[instanceName] = val.Replicas
		}
	}
	//klog.Infof("the demand map is %v", demandMapPerInstanceType)
	return demandMapPerInstanceType
}

func scaleUp(aw *arbv1.AppWrapper, demandMapPerInstanceType map[string]int) {
	if msInformerHasSynced {
		//Assumption is made that the cluster has machineset configure that AW needs
		for userRequestedInstanceType := range demandMapPerInstanceType {
			//TODO: get unique machineset
			replicas := demandMapPerInstanceType[userRequestedInstanceType]
			scaleMachineSet(aw, userRequestedInstanceType, replicas)
		}
		klog.Infof("Completed Scaling for %v", aw.Name)
		scaledAppwrapper = append(scaledAppwrapper, aw.Name)
	}

}

func scaleMachineSet(aw *arbv1.AppWrapper, userRequestedInstanceType string, replicas int) {
	allMachineSet, err := msLister.MachineSets("").List(labels.Everything())
	if err != nil {
		klog.Infof("Error listing a machineset, %v", err)
	}
	for _, aMachineSet := range allMachineSet {
		providerConfig, err := ProviderSpecFromRawExtension(aMachineSet.Spec.Template.Spec.ProviderSpec.Value)
		if err != nil {
			klog.Infof("Error retrieving provider config %v", err)
		}
		if userRequestedInstanceType == providerConfig.InstanceType {
			//klog.Infof("working on %v", userRequestedInstanceType)
			copyOfaMachineSet := aMachineSet.DeepCopy()
			//updatedReplicas := currentReplicas + replicas
			replicas := int32(replicas)
			copyOfaMachineSet.Spec.Replicas = &replicas
			copyOfaMachineSet.ResourceVersion = ""
			copyOfaMachineSet.Spec.Template.Spec.Taints = []corev1.Taint{{Key: aw.Name, Value: "value1", Effect: "PreferNoSchedule"}}
			copyOfaMachineSet.Name = aw.Name + "-" + userRequestedInstanceType
			copyOfaMachineSet.Spec.Template.Labels = map[string]string{
				aw.Name: aw.Name,
			}
			workerLabels := map[string]string{
				aw.Name: aw.Name,
			}
			copyOfaMachineSet.Spec.Selector = metav1.LabelSelector{
				MatchLabels: workerLabels,
			}
			copyOfaMachineSet.Labels["aw"] = aw.Name
			ms, err := machineClient.MachineV1beta1().MachineSets(namespaceToList).Create(context.Background(), copyOfaMachineSet, metav1.CreateOptions{})
			if err != nil {
				klog.Infof("Error creating machineset %v", err)
				return
			}
			//wait until all replicas are available
			for (replicas - ms.Status.AvailableReplicas) != 0 {
				//TODO: user can delete appwrapper work on triggering scale-down
				klog.Infof("waiting for machines to be in state Ready. replicas needed: %v and replicas available: %v", replicas, ms.Status.AvailableReplicas)
				time.Sleep(1 * time.Minute)
				ms, _ = machineClient.MachineV1beta1().MachineSets(namespaceToList).Get(context.Background(), copyOfaMachineSet.Name, metav1.GetOptions{})
				klog.Infof("Querying machinset %v to get updated replicas", ms.Name)
			}
			klog.Infof("Machines are available. replicas needed: %v and replicas available: %v", replicas, ms.Status.AvailableReplicas)
			allMachines, errm := machineClient.MachineV1beta1().Machines(namespaceToList).List(context.Background(), metav1.ListOptions{})
			if errm != nil {
				klog.Infof("Error creating machineset: %v", errm)
			}
			//map machines to machinesets?
			for idx := range allMachines.Items {
				machine := &allMachines.Items[idx]
				for k, _ := range machine.Labels {
					if k == aw.Name {
						nodeName := machine.Status.NodeRef.Name
						labelPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/%s","value":"%s" }]`, aw.Name, aw.Name)
						ms, err := kubeClient.CoreV1().Nodes().Patch(context.Background(), nodeName, types.JSONPatchType, []byte(labelPatch), metav1.PatchOptions{})
						if len(ms.Labels) > 0 && err == nil {
							klog.Infof("label added to node %v, for scaling %v", nodeName, copyOfaMachineSet.Name)
						}
					}
				}
			}
			return
		}
	}
}

// func addLabelsToNodes(ms machinev1.MachineSet, aw *arbv1.AppWrapper) {
// 	allMachines, _ := machineClient.MachineV1beta1().Machines("").List(context.Background(), metav1.ListOptions{})
// 	for idx := range allMachines.Items {
// 		machine := &allMachines.Items[idx]
// 		for k, _ := range machine.Labels {
// 			if k == "role" {
// 				nodeName := machine.Status.NodeRef.Name
// 				labelPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/%s","value":"%s" }]`, "role", aw.Name)
// 				ms, err := kubeClient.CoreV1().Nodes().Patch(context.Background(), nodeName, types.JSONPatchType, []byte(labelPatch), metav1.PatchOptions{})
// 				klog.Infof("The error is %v", err)
// 				klog.Infof("Got ms %v", ms)
// 			}
// 		}
// 	}
// 	scaledAppwrapper = append(scaledAppwrapper, aw.Name)
// }

func IsAwPending() (false bool, aw *arbv1.AppWrapper) {
	queuedJobs, err := queueJobLister.AppWrappers("").List(labels.Everything())
	if err != nil {
		klog.Fatalf("Error listing: %v", err)
	}
	for _, aw := range queuedJobs {
		//skip
		if contains(scaledAppwrapper, aw.Name) {
			continue
		}
		//klog.Infof("Inside for loop %v", aw.Name)
		status := aw.Status.State
		//klog.Infof("The status is %v", status)
		allconditions := aw.Status.Conditions
		//klog.Infof("The conditions are %v", allconditions)
		for _, condition := range allconditions {
			if status == "Pending" && strings.Contains(condition.Message, "Insufficient") {
				klog.Infof("Pending AppWrapper %v needs scaling", aw.Name)
				return true, aw
			}
		}
	}
	return false, nil
}

func onDelete(obj interface{}) {
	aw, ok := obj.(*arbv1.AppWrapper)
	if ok {
		klog.Infof("Appwrapper deleted scale-down machineset: %s ", aw.Name)
		// if reUseMachineSet() {
		// 	return
		// }
		scaleDown(aw)
	}

}

//un-used function, should be a seperate thread
//This should filter already seen appwrappers
// func reUseMachineSet(aw *arbv1.AppWrapper) bool {
// 	queuedJobs, _ := queueJobLister.AppWrappers("").List(labels.Everything())
// 	allMachineSet, _ := msLister.MachineSets("").List(labels.Everything())
// 	for _, aw := range queuedJobs {
// 		for _, aMachineset := range allMachineSet {
// 			if strings.Contains(aw.Name, aMachineset.Name) {
// 				klog.Infof("Need to reuse")
// 				return true
// 			}
// 		}

// 	}
// 	return false
// }

func deleteMachineSet(aw *arbv1.AppWrapper) {
	var err error
	allMachineSet, _ := msLister.MachineSets("").List(labels.Everything())
	for _, aMachineSet := range allMachineSet {
		//klog.Infof("%v.%v", aMachineSet.Name, aw.Name)
		if strings.Contains(aMachineSet.Name, aw.Name) {
			//klog.Infof("Deleting machineset named %v", aw.Name)
			err = machineClient.MachineV1beta1().MachineSets(namespaceToList).Delete(context.Background(), aMachineSet.Name, metav1.DeleteOptions{})
			if err == nil {
				klog.Infof("Delete successful")
			}
		}
	}
}

func scaleDown(aw *arbv1.AppWrapper) {
	//klog.Infof("Inside scale down")
	deleteMachineSet(aw)
	//make a seperate slice
	for idx := range scaledAppwrapper {
		if scaledAppwrapper[idx] == aw.Name {
			scaledAppwrapper[idx] = ""
		}
	}

	pending, aw := IsAwPending()
	if pending {
		demandPerInstanceType := discoverInstanceTypes(aw)
		scaleUp(aw, demandPerInstanceType)
	}
}
