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
	"context"
	"fmt"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/queuejobresources"

	"sync"
	"time"

	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	ssinformer "k8s.io/client-go/informers/apps/v1"
	sslister "k8s.io/client-go/listers/apps/v1"

	clientset "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned"
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

//QueueJobResStatefulSet - stateful sets
type QueueJobResStatefulSet struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	statefulSetStore sslister.StatefulSetLister
	deployInformer   ssinformer.StatefulSetInformer
	rtScheme         *runtime.Scheme
	jsonSerializer   *json.Serializer
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
func (qjrStatefulSet *QueueJobResStatefulSet) GetPodTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.PodTemplateSpec, int32, error) {
	res, err := qjrStatefulSet.getStatefulSetTemplate(qjobRes)
	if err != nil {
		return nil, -1, err
	}
	return &res.Spec.Template, *res.Spec.Replicas, nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) GetAggregatedResources(queueJob *arbv1.AppWrapper) *clusterstateapi.Resource {
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

func (qjrStatefulSet *QueueJobResStatefulSet) GetAggregatedResourcesByPriority(priority float64, queueJob *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	if queueJob.Spec.AggrResources.Items != nil {
		//calculate scaling
		for _, ar := range queueJob.Spec.AggrResources.Items {
			if ar.Priority < priority {
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

func (qjrStatefulSet *QueueJobResStatefulSet) getStatefulSetTemplate(qjobRes *arbv1.AppWrapperResource) (*apps.StatefulSet, error) {
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
	klog.V(4).Infof("==========create statefulSet: %s,  %+v \n", namespace, statefulSet)
	if controllerRef != nil {
		statefulSet.OwnerReferences = append(statefulSet.OwnerReferences, *controllerRef)
	}
	if _, err := qjrStatefulSet.clients.AppsV1().StatefulSets(namespace).Create(context.Background(), statefulSet, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) delStatefulSet(namespace string, name string) error {

	klog.V(4).Infof("==========delete statefulSet: %s,  %s \n", namespace, name)
	if err := qjrStatefulSet.clients.AppsV1().StatefulSets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

func (qjrStatefulSet *QueueJobResStatefulSet) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()

	defer func() {
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, statefulSetInQjr, statefulSetsInEtcd, err := qjrStatefulSet.getStatefulSetForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	statefulSetLen := len(statefulSetsInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(statefulSetLen)

	klog.V(4).Infof("QJob: %s had %d StatefulSets and %d desired StatefulSets", queuejob.Name, statefulSetLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpStatefulSet := apps.StatefulSet{}
		err = qjrStatefulSet.refManager.AddReference(qjobRes, &tmpStatefulSet)
		if err != nil {
			klog.Errorf("Cannot add reference to configmap resource %+v", err)
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

func (qjrStatefulSet *QueueJobResStatefulSet) getStatefulSetForQueueJobRes(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) (*string, *apps.StatefulSet, []*apps.StatefulSet, error) {

	// Get "a" StatefulSet from AppWrapper Resource
	statefulSetInQjr, err := qjrStatefulSet.getStatefulSetTemplate(qjobRes)
	if err != nil {
		klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get StatefulSet"s" in Etcd Server
	var _namespace *string
	if statefulSetInQjr.Namespace != "" {
		_namespace = &statefulSetInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	statefulSetList, err := qjrStatefulSet.clients.AppsV1().StatefulSets(*_namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name)})
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

func (qjrStatefulSet *QueueJobResStatefulSet) deleteQueueJobResStatefulSets(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

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
				klog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeStatefulSets[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrStatefulSet *QueueJobResStatefulSet) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrStatefulSet.deleteQueueJobResStatefulSets(qjobRes, queuejob)
}
