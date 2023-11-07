package genericresource

import (
	"math"
	"fmt"
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	v1 "k8s.io/api/core/v1"

	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"k8s.io/klog/v2"

	"k8s.io/client-go/restmapper"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

)

func getPodResources(pod arbv1.CustomPodResourceTemplate) *clusterstateapi.Resource {
    req := clusterstateapi.NewResource(pod.Requests)
    limit := clusterstateapi.NewResource(pod.Limits)

    calculateResources(req, limit, float64(pod.Replicas))

    return req
}

func getContainerResources(container v1.Container, replicas float64) *clusterstateapi.Resource {
    req := clusterstateapi.NewResource(container.Resources.Requests)
    limit := clusterstateapi.NewResource(container.Resources.Limits)

    calculateResources(req, limit, replicas)

    return req
}


func calculateResources(req *clusterstateapi.Resource, limit *clusterstateapi.Resource, replicas float64) {
    tolerance := 0.001

    // Use limit if request is 0
    if diff := math.Abs(req.MilliCPU); diff < tolerance {
        req.MilliCPU = limit.MilliCPU
    }

    if diff := math.Abs(req.Memory); diff < tolerance {
        req.Memory = limit.Memory
    }

    if req.GPU <= 0 {
        req.GPU = limit.GPU
    }

    req.MilliCPU = req.MilliCPU * replicas
    req.Memory = req.Memory * replicas
    req.GPU = req.GPU * int64(replicas)
}

func retrieveName(awNamespace string, unstruct unstructured.Unstructured, logContext string) (string, error) {
    var name string

    if md, ok := unstruct.Object["metadata"]; ok {
        metadata := md.(map[string]interface{})
        if objectName, ok := metadata["name"]; ok {
            name = objectName.(string)
        }
        if objectns, ok := metadata["namespace"]; ok {
            if objectns.(string) != awNamespace {
                return "", fmt.Errorf("[%s] resource namespace \"%s\" is different from AppWrapper namespace \"%s\"", logContext, objectns.(string), awNamespace)
            }
        }
    }

    return name, nil
}

func createDynamicClient(gr *GenericResources, mapping *meta.RESTMapping) (dynamic.Interface, error) {
	restconfig := gr.kubeClientConfig
	restconfig.GroupVersion = &schema.GroupVersion{
		Group:   mapping.GroupVersionKind.Group,
		Version: mapping.GroupVersionKind.Version,
	}

	dclient, err := dynamic.NewForConfig(restconfig)
	if err != nil {
		klog.Errorf("Error creating new dynamic client, err: %v", err)
		return nil, err
	}
	return dclient, nil
}

func getResourceMapping(dd discovery.DiscoveryInterface, raw []byte, defaultGVK *schema.GroupVersionKind) (*schema.GroupVersionKind, *meta.RESTMapping, error) {
    apigroups, err := restmapper.GetAPIGroupResources(dd)
    if err != nil {
        klog.Errorf("Error getting API resources, err=%#v", err)
        return nil, nil, err
    }

    restmapper := restmapper.NewDiscoveryRESTMapper(apigroups)
    _, gvk, err := unstructured.UnstructuredJSONScheme.Decode(raw, defaultGVK, nil)
    if err != nil {
        klog.Errorf("Decoding error, please check your CR! err: `%v`", err)
        return nil, nil, err
    }

    mapping, err := restmapper.RESTMapping(gvk.GroupKind(), gvk.Version)
    if err != nil {
        klog.Errorf("Mapping error from raw object: `%v`", err)
        return nil, nil, err
    }

    return gvk, mapping, nil
}


// checks if object has replicas and containers field
func hasFields(obj runtime.RawExtension) (hasFields bool, replica float64, containers []v1.Container) {
	unstruct, err := UnmarshalToUnstructured(obj.Raw)
	if err != nil {
	    return false, 0, nil
	}

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

// checks if object has pod template spec and add new labels
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

func createObject(namespaced bool, namespace string, name string, rsrc schema.GroupVersionResource, unstruct unstructured.Unstructured, dclient dynamic.Interface) error {
	res := getResourceInterface(namespaced, namespace, rsrc, dclient)

	_, err := res.Create(context.Background(), &unstruct, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			klog.Errorf("[createObject] Object `%v` already exists: %v", name, err)
			return nil
		}
		klog.Errorf("[createObject] Error creating the object `%v`: %v", name, err)
		return err
	}

	klog.V(4).Infof("[createObject] Resource `%v` created\n", name)
	return nil
}

func deleteObject(namespaced bool, namespace string, name string, rsrc schema.GroupVersionResource, dclient dynamic.Interface) error {
	backGround := metav1.DeletePropagationBackground
	delOptions := metav1.DeleteOptions{PropagationPolicy: &backGround}

	res := getResourceInterface(namespaced, namespace, rsrc, dclient)

	err := res.Delete(context.Background(), name, delOptions)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(4).Infof("[deleteObject] object `%v` not found. No action taken.", name)
			return nil
		}
		klog.Errorf("[deleteObject] Error deleting the object `%v`: %v", name, err)
		return err
	}

	klog.V(4).Infof("[deleteObject] Resource `%v` deleted.\n", name)
	return nil
}

func getResourceInterface(namespaced bool, namespace string, rsrc schema.GroupVersionResource, dclient dynamic.Interface) dynamic.ResourceInterface {
	if namespaced {
		return dclient.Resource(rsrc).Namespace(namespace)
	}
	return dclient.Resource(rsrc)
}
