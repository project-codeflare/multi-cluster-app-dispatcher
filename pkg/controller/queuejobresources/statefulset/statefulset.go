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

package statefulset

import (
	"fmt"
	"github.com/golang/glog"
	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"
	//schedulerapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/scheduler/api"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"

	ssinformer "k8s.io/client-go/informers/apps/v1"
	sslister "k8s.io/client-go/listers/apps/v1"

	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var queueJobKind = arbv1.SchemeGroupVersion.WithKind("XQueueJob")
var queueJobName = "xqueuejob.arbitrator.k8s.io"

const (
	// QueueJobNameLabel label string for queuejob name
	QueueJobNameLabel string = "xqueuejob-name"

	// ControllerUIDLabel label string for queuejob controller uid
	ControllerUIDLabel string = "controller-uid"
)

//QueueJobResStatefulSet - stateful sets
type QueueJobResStatefulSet struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	statefulSetStore   sslister.StatefulSetLister
	deployInformer ssinformer.StatefulSetInformer
	rtScheme       *runtime.Scheme
	jsonSerializer *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

// Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypeStatefulSet, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResStatefulSet(config)
	})
}

//NewQueueJobResStatefulSet - creates a controller for SS
func NewQueueJobResStatefulSet(config *rest.Config) queuejobresources.Interface {
	qjrd := &QueueJobResStatefulSet{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrd.deployInformer = informers.NewSharedInformerFactory(qjrd.clients, 0).Apps().V1().StatefulSets()
	qjrd.deployInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *apps.StatefulSet:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrd.addStatefulSet,
				UpdateFunc: qjrd.updateStatefulSet,
				DeleteFunc: qjrd.deleteStatefulSet,
			},
		})

	qjrd.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrd.rtScheme)
	// v1beta1.AddToScheme(qjrd.rtScheme)
	apps.AddToScheme(qjrd.rtScheme)
	qjrd.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrd.rtScheme, qjrd.rtScheme)

	qjrd.refManager = queuejobresources.NewLabelRefManager()

	return qjrd
}

// Run the main goroutine responsible for watching and services.
func (qjrStatefulSet *QueueJobResStatefulSet) Run(stopCh <-chan struct{}) {
	qjrStatefulSet.deployInformer.Informer().Run(stopCh)
}

//GetPodTemplate Parse queue job api object to get Pod template
func (qjrStatefulSet *QueueJobResStatefulSet) GetPodTemplate(qjobRes *arbv1.XQueueJobResource) (*v1.PodTemplateSpec, int32, error) {
	res, err := qjrStatefulSet.getStatefulSetTemplate(qjobRes)
	if err != nil {
		return nil, -1, err
	}
  return &res.Spec.Template, *res.Spec.Replicas, nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) GetAggregatedResources(queueJob *arbv1.XQueueJob) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	if queueJob.Spec.AggrResources.Items != nil {
		//calculate scaling
		for _, ar := range queueJob.Spec.AggrResources.Items {
			if ar.Type == arbv1.ResourceTypeStatefulSet {
				podTemplate, replicas, _ := qjrStatefulSet.GetPodTemplate(&ar)
				myres := queuejobresources.GetPodResources(podTemplate)
				myres.MilliCPU = float64(replicas) * myres.MilliCPU
				myres.Memory = float64(replicas) * myres.Memory
				myres.GPU = int64(replicas) * myres.GPU
				total = total.Add(myres)
			}
		}
	}
	return total
}

func (qjrStatefulSet *QueueJobResStatefulSet) GetAggregatedResourcesByPriority(priority int, queueJob *arbv1.XQueueJob) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	if queueJob.Spec.AggrResources.Items != nil {
		//calculate scaling
		for _, ar := range queueJob.Spec.AggrResources.Items {
			if ar.Priority < float64(priority) {
				continue
			}
			if ar.Type == arbv1.ResourceTypeStatefulSet {
				podTemplate, replicas, _ := qjrStatefulSet.GetPodTemplate(&ar)
				myres := queuejobresources.GetPodResources(podTemplate)
				myres.MilliCPU = float64(replicas) * myres.MilliCPU
				myres.Memory = float64(replicas) * myres.Memory
				myres.GPU = int64(replicas) * myres.GPU
				total = total.Add(myres)
			}
		}
	}
	return total
}

func (qjrStatefulSet *QueueJobResStatefulSet) addStatefulSet(obj interface{}) {

	return
}

func (qjrStatefulSet *QueueJobResStatefulSet) updateStatefulSet(old, cur interface{}) {

	return
}

func (qjrStatefulSet *QueueJobResStatefulSet) deleteStatefulSet(obj interface{}) {

	return
}

func (qjrStatefulSet *QueueJobResStatefulSet) getStatefulSetTemplate(qjobRes *arbv1.XQueueJobResource) (*apps.StatefulSet, error) {
	statefulSetGVK := schema.GroupVersion{Group: "", Version: "v1"}.WithKind("StatefulSet")
	obj, _, err := qjrStatefulSet.jsonSerializer.Decode(qjobRes.Template.Raw, &statefulSetGVK, nil)
	if err != nil {
		return nil, err
	}
	statefulSet, ok := obj.(*apps.StatefulSet)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a StatefulSet")
	}
	return statefulSet, nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) createStatefulSetWithControllerRef(namespace string, statefulSet *apps.StatefulSet, controllerRef *metav1.OwnerReference) error {
	glog.V(4).Infof("==========create statefulSet: %s,  %+v \n", namespace, statefulSet)
	if controllerRef != nil {
		statefulSet.OwnerReferences = append(statefulSet.OwnerReferences, *controllerRef)
	}
	if _, err := qjrStatefulSet.clients.AppsV1().StatefulSets(namespace).Create(statefulSet); err != nil {
		return err
	}
	return nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) delStatefulSet(namespace string, name string) error {

	glog.V(4).Infof("==========delete statefulSet: %s,  %s \n", namespace, name)
	if err := qjrStatefulSet.clients.AppsV1().StatefulSets(namespace).Delete(name, nil); err != nil {
		return err
	}
	return nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) UpdateQueueJobStatus(queuejob *arbv1.XQueueJob) error {
	return nil
}



func (qjrStatefulSet *QueueJobResStatefulSet) SyncQueueJob(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {

	startTime := time.Now()

	defer func() {
		// glog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
		glog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, statefulSetInQjr, statefulSetsInEtcd, err := qjrStatefulSet.getStatefulSetForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	statefulSetLen := len(statefulSetsInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(statefulSetLen)

	glog.V(4).Infof("QJob: %s had %d StatefulSets and %d desired StatefulSets", queuejob.Name, statefulSetLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpStatefulSet := apps.StatefulSet{}
		err = qjrStatefulSet.refManager.AddReference(qjobRes, &tmpStatefulSet)
		if err != nil {
			glog.Errorf("Cannot add reference to configmap resource %+v", err)
			return err
		}
		if statefulSetInQjr.Labels == nil {
			statefulSetInQjr.Labels = map[string]string{}
		}
		for k, v := range tmpStatefulSet.Labels {
			statefulSetInQjr.Labels[k] = v
		}
		statefulSetInQjr.Labels[queueJobName] = queuejob.Name
    if statefulSetInQjr.Spec.Template.Labels == nil {
            statefulSetInQjr.Labels = map[string]string{}
    }
    statefulSetInQjr.Spec.Template.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()

				err := qjrStatefulSet.createStatefulSetWithControllerRef(*_namespace, statefulSetInQjr, metav1.NewControllerRef(queuejob, queueJobKind))

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


func (qjrStatefulSet *QueueJobResStatefulSet) getStatefulSetForQueueJobRes(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) (*string, *apps.StatefulSet, []*apps.StatefulSet, error) {

	// Get "a" StatefulSet from XQJ Resource
	statefulSetInQjr, err := qjrStatefulSet.getStatefulSetTemplate(qjobRes)
	if err != nil {
		glog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get StatefulSet"s" in Etcd Server
	var _namespace *string
	if statefulSetInQjr.Namespace!=""{
		_namespace = &statefulSetInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	// statefulSetList, err := qjrStatefulSet.clients.CoreV1().StatefulSets(*_namespace).List(metav1.ListOptions{})
	statefulSetList, err := qjrStatefulSet.clients.AppsV1().StatefulSets(*_namespace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name),})
	// statefulSetList, err := qjrStatefulSet.clients.AppsV1().StatefulSets(*_namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}
	statefulSetsInEtcd := []*apps.StatefulSet{}
	// for i, statefulSet := range statefulSetList.Items {
	// 	metaStatefulSet, err := meta.Accessor(&statefulSet)
	// 	if err != nil {
	// 		return nil, nil, nil, err
	// 	}
	// 	controllerRef := metav1.GetControllerOf(metaStatefulSet)
	// 	if controllerRef != nil {
	// 		if controllerRef.UID == queuejob.UID {
	// 			statefulSetsInEtcd = append(statefulSetsInEtcd, &statefulSetList.Items[i])
	// 		}
	// 	}
	// }
	for i, _ := range statefulSetList.Items {
				statefulSetsInEtcd = append(statefulSetsInEtcd, &statefulSetList.Items[i])
	}

	myStatefulSetsInEtcd := []*apps.StatefulSet{}
	for i, statefulSet := range statefulSetsInEtcd {
		if qjrStatefulSet.refManager.BelongTo(qjobRes, statefulSet) {
			myStatefulSetsInEtcd = append(myStatefulSetsInEtcd, statefulSetsInEtcd[i])
		}
	}

	return _namespace, statefulSetInQjr, myStatefulSetsInEtcd, nil
}


func (qjrStatefulSet *QueueJobResStatefulSet) deleteQueueJobResStatefulSets(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) error {

	job := *queuejob

	_namespace, _, activeStatefulSets, err := qjrStatefulSet.getStatefulSetForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activeStatefulSets))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrStatefulSet.delStatefulSet(*_namespace, activeStatefulSets[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				glog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeStatefulSets[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrStatefulSet *QueueJobResStatefulSet) Cleanup(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {
	return qjrStatefulSet.deleteQueueJobResStatefulSets(qjobRes, queuejob)
}



// //SyncQueueJob - syncs the resources of the queuejob
// func (qjrStatefulSet *QueueJobResStatefulSet) SyncQueueJob(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {
//
// 	startTime := time.Now()
// 	defer func() {
// 		glog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
// 	}()
//
// 	statefulSet, err := qjrStatefulSet.getStatefulSetsForQueueJobRes(qjobRes, queuejob)
// 	if err != nil {
// 		return err
// 	}
//
// 	statefulSetLen := len(statefulSet)
// 	replicas := qjobRes.Replicas
//
// 	diff := int(replicas) - int(statefulSetLen)
//
// 	glog.V(4).Infof("QJob: %s had %d statefulSet and %d desired statefulSet", queuejob.Name, replicas, statefulSetLen)
//
// 	if diff > 0 {
// 		template, err := qjrStatefulSet.getStatefulSetTemplate(qjobRes)
// 		if err != nil {
// 			glog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
// 			return err
// 		}
//
// 		// tmpStatefulSet := v1.Service{}
// 		tmpStatefulSet := apps.StatefulSet{}
// 		err = qjrStatefulSet.refManager.AddReference(qjobRes, &tmpStatefulSet)
// 		if err != nil {
// 			glog.Errorf("Cannot add reference to statefulSet resource %+v", err)
// 			return err
// 		}
//
// 		if template.Labels == nil {
// 			template.Labels = map[string]string{}
// 		}
// 		for k, v := range tmpStatefulSet.Labels {
// 			template.Labels[k] = v
// 		}
//
// 		template.Labels[queueJobName] = queuejob.Name
//                 if template.Spec.Template.Labels == nil {
//                         template.Labels = map[string]string{}
//                 }
//                 template.Spec.Template.Labels[queueJobName] = queuejob.Name
//
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
// 				err := qjrStatefulSet.createStatefulSetWithControllerRef(_namespace, template, metav1.NewControllerRef(queuejob, queueJobKind))
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
// func (qjrStatefulSet *QueueJobResStatefulSet) getStatefulSetsForQueueJob(j *arbv1.XQueueJob) ([]*apps.StatefulSet, error) {
// 	statefulSetlist, err := qjrStatefulSet.clients.AppsV1().StatefulSets(j.Namespace).List(
// 									metav1.ListOptions{
//                  					               LabelSelector: fmt.Sprintf("%s=%s", queueJobName, j.Name),
//                 						})
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	statefulSets := []*apps.StatefulSet{}
// 	for _, statefulSet := range statefulSetlist.Items {
// 				statefulSets = append(statefulSets, &statefulSet)
// 	}
// 	return statefulSets, nil
//
// }
//
// func (qjrStatefulSet *QueueJobResStatefulSet) getStatefulSetsForQueueJobRes(qjobRes *arbv1.XQueueJobResource, j *arbv1.XQueueJob) ([]*apps.StatefulSet, error) {
//
// 	statefulSets, err := qjrStatefulSet.getStatefulSetsForQueueJob(j)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	myStatefulSets := []*apps.StatefulSet{}
// 	for i, statefulSet := range statefulSets {
// 		if qjrStatefulSet.refManager.BelongTo(qjobRes, statefulSet) {
// 			myStatefulSets = append(myStatefulSets, statefulSets[i])
// 		}
// 	}
//
// 	return myStatefulSets, nil
//
// }
//
// func (qjrStatefulSet *QueueJobResStatefulSet) deleteQueueJobResStatefulSet(qjobRes *arbv1.XQueueJobResource, queuejob *arbv1.XQueueJob) error {
// 	job := *queuejob
//
// 	activeStatefulSets, err := qjrStatefulSet.getStatefulSetsForQueueJob(queuejob)
// 	if err != nil {
// 		return err
// 	}
//
// 	active := int32(len(activeStatefulSets))
//
// 	wait := sync.WaitGroup{}
// 	wait.Add(int(active))
// 	for i := int32(0); i < active; i++ {
// 		go func(ix int32) {
// 			defer wait.Done()
// 			if err := qjrStatefulSet.delStatefulSet(queuejob.Namespace, activeStatefulSets[ix].Name); err != nil {
// 				defer utilruntime.HandleError(err)
// 				glog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeStatefulSets[ix].Name, job.Namespace, job.Name)
// 			}
// 		}(i)
// 	}
// 	wait.Wait()
//
// 	return nil
// }
//
// //Cleanup - cleans the resources
// func (qjrStatefulSet *QueueJobResStatefulSet) Cleanup(queuejob *arbv1.XQueueJob, qjobRes *arbv1.XQueueJobResource) error {
// 	return qjrStatefulSet.deleteQueueJobResStatefulSet(qjobRes, queuejob)
// }
