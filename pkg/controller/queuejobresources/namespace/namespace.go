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

package namespace

import (
	"context"
	"fmt"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"

	//schedulerapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/scheduler/api"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"

	// "k8s.io/apimachinery/pkg/api/meta"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
type QueueJobResNamespace struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	namespaceStore    corelisters.NamespaceLister
	namespaceInformer corev1informer.NamespaceInformer
	rtScheme          *runtime.Scheme
	jsonSerializer    *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypeNamespace, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResNamespace(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResNamespace(config *rest.Config) queuejobresources.Interface {
	qjrNamespace := &QueueJobResNamespace{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrNamespace.namespaceInformer = informers.NewSharedInformerFactory(qjrNamespace.clients, 0).Core().V1().Namespaces()
	qjrNamespace.namespaceInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.Namespace:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrNamespace.addNamespace,
				UpdateFunc: qjrNamespace.updateNamespace,
				DeleteFunc: qjrNamespace.deleteNamespace,
			},
		})

	qjrNamespace.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrNamespace.rtScheme)

	qjrNamespace.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrNamespace.rtScheme, qjrNamespace.rtScheme)

	qjrNamespace.refManager = queuejobresources.NewLabelRefManager()

	return qjrNamespace
}

// Run the main goroutine responsible for watching and services.
func (qjrNamespace *QueueJobResNamespace) Run(stopCh <-chan struct{}) {

	qjrNamespace.namespaceInformer.Informer().Run(stopCh)
}

func (qjrNamespace *QueueJobResNamespace) GetAggregatedResources(job *arbv1.AppWrapper) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrNamespace *QueueJobResNamespace) addNamespace(obj interface{}) {

	return
}

func (qjrNamespace *QueueJobResNamespace) updateNamespace(old, cur interface{}) {

	return
}

func (qjrNamespace *QueueJobResNamespace) deleteNamespace(obj interface{}) {

	return
}

func (qjrNamespace *QueueJobResNamespace) GetAggregatedResourcesByPriority(priority float64, job *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	return total
}

// Parse queue job api object to get Service template
func (qjrNamespace *QueueJobResNamespace) getNamespaceTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.Namespace, error) {

	namespaceGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("Namespace")

	obj, _, err := qjrNamespace.jsonSerializer.Decode(qjobRes.Template.Raw, &namespaceGVK, nil)
	if err != nil {
		return nil, err
	}

	namespace, ok := obj.(*v1.Namespace)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a Namespace")
	}

	return namespace, nil

}

func (qjrNamespace *QueueJobResNamespace) createNamespaceWithControllerRef(namespace *v1.Namespace, controllerRef *metav1.OwnerReference) error {

	// klog.V(4).Infof("==========create Namespace: %+v \n", namespace)
	if controllerRef != nil {
		namespace.OwnerReferences = append(namespace.OwnerReferences, *controllerRef)
	}

	if _, err := qjrNamespace.clients.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrNamespace *QueueJobResNamespace) delNamespace(name string) error {

	klog.V(4).Infof("==========delete namespace: %s \n", name)
	if err := qjrNamespace.clients.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrNamespace *QueueJobResNamespace) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

//SyncQueueJob syncs the services
func (qjrNamespace *QueueJobResNamespace) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
		// klog.V(4).Infof("Finished syncing queue job resource %s (%v)", qjobRes.Template, time.Now().Sub(startTime))
	}()

	namespaces, err := qjrNamespace.getNamespaceForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	namespaceLen := len(namespaces)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(namespaceLen)

	klog.V(4).Infof("QJob: %s had %d namespaces and %d desired namespaces", queuejob.Name, namespaceLen, replicas)

	if diff > 0 {
		template, err := qjrNamespace.getNamespaceTemplate(qjobRes)
		if err != nil {
			klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
			return err
		}
		//TODO: need set reference after Service has been really added
		tmpNamespace := v1.Namespace{}
		err = qjrNamespace.refManager.AddReference(qjobRes, &tmpNamespace)
		if err != nil {
			klog.Errorf("Cannot add reference to namespace resource %+v", err)
			return err
		}

		if template.Labels == nil {
			template.Labels = map[string]string{}
		}
		for k, v := range tmpNamespace.Labels {
			template.Labels[k] = v
		}
		template.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()
				err := qjrNamespace.createNamespaceWithControllerRef(template, metav1.NewControllerRef(queuejob, queueJobKind))
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

func (qjrNamespace *QueueJobResNamespace) getNamespaceForQueueJob(j *arbv1.AppWrapper) ([]*v1.Namespace, error) {
	// namespacelist, err := qjrNamespace.clients.CoreV1().Namespaces().List(metav1.ListOptions{})
	namespacelist, err := qjrNamespace.clients.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, j.Name)})
	if err != nil {
		return nil, err
	}

	namespaces := []*v1.Namespace{}
	for i, _ := range namespacelist.Items {
		namespaces = append(namespaces, &namespacelist.Items[i])
	}

	return namespaces, nil

}

func (qjrNamespace *QueueJobResNamespace) getNamespaceForQueueJobRes(qjobRes *arbv1.AppWrapperResource, j *arbv1.AppWrapper) ([]*v1.Namespace, error) {

	namespaces, err := qjrNamespace.getNamespaceForQueueJob(j)
	if err != nil {
		return nil, err
	}

	myNamespaces := []*v1.Namespace{}
	for i, namespace := range namespaces {
		if qjrNamespace.refManager.BelongTo(qjobRes, namespace) {
			myNamespaces = append(myNamespaces, namespaces[i])
		}
	}

	return myNamespaces, nil

}

func (qjrNamespace *QueueJobResNamespace) deleteQueueJobResNamespaces(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

	job := *queuejob

	activeNamespaces, err := qjrNamespace.getNamespaceForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activeNamespaces))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrNamespace.delNamespace(activeNamespaces[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				klog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeNamespaces[ix].Name, job.Namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrNamespace *QueueJobResNamespace) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrNamespace.deleteQueueJobResNamespaces(qjobRes, queuejob)
}
