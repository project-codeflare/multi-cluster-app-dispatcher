/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

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

package genericresource

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/klog/v2"

	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var appwrapperJobName = "appwrapper.mcad.ibm.com"
var resourceName = "resourceName"
var appWrapperKind = arbv1.SchemeGroupVersion.WithKind("AppWrapper")

type GenericResources struct {
	clients          *kubernetes.Clientset
	kubeClientConfig *rest.Config
	arbclients       *clientset.Clientset
}

func NewAppWrapperGenericResource(config *rest.Config) *GenericResources {
	return &GenericResources{
		clients:          kubernetes.NewForConfigOrDie(config),
		kubeClientConfig: config,
		arbclients:       clientset.NewForConfigOrDie(config),
	}
}


func (gr *GenericResources) Cleanup(aw *arbv1.AppWrapper, awr *arbv1.AppWrapperGenericResource) (genericResourceName string, groupversionkind *schema.GroupVersionKind, erro error) {
	var err error
	err = nil

	// Default generic source group-version-kind
	default_gvk := &schema.GroupVersionKind{
		Group:   "unknown",
		Version: "unknown",
		Kind:    "unknown",
	}
	// Default generic resource name
	name := ""
	namespaced := true

	dd := gr.clients.Discovery()
	ext := awr.GenericTemplate
	gvk, mapping, err := getResourceMapping(dd, ext.Raw, default_gvk)
	if err != nil {
	    return name, gvk, err
	}

	dclient, err := createDynamicClient(gr, mapping)
	if err != nil {
		return name, gvk, err
	}

	_, apiresourcelist, err := dd.ServerGroupsAndResources()
	if err != nil {
		if derr, ok := err.(*discovery.ErrGroupDiscoveryFailed); ok {
			klog.Warning("Discovery failed for some groups, %d failing: %v", len(derr.Groups), err)
		} else {
			klog.Errorf("Error getting supported groups and resources, err=%#v", err)
			return name, gvk, err
		}
	}
	rsrc := mapping.Resource
	for _, apiresourcegroup := range apiresourcelist {
		if apiresourcegroup.GroupVersion == join(mapping.GroupVersionKind.Group, "/", mapping.GroupVersionKind.Version) {
			for _, apiresource := range apiresourcegroup.APIResources {
				if apiresource.Name == mapping.Resource.Resource && apiresource.Kind == mapping.GroupVersionKind.Kind {
					rsrc = mapping.Resource
					namespaced = apiresource.Namespaced
				}
			}
		}
	}

	unstruct, err := UnmarshalToUnstructured(ext.Raw)
	if err != nil {
	    return name, gvk, err
	}

	namespace := aw.Namespace                       // only delete resources from AppWrapper namespace

	name, err = retrieveName(aw.Namespace, unstruct, "Cleanup")
	if err != nil {
	    return name, gvk, err
	}


	// Get the resource to see if it exists in the AppWrapper namespace
	labelSelector := fmt.Sprintf("%s=%s, %s=%s", appwrapperJobName, aw.Name, resourceName, unstruct.GetName())
	inEtcd, err := dclient.Resource(rsrc).Namespace(aw.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return name, gvk, err
	}

	// Check to see if object already exists in etcd, if not, create the object.
	if inEtcd != nil || len(inEtcd.Items) > 0 {
		newName := truncateName(name)
		err = deleteObject(namespaced, namespace, newName, rsrc, dclient)
		if err != nil {
			if !errors.IsNotFound(err) {
				klog.Errorf("[Cleanup] Error deleting the object `%v`, the error is `%v`.", newName, errors.ReasonForError(err))
			}
			return name, gvk, err
		}
	} else {
		klog.Warningf("[Cleanup] %s/%s not found using label selector: %s.\n", name, namespace, labelSelector)
	}

	return name, gvk, err
}

//SyncQueueJob uses dynamic clients to unwrap (spawn) items inside genericItems block, it is used to create resources inside etcd and return errors when
//unwrapping fails.
//More context here: https://github.com/project-codeflare/multi-cluster-app-dispatcher/issues/598
func (gr *GenericResources) SyncQueueJob(aw *arbv1.AppWrapper, awr *arbv1.AppWrapperGenericResource) (podList []*v1.Pod, err error) {
	startTime := time.Now()
	defer func() {
		if pErr := recover(); pErr != nil {
			klog.Errorf("[SyncQueueJob] Panic occurred: %v, stacktrace: %s", pErr, string(debug.Stack()))
		}
		klog.V(4).Infof("Finished syncing AppWrapper job resource %s/%s (%v)", aw.Namespace, aw.Name, time.Now().Sub(startTime))
	}()

	namespaced := true
	name := ""

	dd := gr.clients.Discovery()
	ext := awr.GenericTemplate
	_, mapping, err := getResourceMapping(dd, ext.Raw, nil)
	if err != nil {
	    return []*v1.Pod{}, err
	}

	dclient, err := createDynamicClient(gr, mapping)
	if err != nil {
		return []*v1.Pod{}, err
	}

	rsrc := mapping.Resource

	unstruct, err := UnmarshalToUnstructured(ext.Raw)
	if err != nil {
	    return []*v1.Pod{}, err
	}

	ownerRef := metav1.NewControllerRef(aw, appWrapperKind)
	unstruct.SetOwnerReferences(append(unstruct.GetOwnerReferences(), *ownerRef))
	namespace := aw.Namespace // only create resources in AppWrapper namespace

	name, err = retrieveName(aw.Namespace, unstruct, "SyncQueueJob")
	if err != nil {
	    return []*v1.Pod{}, err
	}

	labels := map[string]string{}
	if unstruct.GetLabels() == nil {
		unstruct.SetLabels(labels)
	} else {
		labels = unstruct.GetLabels()
	}
	labels[appwrapperJobName] = aw.Name
	labels[resourceName] = unstruct.GetName()
	unstruct.SetLabels(labels)

	// Add labels to pod template if one exists.
	podTemplateFound := addLabelsToPodTemplateField(&unstruct, labels)
	if !podTemplateFound {
		klog.V(4).Infof("[SyncQueueJob] No pod template spec exists for resource: %s to add labels.", name)
	}

	// Get the resource  to see if it exists
	labelSelector := fmt.Sprintf("%s=%s, %s=%s", appwrapperJobName, aw.Name, resourceName, unstruct.GetName())
	inEtcd, err := dclient.Resource(rsrc).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return []*v1.Pod{}, err
	}

	// Check to see if object already exists in etcd, if not, create the object.
	if inEtcd == nil || len(inEtcd.Items) < 1 {
		newName := truncateName(name)
		unstruct.SetName(newName)
		//Asumption object is always namespaced
		namespaced = true
		err = createObject(namespaced, namespace, newName, rsrc, unstruct, dclient)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				klog.V(4).Infof("%v\n", err.Error())
			} else {
				klog.Errorf("Error creating the object `%v`, the error is `%v`", newName, errors.ReasonForError(err))
				return []*v1.Pod{}, err
			}
		}
	}

	return []*v1.Pod{}, nil
}


func GetListOfPodResourcesFromOneGenericItem(awr *arbv1.AppWrapperGenericResource) (resource []*clusterstateapi.Resource, er error) {
	var podResourcesList []*clusterstateapi.Resource

	podTotalresource := clusterstateapi.EmptyResource()
	var err error
	err = nil
	if awr.GenericTemplate.Raw != nil {
		hasContainer, replicas, containers := hasFields(awr.GenericTemplate)
		if hasContainer {
			// Add up all the containers in a pod
			for _, container := range containers {
				res := getContainerResources(container, 1)
				podTotalresource = podTotalresource.Add(res)
			}
			klog.V(8).Infof("[GetListOfPodResourcesFromOneGenericItem] Requested total pod allocation resource from containers `%v`.\n", podTotalresource)
		} else {
			podresources := awr.CustomPodResources
			for _, item := range podresources {
				res := getPodResources(item)
				podTotalresource = podTotalresource.Add(res)
			}
			klog.V(8).Infof("[GetListOfPodResourcesFromOneGenericItem] Requested total allocation resource from 1 pod `%v`.\n", podTotalresource)
		}

		// Addd individual pods to results
		var replicaCount int = int(replicas)
		for i := 0; i < replicaCount; i++ {
			podResourcesList = append(podResourcesList, podTotalresource)
		}
	}

	return podResourcesList, err
}

func GetResources(awr *arbv1.AppWrapperGenericResource) (resource *clusterstateapi.Resource, er error) {

	totalresource := clusterstateapi.EmptyResource()
	var err error
	err = nil
	if awr.GenericTemplate.Raw != nil {
		if len(awr.CustomPodResources) > 0 {
			podresources := awr.CustomPodResources
			for _, item := range podresources {
				res := getPodResources(item)
				totalresource = totalresource.Add(res)
			}
			klog.V(4).Infof("[GetResources] Requested total allocation resource from custompodresources `%v`.\n", totalresource)
			return totalresource, err
		}
		hasContainer, replicas, containers := hasFields(awr.GenericTemplate)
		if hasContainer {
			for _, item := range containers {
				res := getContainerResources(item, replicas)
				totalresource = totalresource.Add(res)
			}
			klog.V(4).Infof("[GetResources] Requested total allocation resource from containers `%v`.\n", totalresource)
			return totalresource, err
		}

	} else {
		err = fmt.Errorf("generic template raw object is not defined (nil)")
	}

	return totalresource, err
}





// returns status of an item present in etcd
func (gr *GenericResources) IsItemCompleted(awgr *arbv1.AppWrapperGenericResource, namespace string, appwrapperName string, genericItemName string) (completed bool) {
	dd := gr.clients.Discovery()

	_, mapping, err := getResourceMapping(dd, awgr.GenericTemplate.Raw, nil)
	if err != nil {
	    return false
	}

	dclient, err := createDynamicClient(gr, mapping)
	if err != nil {
		return false
	}

	rsrc := mapping.Resource
	labelSelector := fmt.Sprintf("%s=%s", appwrapperJobName, appwrapperName)
	inEtcd, err := dclient.Resource(rsrc).Namespace(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		klog.Errorf("[IsItemCompleted] Error listing object: %v", err)
		return false
	}

	for _, job := range inEtcd.Items {
		// job.UnstructuredContent() has status information
		unstructuredObjectName := job.GetName()
		if unstructuredObjectName != genericItemName {
			continue
		}
		jobOwnerRef := job.GetOwnerReferences()
		validAwOwnerRef := false
		for _, val := range jobOwnerRef {
			if val.Name == appwrapperName {
				validAwOwnerRef = true
			}
		}
		if !validAwOwnerRef {
			klog.Warningf("[IsItemCompleted] Item owner name %v does match appwrappper name %v in namespace %v", unstructuredObjectName, appwrapperName, namespace)
			continue
		}

		// check with a false status field
		// check also conditions object
		jobMap := job.UnstructuredContent()
		if jobMap == nil {
			continue
		}

		if job.Object["status"] != nil {
			status := job.Object["status"].(map[string]interface{})
			if status["conditions"] != nil {
				conditions, ok := job.Object["status"].(map[string]interface{})["conditions"].([]interface{})
				// if condition not found skip for this interation
				if !ok {
					klog.Errorf("[IsItemCompleted] Error processing of unstructured object %v in namespace %v with labels %v, err: %v", job.GetName(), job.GetNamespace(), job.GetLabels(), err)
					continue
				}
				for _, item := range conditions {
					completionType := fmt.Sprint(item.(map[string]interface{})["type"])
					// Move this to utils package?
					userSpecfiedCompletionConditions := strings.Split(awgr.CompletionStatus, ",")
					for _, condition := range userSpecfiedCompletionConditions {
						if strings.Contains(strings.ToLower(completionType), strings.ToLower(condition)) {
							return true
						}
					}
				}
			}
		} else {
			klog.Errorf("[IsItemCompleted] Found item with name %v that has status nil in namespace %v with labels %v", job.GetName(), job.GetNamespace(), job.GetLabels())
		}
	}
	return false
}
