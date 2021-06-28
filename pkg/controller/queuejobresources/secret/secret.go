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

package secret

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
type QueueJobResSecret struct {
	clients    *kubernetes.Clientset
	arbclients *clientset.Clientset
	// A store of services, populated by the serviceController
	secretStore    corelisters.SecretLister
	secretInformer corev1informer.SecretInformer
	rtScheme       *runtime.Scheme
	jsonSerializer *json.Serializer
	// Reference manager to manage membership of queuejob resource and its members
	refManager queuejobresources.RefManager
}

//Register registers a queue job resource type
func Register(regs *queuejobresources.RegisteredResources) {
	regs.Register(arbv1.ResourceTypeSecret, func(config *rest.Config) queuejobresources.Interface {
		return NewQueueJobResSecret(config)
	})
}

//NewQueueJobResService creates a service controller
func NewQueueJobResSecret(config *rest.Config) queuejobresources.Interface {
	qjrSecret := &QueueJobResSecret{
		clients:    kubernetes.NewForConfigOrDie(config),
		arbclients: clientset.NewForConfigOrDie(config),
	}

	qjrSecret.secretInformer = informers.NewSharedInformerFactory(qjrSecret.clients, 0).Core().V1().Secrets()
	qjrSecret.secretInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch obj.(type) {
				case *v1.Secret:
					return true
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    qjrSecret.addSecret,
				UpdateFunc: qjrSecret.updateSecret,
				DeleteFunc: qjrSecret.deleteSecret,
			},
		})

	qjrSecret.rtScheme = runtime.NewScheme()
	v1.AddToScheme(qjrSecret.rtScheme)

	qjrSecret.jsonSerializer = json.NewYAMLSerializer(json.DefaultMetaFactory, qjrSecret.rtScheme, qjrSecret.rtScheme)

	qjrSecret.refManager = queuejobresources.NewLabelRefManager()

	return qjrSecret
}

// Run the main goroutine responsible for watching and services.
func (qjrSecret *QueueJobResSecret) Run(stopCh <-chan struct{}) {

	qjrSecret.secretInformer.Informer().Run(stopCh)
}

func (qjrSecret *QueueJobResSecret) GetAggregatedResources(job *arbv1.AppWrapper) *clusterstateapi.Resource {
	return clusterstateapi.EmptyResource()
}

func (qjrSecret *QueueJobResSecret) addSecret(obj interface{}) {

	return
}

func (qjrSecret *QueueJobResSecret) updateSecret(old, cur interface{}) {

	return
}

func (qjrSecret *QueueJobResSecret) deleteSecret(obj interface{}) {

	return
}

func (qjrSecret *QueueJobResSecret) GetAggregatedResourcesByPriority(priority float64, job *arbv1.AppWrapper) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	return total
}

// Parse queue job api object to get Service template
func (qjrSecret *QueueJobResSecret) getSecretTemplate(qjobRes *arbv1.AppWrapperResource) (*v1.Secret, error) {

	secretGVK := schema.GroupVersion{Group: v1.GroupName, Version: "v1"}.WithKind("Secret")

	obj, _, err := qjrSecret.jsonSerializer.Decode(qjobRes.Template.Raw, &secretGVK, nil)
	if err != nil {
		return nil, err
	}

	secret, ok := obj.(*v1.Secret)
	if !ok {
		return nil, fmt.Errorf("Queuejob resource not defined as a Secret")
	}

	return secret, nil

}

func (qjrSecret *QueueJobResSecret) createSecretWithControllerRef(namespace string, secret *v1.Secret, controllerRef *metav1.OwnerReference) error {

	if controllerRef != nil {
		secret.OwnerReferences = append(secret.OwnerReferences, *controllerRef)
	}

	if _, err := qjrSecret.clients.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrSecret *QueueJobResSecret) delSecret(namespace string, name string) error {

	klog.V(4).Infof("==========delete secret: %s \n", name)
	if err := qjrSecret.clients.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	return nil
}

func (qjrSecret *QueueJobResSecret) UpdateQueueJobStatus(queuejob *arbv1.AppWrapper) error {
	return nil
}

func (qjrSecret *QueueJobResSecret) SyncQueueJob(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {

	startTime := time.Now()

	defer func() {
		// klog.V(4).Infof("Finished syncing queue job resource %q (%v)", qjobRes.Template, time.Now().Sub(startTime))
		klog.V(4).Infof("Finished syncing queue job resource %s (%v)", queuejob.Name, time.Now().Sub(startTime))
	}()

	_namespace, secretInQjr, secretsInEtcd, err := qjrSecret.getSecretForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	secretLen := len(secretsInEtcd)
	replicas := qjobRes.Replicas

	diff := int(replicas) - int(secretLen)

	klog.V(4).Infof("QJob: %s had %d Secrets and %d desired Secrets", queuejob.Name, secretLen, replicas)

	if diff > 0 {
		//TODO: need set reference after Service has been really added
		tmpSecret := v1.Secret{}
		err = qjrSecret.refManager.AddReference(qjobRes, &tmpSecret)
		if err != nil {
			klog.Errorf("Cannot add reference to configmap resource %+v", err)
			return err
		}

		if secretInQjr.Labels == nil {
			secretInQjr.Labels = map[string]string{}
		}
		for k, v := range tmpSecret.Labels {
			secretInQjr.Labels[k] = v
		}
		secretInQjr.Labels[queueJobName] = queuejob.Name

		wait := sync.WaitGroup{}
		wait.Add(int(diff))
		for i := 0; i < diff; i++ {
			go func() {
				defer wait.Done()

				err := qjrSecret.createSecretWithControllerRef(*_namespace, secretInQjr, metav1.NewControllerRef(queuejob, queueJobKind))

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

func (qjrSecret *QueueJobResSecret) getSecretForQueueJobRes(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) (*string, *v1.Secret, []*v1.Secret, error) {

	// Get "a" Secret from AppWrapper Resource
	secretInQjr, err := qjrSecret.getSecretTemplate(qjobRes)
	if err != nil {
		klog.Errorf("Cannot read template from resource %+v %+v", qjobRes, err)
		return nil, nil, nil, err
	}

	// Get Secret"s" in Etcd Server
	var _namespace *string
	if secretInQjr.Namespace != "" {
		_namespace = &secretInQjr.Namespace
	} else {
		_namespace = &queuejob.Namespace
	}
	secretList, err := qjrSecret.clients.CoreV1().Secrets(*_namespace).List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", queueJobName, queuejob.Name)})
	if err != nil {
		return nil, nil, nil, err
	}
	secretsInEtcd := []*v1.Secret{}
	for i, _ := range secretList.Items {
		secretsInEtcd = append(secretsInEtcd, &secretList.Items[i])
	}

	// for i, secret := range secretList.Items {
	// 	metaSecret, err := meta.Accessor(&secret)
	// 	if err != nil {
	// 		return nil, nil, nil, err
	// 	}
	// 	controllerRef := metav1.GetControllerOf(metaSecret)
	// 	if controllerRef != nil {
	// 		if controllerRef.UID == queuejob.UID {
	// 			secretsInEtcd = append(secretsInEtcd, &secretList.Items[i])
	// 		}
	// 	}
	// }
	mySecretsInEtcd := []*v1.Secret{}
	for i, secret := range secretsInEtcd {
		if qjrSecret.refManager.BelongTo(qjobRes, secret) {
			mySecretsInEtcd = append(mySecretsInEtcd, secretsInEtcd[i])
		}
	}

	return _namespace, secretInQjr, mySecretsInEtcd, nil
}

func (qjrSecret *QueueJobResSecret) deleteQueueJobResSecrets(qjobRes *arbv1.AppWrapperResource, queuejob *arbv1.AppWrapper) error {

	job := *queuejob

	_namespace, _, activeSecrets, err := qjrSecret.getSecretForQueueJobRes(qjobRes, queuejob)
	if err != nil {
		return err
	}

	active := int32(len(activeSecrets))

	wait := sync.WaitGroup{}
	wait.Add(int(active))
	for i := int32(0); i < active; i++ {
		go func(ix int32) {
			defer wait.Done()
			if err := qjrSecret.delSecret(*_namespace, activeSecrets[ix].Name); err != nil {
				defer utilruntime.HandleError(err)
				klog.V(2).Infof("Failed to delete %v, queue job %q/%q deadline exceeded", activeSecrets[ix].Name, *_namespace, job.Name)
			}
		}(i)
	}
	wait.Wait()

	return nil
}

//Cleanup deletes all services
func (qjrSecret *QueueJobResSecret) Cleanup(queuejob *arbv1.AppWrapper, qjobRes *arbv1.AppWrapperResource) error {
	return qjrSecret.deleteQueueJobResSecrets(qjobRes, queuejob)
}
