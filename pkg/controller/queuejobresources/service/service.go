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

package service

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
type QueueJobResService struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	serviceStore    corelisters.ServiceLister
	serviceInformer corev1informer.ServiceInformer
	rtScheme        *runtime.Scheme
	jsonSerializer  *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypeService, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResService(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResService(config *rest.Config) queuejobresources.Interface {
	qjrService := &QueueJobResService{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrService.serviceInformer = informers.NewSharedInformerFactory(qjrService.clients, 0).Core().V1().Services()
	qjrService.serviceInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.Service:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrService.addService,
				UpdateFunc: qjrService.updateService,
				DeleteFunc: qjrService.deleteService,
			},
		})

	qjrService.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrService.rtScheme)

	qjrService.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrService.rtScheme, qjrService.rtScheme)

	qjrService.refManager = queuejobresources.NewLabelRefManager()

	return qjrService
}

// Run the main goroutine responsible for watching and services.
func (qjrService *QueueJobResService) Run(stopCh <-chan struct{}) {

	qjrService.serviceInformer.Informer().Run(stopCh)
}

func (qjrService *QueueJobResService) GetAggregatedResources(job *arbv1.AppWrapper) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrService *QueueJobResService) addService(obj interface{}) {

	return
}

func (qjrService *QueueJobResService) updateService(old, cur interface{}) {

	return
}

func (qjrService *QueueJobResService) deleteService(obj interface{}) {

	return
}

func (qjrService *QueueJobResService) GetAggregatedResourcesByPriority(priority float64, job *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	return total
}

// Parse queue job api object to get Service template
func (qjrService *QueueJobResService) getServiceTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.Service, error) {

	serviceGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("Service")

	obj, _, err := qjrService.jsonSerializer.Decode(qjobRes.Template.Raw, &serviceGVK, nil)
	if err != nil {
		return nil, err
	}

	service, ok := obj.(*v1.Service)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a Service")
	}

	return service, nil

}

func (qjrService *QueueJobResService) createServiceWithControllerRef(namespace string, service *v1.Service, controllerRef *metav1.OwnerReference) error {

	// klog.V(4).Infof("==========create service: %s,  %+v \n", namespace, service)
	if controllerRef != nil {
		service.OwnerReferences = append(service.OwnerReferences, *controllerRef)
	}

	if _, err := qjrService.clients.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrService *QueueJobResService) delService(namespace string, name string) error {

	klog.V(4).Infof("==========delete service: %s,  %s \n", namespace, name)
	if err := qjrService.clients.CoreV1().Services(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrService *QueueJobResService) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

func (qjrService *QueueJobResService) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()

	defer func() {
		// klog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, serviceInQjr, servicesInEtcd, err := qjrService.getServiceForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	serviceLen := len(servicesInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(serviceLen)

	klog.V(4).Infof("QJob: %s had %d Services and %d desired Services", queuejob.Name, serviceLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpService := v1.Service{}
		err = qjrService.refManager.AddReference(qjobRes, &tmpService)
		if err != nil {
			klog.Errorf("Cannot add reference to configmap resource %+v", err)
			return err
		}

		if serviceInQjr.Labels == nil {
			serviceInQjr.Labels = map[string]string{}
		}
		for k, v := range tmpService.Labels {
			serviceInQjr.Labels[k] = v
		}
		serviceInQjr.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()

				err := qjrService.createServiceWithControllerRef(*_namespace, serviceInQjr, metav1.NewControllerRef(queuejob, queueJobKind))

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

func (qjrService *QueueJobResService) getServiceForQueueJobRes(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) (*string, *v1.Service, []*v1.Service, error) {

	// Get "a" Service from AppWrapper Resource
	serviceInQjr, err := qjrService.getServiceTemplate(qjobRes)
	if err != nil {
		klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get Service"s" in Etcd Server
	var _namespace *string
	if serviceInQjr.Namespace != "" {
		_namespace = &serviceInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	serviceList, err := qjrService.clients.CoreV1().Services(*_namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name)})
	if err != nil {
		return nil, nil, nil, err
	}
	servicesInEtcd := []*v1.Service{}
	for i, _ := range serviceList.Items {
		servicesInEtcd = append(servicesInEtcd, &serviceList.Items[i])
	}

	// for i, service := range serviceList.Items {
	// 	metaService, err := meta.Accessor(&service)
	// 	if err != nil {
	// 		return nil, nil, nil, err
	// 	}
	// 	controllerRef := metav1.GetControllerOf(metaService)
	// 	if controllerRef != nil {
	// 		if controllerRef.UID == queuejob.UID {
	// 			servicesInEtcd = append(servicesInEtcd, &serviceList.Items[i])
	// 		}
	// 	}
	// }
	myServicesInEtcd := []*v1.Service{}
	for i, service := range servicesInEtcd {
		if qjrService.refManager.BelongTo(qjobRes, service) {
			myServicesInEtcd = append(myServicesInEtcd, servicesInEtcd[i])
		}
	}

	return _namespace, serviceInQjr, myServicesInEtcd, nil
}

func (qjrService *QueueJobResService) deleteQueueJobResServices(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

	job := *queuejob

	_namespace, _, activeServices, err := qjrService.getServiceForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activeServices))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrService.delService(*_namespace, activeServices[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				klog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeServices[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrService *QueueJobResService) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrService.deleteQueueJobResServices(qjobRes, queuejob)
}
