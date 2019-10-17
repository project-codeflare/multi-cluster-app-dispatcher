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

package persistentvolumeclaim

import (
	"fmt"
	"github.com/golang/glog"
	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"
	//schedulerapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/scheduler/api"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"sync"
	"time"
)

var queueJobKind = arbv1.SchemeGroupVersion.WithKind("XQueueJob")
var queueJobName = "xqueuejob.arbitrator.k8s.io"

const (
	// QueueJobNameLabel label string for queuejob name
	QueueJobNameLabel string = "xqueuejob-name"

	// ControllerUIDLabel label string for queuejob controller uid
	ControllerUIDLabel string = "controller-uid"
)

//QueueJobResService contains service info
type QueueJobResPersistentVolumeClaim struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	persistentvolumeclaimStore    corelisters.PersistentVolumeClaimLister
	persistentvolumeclaimInformer corev1informer.PersistentVolumeClaimInformer
	rtScheme        *runtime.Scheme
	jsonSerializer  *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypePersistentVolumeClaim, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResPersistentVolumeClaim(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResPersistentVolumeClaim(config *rest.Config) queuejobresources.Interface {
	qjrPersistentVolumeClaim := &QueueJobResPersistentVolumeClaim{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrPersistentVolumeClaim.persistentvolumeclaimInformer = informers.NewSharedInformerFactory(qjrPersistentVolumeClaim.clients, 0).Core().V1().PersistentVolumeClaims()
	qjrPersistentVolumeClaim.persistentvolumeclaimInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.PersistentVolumeClaim:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrPersistentVolumeClaim.addPersistentVolumeClaim,
				UpdateFunc: qjrPersistentVolumeClaim.updatePersistentVolumeClaim,
				DeleteFunc: qjrPersistentVolumeClaim.deletePersistentVolumeClaim,
			},
		})

	qjrPersistentVolumeClaim.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrPersistentVolumeClaim.rtScheme)

	qjrPersistentVolumeClaim.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrPersistentVolumeClaim.rtScheme, qjrPersistentVolumeClaim.rtScheme)

	qjrPersistentVolumeClaim.refManager = queuejobresources.NewLabelRefManager()

	return qjrPersistentVolumeClaim
}

// Run the main goroutine responsible for watching and services.
func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) Run(stopCh <-chan struct{}) {

	qjrPersistentVolumeClaim.persistentvolumeclaimInformer.Informer().Run(stopCh)
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) GetAggregatedResources(job *arbv1.XQueueJob) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) addPersistentVolumeClaim(obj interface{}) {

	return
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) updatePersistentVolumeClaim(old, cur interface{}) {

	return
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) deletePersistentVolumeClaim(obj interface{}) {

	return
}


func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) GetAggregatedResourcesByPriority(priority int, job *arbv1.XQueueJob) *clusterstateapi.Resource {
        total := clusterstateapi.EmptyResource()
        return total
}


// Parse queue job api object to get Service template
func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) getPersistentVolumeClaimTemplate(qjobRes *arbv1.XQueueJobResource) (*v1.PersistentVolumeClaim, error) {

	persistentvolumeclaimGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("PersistentVolumeClaim")

	obj, _, err := qjrPersistentVolumeClaim.jsonSerializer.Decode(qjobRes.Template.Raw, &persistentvolumeclaimGVK, nil)
	if err != nil {
		return nil, err
	}

	persistentvolumeclaim, ok := obj.(*v1.PersistentVolumeClaim)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a PersistentVolumeClaim")
	}

	return persistentvolumeclaim, nil

}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) createPersistentVolumeClaimWithControllerRef(namespace string, persistentvolumeclaim *v1.PersistentVolumeClaim, controllerRef *metav1.OwnerReference) error {

	// glog.V(4).Infof("==========create PersistentVolumeClaim: %+v \n", persistentvolumeclaim)
	if controllerRef != nil {
		persistentvolumeclaim.OwnerReferences = append(persistentvolumeclaim.OwnerReferences, *controllerRef)
	}

	if _, err := qjrPersistentVolumeClaim.clients.Core().PersistentVolumeClaims(namespace).Create(persistentvolumeclaim); err != nil {
		return err
	}

	return nil
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) delPersistentVolumeClaim(namespace string, name string) error {

	glog.V(4).Infof("==========delete persistentvolumeclaim: %s \n", name)
	if err := qjrPersistentVolumeClaim.clients.Core().PersistentVolumeClaims(namespace).Delete(name, nil); err != nil {
		return err
	}

	return nil
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) UpdateQueueJobStatus(queuejob *arbv1.XQueueJob) error {
	return nil
}

func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) SyncQueueJob(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {

	startTime := time.Now()

	defer func() {
		// glog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
		glog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, persistentVolumeClaimInQjr, persistentVolumeClaimsInEtcd, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	persistentVolumeClaimLen := len(persistentVolumeClaimsInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(persistentVolumeClaimLen)

	glog.V(4).Infof("QJob: %s had %d PersistVolumeClaims and %d desired PersistVolumeClaims", queuejob.Name, persistentVolumeClaimLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpPersistentVolumeClaim := v1.PersistentVolumeClaim{}
		err = qjrPersistentVolumeClaim.refManager.AddReference(qjobRes, &tmpPersistentVolumeClaim)
		if err != nil {
			glog.Errorf("Cannot add reference to configmap resource %+v", err)
			return err
		}

		if persistentVolumeClaimInQjr.Labels == nil {
			persistentVolumeClaimInQjr.Labels = map[string]string{}
		}
		for k, v := range tmpPersistentVolumeClaim.Labels {
			persistentVolumeClaimInQjr.Labels[k] = v
		}
		persistentVolumeClaimInQjr.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()

				err := qjrPersistentVolumeClaim.createPersistentVolumeClaimWithControllerRef(*_namespace, persistentVolumeClaimInQjr, metav1.NewControllerRef(queuejob, queueJobKind))

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


func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) getPersistentVolumeClaimForQueueJobRes(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) (*string, *v1.PersistentVolumeClaim, []*v1.PersistentVolumeClaim, error) {

	// Get "a" PersistentVolumeClaim from XQJ Resource
	persistentVolumeClaimInQjr, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimTemplate(qjobRes)
	if err != nil {
		glog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get PersistentVolumeClaim"s" in Etcd Server
	var _namespace *string
	if persistentVolumeClaimInQjr.Namespace!=""{
		_namespace = &persistentVolumeClaimInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	persistentVolumeClaimList, err := qjrPersistentVolumeClaim.clients.CoreV1().PersistentVolumeClaims(*_namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name),})
	if err != nil {
		return nil, nil, nil, err
	}
	persistentVolumeClaimsInEtcd := []*v1.PersistentVolumeClaim{}
	for i, _ := range persistentVolumeClaimList.Items {
				persistentVolumeClaimsInEtcd = append(persistentVolumeClaimsInEtcd, &persistentVolumeClaimList.Items[i])
	}

	// for i, persistentVolumeClaim := range persistentVolumeClaimList.Items {
	// 	metaPersistentVolumeClaim, err := meta.Accessor(&persistentVolumeClaim)
	// 	if err != nil {
	// 		return nil, nil, nil, err
	// 	}
	// 	controllerRef := metav1.GetControllerOf(metaPersistentVolumeClaim)
	// 	if controllerRef != nil {
	// 		if controllerRef.UID == queuejob.UID {
	// 			persistentVolumeClaimsInEtcd = append(persistentVolumeClaimsInEtcd, &persistentVolumeClaimList.Items[i])
	// 		}
	// 	}
	// }
	myPersistentVolumeClaimsInEtcd := []*v1.PersistentVolumeClaim{}
	for i, persistentVolumeClaim := range persistentVolumeClaimsInEtcd {
		if qjrPersistentVolumeClaim.refManager.BelongTo(qjobRes, persistentVolumeClaim) {
			myPersistentVolumeClaimsInEtcd = append(myPersistentVolumeClaimsInEtcd, persistentVolumeClaimsInEtcd[i])
		}
	}

	return _namespace, persistentVolumeClaimInQjr, myPersistentVolumeClaimsInEtcd, nil
}


func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) deleteQueueJobResPersistentVolumeClaims(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) error {

	job := *queuejob

	_namespace, _, activePersistentVolumeClaims, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activePersistentVolumeClaims))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrPersistentVolumeClaim.delPersistentVolumeClaim(*_namespace, activePersistentVolumeClaims[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				glog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activePersistentVolumeClaims[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) Cleanup(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {
	return qjrPersistentVolumeClaim.deleteQueueJobResPersistentVolumeClaims(qjobRes, queuejob)
}



// func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) SyncQueueJob(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {
//
// 	startTime := time.Now()
// 	defer func() {
// 		glog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
// 		// glog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
// 	}()
//
// 	persistentvolumeclaims, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimForQueueJobRes(qjobRes, queuejob)
// 	if err != nil {
// 		return err
// 	}
//
// 	persistentvolumeclaimLen := len(persistentvolumeclaims)
// 	replicas := qjobRes.Replicas
//
// 	diff := int(replicas) - int(persistentvolumeclaimLen)
//
// 	glog.V(4).Infof("QJob: %s had %d persistentvolumeclaims and %d desired persistentvolumeclaims", queuejob.Name, replicas, persistentvolumeclaimLen)
//
// 	if diff > 0 {
// 		template, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimTemplate(qjobRes)
// 		if err != nil {
// 			glog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
// 			return err
// 		}
// 		//TODO: need set reference after Service has been really added
// 		tmpPersistentVolumeClaim := v1.PersistentVolumeClaim{}
// 		err = qjrPersistentVolumeClaim.refManager.AddReference(qjobRes, &tmpPersistentVolumeClaim)
// 		if err != nil {
// 			glog.Errorf("Cannot add reference to persistentvolumeclaim resource %+v", err)
// 			return err
// 		}
//
// 		if template.Labels == nil {
// 			template.Labels = map[string]string{}
// 		}
// 		for k, v := range tmpPersistentVolumeClaim.Labels {
// 			template.Labels[k] = v
// 		}
// 		wait := sync.WaitGroup{}
// 		wait.Add(int(diff))
// 		for i := 0; i < diff; i++ {
// 			go func() {
// 				defer wait.Done()
// 				_namespace:=""
// 				if template.Namespace!=""{
// 					_namespace=template.Namespace
// 				} else {
// 					_namespace=queuejob.Namespace
// 				}
// 				err := qjrPersistentVolumeClaim.createPersistentVolumeClaimWithControllerRef(_namespace, template, metav1.NewControllerRef(queuejob, queueJobKind))
// 				if err != nil && errors.IsTimeout(err) {
// 					return
// 				}
// 				if err != nil {
// 					defer utilruntime.HandleError(err)
// 				}
// 			}()
// 		}
// 		wait.Wait()
// 	}
//
// 	return nil
// }
//
// func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) getPersistentVolumeClaimForQueueJob(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) ([]*v1.PersistentVolumeClaim, error) {
//
// 	template, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimTemplate(qjobRes)
// 	if err != nil {
// 		glog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
// 		return nil, err
// 	}
//
// 	_namespace:=""
// 	if template.Namespace!=""{
// 		_namespace=template.Namespace
// 	} else {
// 		_namespace=queuejob.Namespace
// 	}
//
// 	persistentvolumeclaimlist, err := qjrPersistentVolumeClaim.clients.CoreV1().PersistentVolumeClaims(_namespace).List(metav1.ListOptions{})
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	persistentvolumeclaims := []*v1.PersistentVolumeClaim{}
// 	for i, persistentvolumeclaim := range persistentvolumeclaimlist.Items {
// 		metaPersistentVolumeClaim, err := meta.Accessor(&persistentvolumeclaim)
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		controllerRef := metav1.GetControllerOf(metaPersistentVolumeClaim)
// 		if controllerRef != nil {
// 			if controllerRef.UID == queuejob.UID {
// 				persistentvolumeclaims = append(persistentvolumeclaims, &persistentvolumeclaimlist.Items[i])
// 			}
// 		}
// 	}
// 	return persistentvolumeclaims, nil
//
// }
//
// func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) getPersistentVolumeClaimForQueueJobRes(qjobRes *arbv1.XQueueJobResource, j *arbv1.XQueueJob) ([]*v1.PersistentVolumeClaim, error) {
//
// 	persistentvolumeclaims, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimForQueueJob(qjobRes, j)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	myPersistentVolumeClaims := []*v1.PersistentVolumeClaim{}
// 	for i, persistentvolumeclaim := range persistentvolumeclaims {
// 		if qjrPersistentVolumeClaim.refManager.BelongTo(qjobRes, persistentvolumeclaim) {
// 			myPersistentVolumeClaims = append(myPersistentVolumeClaims, persistentvolumeclaims[i])
// 		}
// 	}
//
// 	return myPersistentVolumeClaims, nil
//
// }
//
// func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) deleteQueueJobResPersistentVolumeClaims(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) error {
//
// 	job := *queuejob
//
// 	activePersistentVolumeClaims, err := qjrPersistentVolumeClaim.getPersistentVolumeClaimForQueueJobRes(qjobRes, queuejob)
// 	if err != nil {
// 		return err
// 	}
//
// 	active := int32(len(activePersistentVolumeClaims))
//
// 	wait := sync.WaitGroup{}
// 	wait.Add(int(active))
// 	for i := int32(0); i < active; i++ {
// 		go func(ix int32) {
// 			defer wait.Done()
// 			if err := qjrPersistentVolumeClaim.delPersistentVolumeClaim(queuejob.Namespace, activePersistentVolumeClaims[ix].Name); err != nil {
// 				defer utilruntime.HandleError(err)
// 				glog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activePersistentVolumeClaims[ix].Name, job.Namespace, job.Name)
// 			}
// 		}(i)
// 	}
// 	wait.Wait()
//
// 	return nil
// }
//
// //Cleanup deletes all services
// func (qjrPersistentVolumeClaim *QueueJobResPersistentVolumeClaim) Cleanup(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {
// 	return qjrPersistentVolumeClaim.deleteQueueJobResPersistentVolumeClaims(qjobRes, queuejob)
// }
