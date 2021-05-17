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

package networkpolicy

import (
	"context"
	"fmt"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"

	//schedulerapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/scheduler/api"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	networkingv1 "k8s.io/api/networking/v1"
	networkingv1informer "k8s.io/client-go/informers/networking/v1"
	networkingv1lister "k8s.io/client-go/listers/networking/v1"
)

var queueJobKind = arbv1.SchemeGroupVersion.WithKind("AppWrapper")
var queueJobName = "appwrapper.mcad.ibm.com"

const (
	// QueueJobNameLabel label string for queuejob name
	QueueJobNameLabel string = "appwrapper-name"

	// ControllerUIDLabel label string for queuejob controller uid
	ControllerUIDLabel string = "controller-uid"
)

//QueueJobResService contains service info
type QueueJobResNetworkPolicy struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	networkpolicyStore    networkingv1lister.NetworkPolicyLister
	networkpolicyInformer networkingv1informer.NetworkPolicyInformer
	rtScheme              *runtime.Scheme
	jsonSerializer        *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypeNetworkPolicy, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResNetworkPolicy(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResNetworkPolicy(config *rest.Config) queuejobresources.Interface {
	qjrNetworkPolicy := &QueueJobResNetworkPolicy{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrNetworkPolicy.networkpolicyInformer = informers.NewSharedInformerFactory(qjrNetworkPolicy.clients, 0).Networking().V1().NetworkPolicies()
	qjrNetworkPolicy.networkpolicyInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *networkingv1.NetworkPolicy:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrNetworkPolicy.addNetworkPolicy,
				UpdateFunc: qjrNetworkPolicy.updateNetworkPolicy,
				DeleteFunc: qjrNetworkPolicy.deleteNetworkPolicy,
			},
		})

	qjrNetworkPolicy.rtScheme = runtime.NewScheme()
	// v1.AddToScheme(qjrNetworkPolicy.rtScheme)
	networkingv1.AddToScheme(qjrNetworkPolicy.rtScheme) // Tonghoon

	qjrNetworkPolicy.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrNetworkPolicy.rtScheme, qjrNetworkPolicy.rtScheme)

	qjrNetworkPolicy.refManager = queuejobresources.NewLabelRefManager()

	return qjrNetworkPolicy
}

// Run the main goroutine responsible for watching and services.
func (qjrNetworkPolicy *QueueJobResNetworkPolicy) Run(stopCh <-chan struct{}) {

	qjrNetworkPolicy.networkpolicyInformer.Informer().Run(stopCh)
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) GetAggregatedResources(job *arbv1.AppWrapper) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) addNetworkPolicy(obj interface{}) {

	return
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) updateNetworkPolicy(old, cur interface{}) {

	return
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) deleteNetworkPolicy(obj interface{}) {

	return
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) GetAggregatedResourcesByPriority(priority float64, job *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	return total
}

// Parse queue job api object to get Service template
func (qjrNetworkPolicy *QueueJobResNetworkPolicy) getNetworkPolicyTemplate(qjobRes *arbv1.AppWrapperResource) (*networkingv1.NetworkPolicy, error) {

	networkpolicyGVK := schema.GroupVersion{Group: networkingv1.GroupName, Version: "v1"}.WithKind("NetworkPolicy")
	obj, _, err := qjrNetworkPolicy.jsonSerializer.Decode(qjobRes.Template.Raw, &networkpolicyGVK, nil)
	if err != nil {
		// klog.Infof("Decoding Error for NetworkPolicy=================================================")
		return nil, err
	}

	networkpolicy, ok := obj.(*networkingv1.NetworkPolicy)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a NetworkPolicy")
	}

	return networkpolicy, nil

}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) createNetworkPolicyWithControllerRef(namespace string, networkpolicy *networkingv1.NetworkPolicy, controllerRef *metav1.OwnerReference) error {

	// klog.V(4).Infof("==========create NetworkPolicy: %+v \n", networkpolicy)
	if controllerRef != nil {
		networkpolicy.OwnerReferences = append(networkpolicy.OwnerReferences, *controllerRef)
	}

	if _, err := qjrNetworkPolicy.clients.NetworkingV1().NetworkPolicies(namespace).Create(context.Background(), networkpolicy, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) delNetworkPolicy(namespace string, name string) error {

	klog.V(4).Infof("==========delete networkpolicy: %s \n", name)
	if err := qjrNetworkPolicy.clients.NetworkingV1().NetworkPolicies(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()

	defer func() {
		// klog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, networkPolicyInQjr, networkPoliciesInEtcd, err := qjrNetworkPolicy.getNetworkPolicyForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	networkPolicyLen := len(networkPoliciesInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(networkPolicyLen)

	klog.V(4).Infof("QJob: %s had %d NetworkPolicies and %d desired NetworkPolicies", queuejob.Name, networkPolicyLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpNetworkPolicy := networkingv1.NetworkPolicy{}
		err = qjrNetworkPolicy.refManager.AddReference(qjobRes, &tmpNetworkPolicy)
		if err != nil {
			klog.Errorf("Cannot add reference to configmap resource %+v", err)
			return err
		}

		if networkPolicyInQjr.Labels == nil {
			networkPolicyInQjr.Labels = map[string]string{}
		}
		for k, v := range tmpNetworkPolicy.Labels {
			networkPolicyInQjr.Labels[k] = v
		}
		networkPolicyInQjr.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()

				err := qjrNetworkPolicy.createNetworkPolicyWithControllerRef(*_namespace, networkPolicyInQjr, metav1.NewControllerRef(queuejob, queueJobKind))

				if err != nil && errors.IsTimeout(err) {
					return
				}
				if err != nil {
					defer utilruntime.HandleError(err)
				}
			}()
		}
		wait.Wait()
	}

	return nil
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) getNetworkPolicyForQueueJobRes(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) (*string, *networkingv1.NetworkPolicy, []*networkingv1.NetworkPolicy, error) {

	// Get "a" NetworkPolicy from AppWrapper Resource
	networkPolicyInQjr, err := qjrNetworkPolicy.getNetworkPolicyTemplate(qjobRes)
	if err != nil {
		klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get NetworkPolicy"s" in Etcd Server
	var _namespace *string
	if networkPolicyInQjr.Namespace != "" {
		_namespace = &networkPolicyInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	// networkPolicyList, err := qjrNetworkPolicy.clients.Networking().NetworkPolicies(*_namespace).List(metav1.ListOptions{})
	networkPolicyList, err := qjrNetworkPolicy.clients.NetworkingV1().NetworkPolicies(*_namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name)})
	if err != nil {
		return nil, nil, nil, err
	}
	networkPoliciesInEtcd := []*networkingv1.NetworkPolicy{}
	for i, _ := range networkPolicyList.Items {
		networkPoliciesInEtcd = append(networkPoliciesInEtcd, &networkPolicyList.Items[i])
	}
	myNetworkPoliciesInEtcd := []*networkingv1.NetworkPolicy{}
	for i, networkPolicy := range networkPoliciesInEtcd {
		if qjrNetworkPolicy.refManager.BelongTo(qjobRes, networkPolicy) {
			myNetworkPoliciesInEtcd = append(myNetworkPoliciesInEtcd, networkPoliciesInEtcd[i])
		}
	}

	return _namespace, networkPolicyInQjr, myNetworkPoliciesInEtcd, nil
}

func (qjrNetworkPolicy *QueueJobResNetworkPolicy) deleteQueueJobResNetworkPolicies(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

	job := *queuejob

	_namespace, _, activeNetworkPolicies, err := qjrNetworkPolicy.getNetworkPolicyForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activeNetworkPolicies))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrNetworkPolicy.delNetworkPolicy(*_namespace, activeNetworkPolicies[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				klog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeNetworkPolicies[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrNetworkPolicy *QueueJobResNetworkPolicy) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrNetworkPolicy.deleteQueueJobResNetworkPolicies(qjobRes, queuejob)
}
