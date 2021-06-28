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

package configmap

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
type QueueJobResConfigMap struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	configmapStore    corelisters.ConfigMapLister
	configmapInformer corev1informer.ConfigMapInformer
	rtScheme          *runtime.Scheme
	jsonSerializer    *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypeConfigMap, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResConfigMap(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResConfigMap(config *rest.Config) queuejobresources.Interface {
	qjrConfigMap := &QueueJobResConfigMap{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrConfigMap.configmapInformer = informers.NewSharedInformerFactory(qjrConfigMap.clients, 0).Core().V1().ConfigMaps()
	qjrConfigMap.configmapInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.ConfigMap:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrConfigMap.addConfigMap,
				UpdateFunc: qjrConfigMap.updateConfigMap,
				DeleteFunc: qjrConfigMap.deleteConfigMap,
			},
		})

	qjrConfigMap.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrConfigMap.rtScheme)

	qjrConfigMap.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrConfigMap.rtScheme, qjrConfigMap.rtScheme)

	qjrConfigMap.refManager = queuejobresources.NewLabelRefManager()

	return qjrConfigMap
}

// Run the main goroutine responsible for watching and services.
func (qjrConfigMap *QueueJobResConfigMap) Run(stopCh <-chan struct{}) {

	qjrConfigMap.configmapInformer.Informer().Run(stopCh)
}

func (qjrConfigMap *QueueJobResConfigMap) GetAggregatedResources(job *arbv1.AppWrapper) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrConfigMap *QueueJobResConfigMap) addConfigMap(obj interface{}) {

	return
}

func (qjrConfigMap *QueueJobResConfigMap) updateConfigMap(old, cur interface{}) {

	return
}

func (qjrConfigMap *QueueJobResConfigMap) deleteConfigMap(obj interface{}) {

	return
}

func (qjrConfigMap *QueueJobResConfigMap) GetAggregatedResourcesByPriority(priority float64, job *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	return total
}

// Parse queue job api object to get Service template
func (qjrConfigMap *QueueJobResConfigMap) getConfigMapTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.ConfigMap, error) {

	configmapGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("ConfigMap")

	obj, _, err := qjrConfigMap.jsonSerializer.Decode(qjobRes.Template.Raw, &configmapGVK, nil)
	if err != nil {
		return nil, err
	}

	configmap, ok := obj.(*v1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a ConfigMap")
	}

	return configmap, nil

}

func (qjrConfigMap *QueueJobResConfigMap) createConfigMapWithControllerRef(namespace string, configmap *v1.ConfigMap, controllerRef *metav1.OwnerReference) error {
	if controllerRef != nil {
		configmap.OwnerReferences = append(configmap.OwnerReferences, *controllerRef)
	}

	if _, err := qjrConfigMap.clients.CoreV1().ConfigMaps(namespace).Create(context.Background(), configmap, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrConfigMap *QueueJobResConfigMap) delConfigMap(namespace string, name string) error {
	if err := qjrConfigMap.clients.CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrConfigMap *QueueJobResConfigMap) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

func (qjrConfigMap *QueueJobResConfigMap) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()

	defer func() {
		// klog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, configMapInQjr, configMapsInEtcd, err := qjrConfigMap.getConfigMapForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	configMapLen := len(configMapsInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(configMapLen)

	klog.V(4).Infof("QJob: %s had %d configMaps and %d desired configMaps", queuejob.Name, configMapLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpConfigMap := v1.ConfigMap{}
		err = qjrConfigMap.refManager.AddReference(qjobRes, &tmpConfigMap)
		if err != nil {
			klog.Errorf("Cannot add reference to configmap resource %+v", err)
			return err
		}
		if configMapInQjr.Labels == nil {
			configMapInQjr.Labels = map[string]string{}
		}
		for k, v := range tmpConfigMap.Labels {
			configMapInQjr.Labels[k] = v
		}
		configMapInQjr.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()

				err := qjrConfigMap.createConfigMapWithControllerRef(*_namespace, configMapInQjr, metav1.NewControllerRef(queuejob, queueJobKind))

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

func (qjrConfigMap *QueueJobResConfigMap) getConfigMapForQueueJobRes(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) (*string, *v1.ConfigMap, []*v1.ConfigMap, error) {

	// Get "a" ConfigMap from AppWrapper Resource
	configMapInQjr, err := qjrConfigMap.getConfigMapTemplate(qjobRes)
	if err != nil {
		klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get ConfigMap"s" in Etcd Server
	var _namespace *string
	if configMapInQjr.Namespace != "" {
		_namespace = &configMapInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	configMapList, err := qjrConfigMap.clients.CoreV1().ConfigMaps(*_namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name)})
	// configMapList, err := qjrConfigMap.clients.CoreV1().ConfigMaps(*_namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	configMapsInEtcd := []*v1.ConfigMap{}
	for i, _ := range configMapList.Items {
		configMapsInEtcd = append(configMapsInEtcd, &configMapList.Items[i])
	}

	myConfigMapsInEtcd := []*v1.ConfigMap{}
	for i, configMap := range configMapsInEtcd {
		if qjrConfigMap.refManager.BelongTo(qjobRes, configMap) {
			myConfigMapsInEtcd = append(myConfigMapsInEtcd, configMapsInEtcd[i])
		}
	}

	return _namespace, configMapInQjr, myConfigMapsInEtcd, nil
}

func (qjrConfigMap *QueueJobResConfigMap) deleteQueueJobResConfigMaps(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

	job := *queuejob

	_namespace, _, activeConfigMaps, err := qjrConfigMap.getConfigMapForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activeConfigMaps))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrConfigMap.delConfigMap(*_namespace, activeConfigMaps[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				klog.V(2).Infof("Failed to delete %v, application wrapper %q/%q deadline exceeded", activeConfigMaps[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrConfigMap *QueueJobResConfigMap) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrConfigMap.deleteQueueJobResConfigMaps(qjobRes, queuejob)
}
