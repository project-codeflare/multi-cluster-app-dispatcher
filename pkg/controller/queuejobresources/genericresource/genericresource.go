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
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
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

func join(strs ...string) string {
	var result string
	if strs[0] == "" {
		return strs[len(strs)-1]
	}
	for _, str := range strs {
		result += str
	}
	return result
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
	//todo:DELETEME	dd := common.KubeClient.Discovery()
	dd := gr.clients.Discovery()
	apigroups, err := restmapper.GetAPIGroupResources(dd)
	if err != nil {
		klog.Errorf("[Cleanup] Error getting API resources, err=%#v", err)
		return name, default_gvk, err
	}
	ext := awr.GenericTemplate
	restmapper := restmapper.NewDiscoveryRESTMapper(apigroups)
	_, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, default_gvk, nil)
	if err != nil {
		klog.Errorf("Decoding error, please check your CR! Aborting handling the resource creation, err:  `%v`", err)
		return name, gvk, err
	}

	mapping, err := restmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		klog.Errorf("mapping error from raw object: `%v`", err)
		return name, gvk, err
	}

	//todo:DELETEME		restconfig := common.KubeConfig
	restconfig := gr.kubeClientConfig
	restconfig.GroupVersion = &schema.GroupVersion{
		Group:   mapping.GroupVersionKind.Group,
		Version: mapping.GroupVersionKind.Version,
	}
	dclient, err := dynamic.NewForConfig(restconfig)
	if err != nil {
		klog.Errorf("[Cleanup] Error creating new dynamic client, err=%#v.", err)
		return name, gvk, err
	}

	_, apiresourcelist, err := dd.ServerGroupsAndResources()
	if err != nil {
		klog.Errorf("Error getting supported groups and resources, err=%#v", err)
		return name, gvk, err
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

	// Unmarshal generic item raw object
	var unstruct unstructured.Unstructured
	unstruct.Object = make(map[string]interface{})
	var blob interface{}
	if err = json.Unmarshal(ext.Raw, &blob); err != nil {
		klog.Errorf("[Cleanup] Error unmarshalling, err=%#v", err)
		return name, gvk, err
	}

	unstruct.Object = blob.(map[string]interface{}) //set object to the content of the blob after Unmarshalling
	namespace := ""
	if md, ok := unstruct.Object["metadata"]; ok {

		metadata := md.(map[string]interface{})
		if objectName, ok := metadata["name"]; ok {
			name = objectName.(string)
		}
		if objectns, ok := metadata["namespace"]; ok {
			namespace = objectns.(string)
		}
	}

	// Get the resource to see if it exists
	labelSelector := fmt.Sprintf("%s=%s, %s=%s", appwrapperJobName, aw.Name, resourceName, unstruct.GetName())
	inEtcd, err := dclient.Resource(rsrc).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return name, gvk, err
	}

	// Check to see if object already exists in etcd, if not, create the object.
	if inEtcd != nil || len(inEtcd.Items) > 0 {
		newName := name
		if len(newName) > 63 {
			newName = newName[:63]
		}

		err = deleteObject(namespaced, namespace, newName, rsrc, dclient)
		if err != nil {
			if errors.IsAlreadyExists(err) {
				klog.V(4).Infof("%v\n", err.Error())
			} else {
				klog.Errorf("[Cleanup] Error deleting the object `%v`, the error is `%v`.", newName, errors.ReasonForError(err))
				return name, gvk, err
			}
		}
	} else {
		klog.Warningf("[Cleanup] %s/%s not found using label selector: %s.\n", name, namespace, labelSelector)
	}

	return name, gvk, err
}

func (gr *GenericResources) SyncQueueJob(aw *arbv1.AppWrapper, awr *arbv1.AppWrapperGenericResource) (podList []*v1.Pod, err error) {
	startTime := time.Now()
	defer func() {
		if pErr := recover(); pErr != nil {
			klog.Errorf("[SyncQueueJob] Panic occurred: %v, stacktrace: %s", pErr, string(debug.Stack()))
		}
		klog.V(4).Infof("Finished syncing AppWrapper job resource %s (%v)", aw.Name, time.Now().Sub(startTime))
	}()

	namespaced := true
	//todo:DELETEME	dd := common.KubeClient.Discovery()
	dd := gr.clients.Discovery()
	apigroups, err := restmapper.GetAPIGroupResources(dd)
	if err != nil {
		klog.Errorf("Error getting API resources, err=%#v", err)
		return []*v1.Pod{}, err
	}
	ext := awr.GenericTemplate
	restmapper := restmapper.NewDiscoveryRESTMapper(apigroups)
	//versions := &unstructured.Unstructured{}
	//_, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, versions)
	_, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, nil)
	if err != nil {
		klog.Errorf("Decoding error, please check your CR! Aborting handling the resource creation, err:  `%v`", err)
		return []*v1.Pod{}, err
	}
	mapping, err := restmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		klog.Errorf("mapping error from raw object: `%v`", err)
		return []*v1.Pod{}, err
	}

	//todo:DELETEME		restconfig := common.KubeConfig
	restconfig := gr.kubeClientConfig
	restconfig.GroupVersion = &schema.GroupVersion{
		Group:   mapping.GroupVersionKind.Group,
		Version: mapping.GroupVersionKind.Version,
	}
	dclient, err := dynamic.NewForConfig(restconfig)
	if err != nil {
		klog.Errorf("Error creating new dynamic client, err=%#v", err)
		return []*v1.Pod{}, err
	}

	_, apiresourcelist, err := dd.ServerGroupsAndResources()
	if err != nil {
		klog.Errorf("Error getting supported groups and resources, err=%#v", err)
		return []*v1.Pod{}, err
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
	var unstruct unstructured.Unstructured
	unstruct.Object = make(map[string]interface{})
	var blob interface{}
	if err = json.Unmarshal(ext.Raw, &blob); err != nil {
		klog.Errorf("Error unmarshalling, err=%#v", err)
		return []*v1.Pod{}, err
	}
	ownerRef := metav1.NewControllerRef(aw, appWrapperKind)
	unstruct.Object = blob.(map[string]interface{}) //set object to the content of the blob after Unmarshalling
	unstruct.SetOwnerReferences(append(unstruct.GetOwnerReferences(), *ownerRef))
	namespace := "default"
	name := ""
	if md, ok := unstruct.Object["metadata"]; ok {

		metadata := md.(map[string]interface{})
		if objectName, ok := metadata["name"]; ok {
			name = objectName.(string)
		}
		if objectns, ok := metadata["namespace"]; ok {
			namespace = objectns.(string)
		}
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
		newName := name
		if len(newName) > 63 {
			newName = newName[:63]
		}
		unstruct.SetName(newName)
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

	// Get the related resources of created object
	var thisObj *unstructured.Unstructured
	var err1 error
	if namespaced {
		thisObj, err1 = dclient.Resource(rsrc).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	} else {
		thisObj, err1 = dclient.Resource(rsrc).Get(context.Background(), name, metav1.GetOptions{})
	}
	if err1 != nil {
		klog.Errorf("Could not get created resource with error %v", err1)
		return []*v1.Pod{}, err1
	}
	thisOwnerRef := metav1.NewControllerRef(thisObj, thisObj.GroupVersionKind())

	podL, _ := gr.clients.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	pods := []*v1.Pod{}
	for _, pod := range (*podL).Items {
		parent := metav1.GetControllerOf(&pod)
		if reflect.DeepEqual(thisOwnerRef, parent) {
			pods = append(pods, &pod)
		}
		klog.V(10).Infof("[SyncQueueJob] pod %s created from a Generic Item\n", pod.Name)
	}
	return pods, nil
}

//checks if object has pod template spec and add new labels
func addLabelsToPodTemplateField(unstruct *unstructured.Unstructured, labels map[string]string) (hasFields bool) {
	spec, isFound, _ := unstructured.NestedMap(unstruct.UnstructuredContent(), "spec")
	if !isFound {
		klog.V(10).Infof("[addLabelsToPodTemplateField] 'spec' field not found.")
		return false
	}
	template, isFound, _ := unstructured.NestedMap(spec, "template")
	if !isFound {
		klog.V(10).Infof("[addLabelsToPodTemplateField] 'spec.template' field not found.")
		return false
	}

	marshal, _ := json.Marshal(template)
	unmarshal := v1.PodTemplateSpec{}
	if err := json.Unmarshal(marshal, &unmarshal); err != nil {
		klog.Warning(err)
		return false
	}
	existingLabels, isFound, _ := unstructured.NestedStringMap(template, "metadata", "labels")
	if !isFound {
		klog.V(10).Infof("[addLabelsToPodTemplateField] 'spec.template.metadata.labels' field not found.")
		return false
	}
	newLength := len(existingLabels) + len(labels)
	m := make(map[string]string, newLength) // convert map[string]string into map[string]interface{}
	for k, v := range existingLabels {
		m[k] = v
	}

	for k, v := range labels {
		m[k] = v
	}

	if err := unstructured.SetNestedStringMap(unstruct.Object, m, "spec", "template", "metadata", "labels"); err != nil {
		klog.Warning(err)
		return false
	}

	return isFound
}

//checks if object has replicas and containers field
func hasFields(obj runtime.RawExtension) (hasFields bool, replica float64, containers []v1.Container) {
	var unstruct unstructured.Unstructured
	unstruct.Object = make(map[string]interface{})
	var blob interface{}
	if err := json.Unmarshal(obj.Raw, &blob); err != nil {
		klog.Errorf("Error unmarshalling, err=%#v", err)
		return false, 0, nil
	}
	unstruct.Object = blob.(map[string]interface{})
	spec, isFound, _ := unstructured.NestedMap(unstruct.UnstructuredContent(), "spec")
	if !isFound {
		klog.Warningf("[hasFields] No spec field found in raw object: %#v", unstruct.UnstructuredContent())
	}

	replicas, isFound, _ := unstructured.NestedFloat64(spec, "replicas")
	// Set default to 1 if no replicas field is found (handles the case of a single pod creation without replicaset.
	if !isFound {
		replicas = 1
	}

	template, isFound, _ := unstructured.NestedMap(spec, "template")
	// If spec does not contain a podtemplate, check for pod singletons
	var subspec map[string]interface{}
	if !isFound {
		subspec = spec
		klog.V(6).Infof("[hasFields] No template field found in raw object: %#v", spec)
	} else {
		subspec, isFound, _ = unstructured.NestedMap(template, "spec")
	}

	containerList, isFound, _ := unstructured.NestedSlice(subspec, "containers")
	if !isFound {
		klog.Warningf("[hasFields] No containers field found in raw object: %#v", subspec)
		return false, 0, nil
	}
	objContainers := make([]v1.Container, len(containerList))
	for _, container := range containerList {
		marshal, _ := json.Marshal(container)
		unmarshal := v1.Container{}
		_ = json.Unmarshal(marshal, &unmarshal)
		objContainers = append(objContainers, unmarshal)
	}
	return isFound, replicas, objContainers
}

func createObject(namespaced bool, namespace string, name string, rsrc schema.GroupVersionResource, unstruct unstructured.Unstructured, dclient dynamic.Interface) (erro error) {
	var err error
	if !namespaced {
		res := dclient.Resource(rsrc)
		_, err = res.Create(context.Background(), &unstruct, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				klog.Errorf("%v\n", err.Error())
				return nil
			} else {
				klog.Errorf("Error creating the object `%v`, the error is `%v`", name, errors.ReasonForError(err))
				return err
			}
		} else {
			klog.V(4).Infof("Resource `%v` created\n", name)
			return nil
		}
	} else {
		res := dclient.Resource(rsrc).Namespace(namespace)
		_, err = res.Create(context.Background(), &unstruct, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				klog.Errorf("%v\n", err.Error())
				return nil
			} else {
				klog.Errorf("Error creating the object `%v`, the error is `%v`", name, errors.ReasonForError(err))
				return err
			}
		} else {
			klog.V(4).Infof("Resource `%v` created\n", name)
			return nil

		}
	}
}

func deleteObject(namespaced bool, namespace string, name string, rsrc schema.GroupVersionResource, dclient dynamic.Interface) (erro error) {
	var err error
	backGround := metav1.DeletePropagationBackground
	delOptions := metav1.DeleteOptions{PropagationPolicy: &backGround}
	if !namespaced {
		res := dclient.Resource(rsrc)
		err = res.Delete(context.Background(), name, delOptions)
	} else {
		res := dclient.Resource(rsrc).Namespace(namespace)
		err = res.Delete(context.Background(), name, delOptions)
	}

	if err != nil {
		klog.Errorf("[deleteObject] Error deleting the object `%v`, the error is `%v`.", name, errors.ReasonForError(err))
		return err
	} else {
		klog.V(4).Infof("[deleteObject] Resource `%v` deleted.\n", name)
		return nil
	}
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

func getPodResources(pod arbv1.CustomPodResourceTemplate) (resource *clusterstateapi.Resource) {
	replicas := pod.Replicas
	req := clusterstateapi.NewResource(pod.Requests)
	limit := clusterstateapi.NewResource(pod.Limits)
	tolerance := 0.001

	// Use limit if request is 0
	if diff := math.Abs(req.MilliCPU - float64(0.0)); diff < tolerance {
		req.MilliCPU = limit.MilliCPU
	}

	if diff := math.Abs(req.Memory - float64(0.0)); diff < tolerance {
		req.Memory = limit.Memory
	}

	if req.GPU <= 0 {
		req.GPU = limit.GPU
	}
	req.MilliCPU = req.MilliCPU * float64(replicas)
	req.Memory = req.Memory * float64(replicas)
	req.GPU = req.GPU * int64(replicas)
	return req
}

func getContainerResources(container v1.Container, replicas float64) *clusterstateapi.Resource {
	req := clusterstateapi.NewResource(container.Resources.Requests)
	limit := clusterstateapi.NewResource(container.Resources.Limits)

	tolerance := 0.001

	// Use limit if request is 0
	if diff := math.Abs(req.MilliCPU - float64(0.0)); diff < tolerance {
		req.MilliCPU = limit.MilliCPU
	}

	if diff := math.Abs(req.Memory - float64(0.0)); diff < tolerance {
		req.Memory = limit.Memory
	}

	if req.GPU <= 0 {
		req.GPU = limit.GPU
	}

	req.MilliCPU = req.MilliCPU * float64(replicas)
	req.Memory = req.Memory * float64(replicas)
	req.GPU = req.GPU * int64(replicas)
	return req
}

//returns status of an item present in etcd
func (gr *GenericResources) IsItemCompleted(awgr *arbv1.AppWrapperGenericResource, namespace string, appwrapperName string, genericItemName string) (completed bool, condition string) {
	dd := gr.clients.Discovery()
	klog.V(8).Infof("[IsItemCompleted] - checking status !!!!!!!")
	apigroups, err := restmapper.GetAPIGroupResources(dd)
	if err != nil {
		klog.Errorf("[IsItemCompleted] Error getting API resources, err=%#v", err)
		return false, ""
	}
	restmapper := restmapper.NewDiscoveryRESTMapper(apigroups)
	_, gvk, err := unstructured.UnstructuredJSONScheme.Decode(awgr.GenericTemplate.Raw, nil, nil)
	if err != nil {
		klog.Errorf("[IsItemCompleted] Decoding error, please check your CR! Aborting handling the resource creation, err:  `%v`", err)
		return false, ""
	}

	mapping, err := restmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		klog.Errorf("[IsItemCompleted] mapping error from raw object: `%v`", err)
		return false, ""
	}
	restconfig := gr.kubeClientConfig
	restconfig.GroupVersion = &schema.GroupVersion{
		Group:   mapping.GroupVersionKind.Group,
		Version: mapping.GroupVersionKind.Version,
	}
	rsrc := mapping.Resource
	dclient, err := dynamic.NewForConfig(restconfig)
	if err != nil {
		klog.Errorf("[IsItemCompleted] Error creating new dynamic client, err %v", err)
		return false, ""
	}

	labelSelector := fmt.Sprintf("%s=%s", appwrapperJobName, appwrapperName)
	inEtcd, err := dclient.Resource(rsrc).Namespace(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		klog.Errorf("[IsItemCompleted] Error listing object: %v", err)
		return false, ""
	}

	for _, job := range inEtcd.Items {
		//job.UnstructuredContent() has status information
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

		//check with a false status field
		//check also conditions object
		jobMap := job.UnstructuredContent()
		if jobMap == nil {
			continue
		}
		
		if job.Object["status"] != nil {
			status := job.Object["status"].(map[string]interface{})
			if status["conditions"] != nil {
				conditions, ok := job.Object["status"].(map[string]interface{})["conditions"].([]interface{})
				//if condition not found skip for this interation
				if !ok {
					klog.Errorf("[IsItemCompleted] Error processing of unstructured object %v in namespace %v with labels %v, err: %v", job.GetName(), job.GetNamespace(), job.GetLabels(), err)
					continue
				}
				for _, item := range conditions {
					completionType := fmt.Sprint(item.(map[string]interface{})["type"])
					//Move this to utils package?
					userSpecfiedCompletionConditions := strings.Split(awgr.CompletionStatus, ",")
					for _, condition := range userSpecfiedCompletionConditions {
						if strings.Contains(strings.ToLower(completionType), strings.ToLower(condition)) {
							klog.V(8).Infof("[IsItemCompleted] condition `%v`.\n", condition)

							return true, condition
						}
					}
				}
			}
		} else {
			klog.Errorf("[IsItemCompleted] Found item with name %v that has status nil in namespace %v with labels %v", job.GetName(), job.GetNamespace(), job.GetLabels())
		}
	}
	return false, ""
}
