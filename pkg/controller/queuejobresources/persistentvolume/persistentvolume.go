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

package persistentvolume

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
type QueueJobResPersistentvolume struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	persistentvolumeStore    corelisters.PersistentVolumeLister
	persistentvolumeInformer corev1informer.PersistentVolumeInformer
	rtScheme                 *runtime.Scheme
	jsonSerializer           *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypePersistentVolume, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResPersistentvolume(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResPersistentvolume(config *rest.Config) queuejobresources.Interface {
	qjrPersistentvolume := &QueueJobResPersistentvolume{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrPersistentvolume.persistentvolumeInformer = informers.NewSharedInformerFactory(qjrPersistentvolume.clients, 0).Core().V1().PersistentVolumes()
	qjrPersistentvolume.persistentvolumeInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.PersistentVolume:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrPersistentvolume.addPersistentVolume,
				UpdateFunc: qjrPersistentvolume.updatePersistentVolume,
				DeleteFunc: qjrPersistentvolume.deletePersistentVolume,
			},
		})

	qjrPersistentvolume.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrPersistentvolume.rtScheme)

	qjrPersistentvolume.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrPersistentvolume.rtScheme, qjrPersistentvolume.rtScheme)

	qjrPersistentvolume.refManager = queuejobresources.NewLabelRefManager()

	return qjrPersistentvolume
}

// Run the main goroutine responsible for watching and services.
func (qjrPersistentvolume *QueueJobResPersistentvolume) Run(stopCh <-chan struct{}) {

	qjrPersistentvolume.persistentvolumeInformer.Informer().Run(stopCh)
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) GetAggregatedResources(job *arbv1.AppWrapper) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) addPersistentVolume(obj interface{}) {

	return
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) updatePersistentVolume(old, cur interface{}) {

	return
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) deletePersistentVolume(obj interface{}) {

	return
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) GetAggregatedResourcesByPriority(priority float64, job *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	return total
}

// Parse queue job api object to get Service template
func (qjrPersistentvolume *QueueJobResPersistentvolume) getPersistentVolumeTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.PersistentVolume, error) {

	persistentvolumeGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("PersistentVolume")

	obj, _, err := qjrPersistentvolume.jsonSerializer.Decode(qjobRes.Template.Raw, &persistentvolumeGVK, nil)
	if err != nil {
		return nil, err
	}

	persistentvolume, ok := obj.(*v1.PersistentVolume)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a PersistentVolume")
	}

	return persistentvolume, nil

}

func (qjrPersistentvolume *QueueJobResPersistentvolume) createPersistentVolumeWithControllerRef(persistentvolume *v1.PersistentVolume, controllerRef *metav1.OwnerReference) error {

	// klog.V(4).Infof("==========create PersistentVolume: %+v \n", persistentvolume)
	if controllerRef != nil {
		persistentvolume.OwnerReferences = append(persistentvolume.OwnerReferences, *controllerRef)
	}

	if _, err := qjrPersistentvolume.clients.CoreV1().PersistentVolumes().Create(context.Background(), persistentvolume, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) delPersistentVolume(name string) error {

	klog.V(4).Infof("==========delete persistentvolume: %s \n", name)
	if err := qjrPersistentvolume.clients.CoreV1().PersistentVolumes().Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrPersistentvolume *QueueJobResPersistentvolume) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

//SyncQueueJob syncs the services
func (qjrPersistentvolume *QueueJobResPersistentvolume) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
		// klog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
	}()

	persistentvolumes, err := qjrPersistentvolume.getPersistentVolumeForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	persistentvolumeLen := len(persistentvolumes)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(persistentvolumeLen)

	klog.V(4).Infof("QJob: %s had %d persistentvolumes and %d desired persistentvolumes", queuejob.Name, persistentvolumeLen, replicas)

	if diff > 0 {
		template, err := qjrPersistentvolume.getPersistentVolumeTemplate(qjobRes)
		if err != nil {
			klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
			return err
		}
		//TODO: need set reference after Service has been really added
		tmpPersistentVolume := v1.PersistentVolume{}
		err = qjrPersistentvolume.refManager.AddReference(qjobRes, &tmpPersistentVolume)
		if err != nil {
			klog.Errorf("Cannot add reference to persistentvolume resource %+v", err)
			return err
		}

		if template.Labels == nil {
			template.Labels = map[string]string{}
		}
		for k, v := range tmpPersistentVolume.Labels {
			template.Labels[k] = v
		}
		template.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()
				err := qjrPersistentvolume.createPersistentVolumeWithControllerRef(template, metav1.NewControllerRef(queuejob, queueJobKind))
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

func (qjrPersistentvolume *QueueJobResPersistentvolume) getPersistentVolumeForQueueJob(j *arbv1.AppWrapper) ([]*v1.PersistentVolume, error) {
	persistentvolumelist, err := qjrPersistentvolume.clients.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, j.Name)})
	if err != nil {
		return nil, err
	}

	persistentvolumes := []*v1.PersistentVolume{}
	for i, _ := range persistentvolumelist.Items {
		persistentvolumes = append(persistentvolumes, &persistentvolumelist.Items[i])
	}

	// for i, persistentvolume := range persistentvolumelist.Items {
	// 	metaPersistentVolume, err := meta.Accessor(&persistentvolume)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	// 	controllerRef := metav1.GetControllerOf(metaPersistentVolume)
	// 	if controllerRef != nil {
	// 		if controllerRef.UID == j.UID {
	// 			persistentvolumes = append(persistentvolumes, &persistentvolumelist.Items[i])
	// 		}
	// 	}
	// }
	return persistentvolumes, nil

}

func (qjrPersistentvolume *QueueJobResPersistentvolume) getPersistentVolumeForQueueJobRes(qjobRes *arbv1.AppWrapperResource, j *arbv1.AppWrapper) ([]*v1.PersistentVolume, error) {

	persistentvolumes, err := qjrPersistentvolume.getPersistentVolumeForQueueJob(j)
	if err != nil {
		return nil, err
	}

	myPersistentVolumes := []*v1.PersistentVolume{}
	for i, persistentvolume := range persistentvolumes {
		if qjrPersistentvolume.refManager.BelongTo(qjobRes, persistentvolume) {
			myPersistentVolumes = append(myPersistentVolumes, persistentvolumes[i])
		}
	}

	return myPersistentVolumes, nil

}

func (qjrPersistentvolume *QueueJobResPersistentvolume) deleteQueueJobResPersistentVolumes(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

	job := *queuejob

	activePersistentVolumes, err := qjrPersistentvolume.getPersistentVolumeForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activePersistentVolumes))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrPersistentvolume.delPersistentVolume(activePersistentVolumes[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				klog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activePersistentVolumes[ix].Name, job.Namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrPersistentvolume *QueueJobResPersistentvolume) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrPersistentvolume.deleteQueueJobResPersistentVolumes(qjobRes, queuejob)
}
