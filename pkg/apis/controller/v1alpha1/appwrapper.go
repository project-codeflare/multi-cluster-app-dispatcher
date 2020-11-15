/*
Copyright 2018 The Kubernetes Authors.

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

package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const AppWrapperPlural string = "appwrappers"

// AppWrapperAnnotationKey is the annotation key of Pod to identify
// which AppWrapper it belongs to.
const AppWrapperAnnotationKey = "appwrapper.mcad.ibm.com/appwrapper-name"

// Definition of AppWrapper class
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppWrapper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppWrapperSpec   `json:"spec"`
	Status            AppWrapperStatus `json:"status,omitempty"`
}

// AppWrapperList is a collection of AppWrappers.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppWrapperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AppWrapper `json:"items"`
}

// AppWrapperSpec describes how the App Wrapper will look like.
type AppWrapperSpec struct {
	Priority      float64                `json:"priority,omitempty"`
	PrioritySlope float64                `json:"priorityslope,omitempty"`
	Service       AppWrapperService      `json:"service"`
	AggrResources AppWrapperResourceList `json:"resources"`

	Selector *metav1.LabelSelector `json:"selector,omitempty" protobuf:"bytes,1,opt,name=selector"`

	// SchedSpec specifies the parameters for scheduling.
	SchedSpec SchedulingSpecTemplate `json:"schedulingSpec,omitempty" protobuf:"bytes,2,opt,name=schedulingSpec"`
}
// a collection of AppWrapperResource
type AppWrapperResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AppWrapperResource
	GenericItems	[]AppWrapperGenericResource
}

// AppWrapperService is App Wrapper service definition
type AppWrapperService struct {
	Spec v1.ServiceSpec `json:"spec"`
}

// AppWrapperResource is App Wrapper aggregation resource
//todo: To be depricated
type AppWrapperResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Replicas is the number of desired replicas
	Replicas int32 `json:"replicas,omitempty" protobuf:"bytes,2,opt,name=replicas"`

	// The minimal available pods to run for this AppWrapper; the default value is nil
	MinAvailable *int32 `json:"minavailable,omitempty" protobuf:"bytes,3,opt,name=minavailable"`

	// The number of allocated replicas from this resource type
	AllocatedReplicas int32 `json:"allocatedreplicas"`

	// The priority of this resource
	Priority float64 `json:"priority"`

	// The increasing rate of priority value for this resource
	PrioritySlope float64 `json:"priorityslope"`

	//The type of the resource (is the resource a Pod, a ReplicaSet, a ... ?)
	Type ResourceType `json:"type"`

	//The template for the resource; it is now a raw text because we don't know for what resource
	//it should be instantiated
	Template runtime.RawExtension `json:"template"`

}

// AppWrapperResource is App Wrapper aggregation resource
type AppWrapperGenericResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Replicas is the number of desired replicas
	DesiredAvailable int32 `json:"replicas,omitempty" protobuf:"bytes,2,opt,name=desiredavailable"`

	// The minimal available pods to run for this AppWrapper; the default value is nil
	MinAvailable *int32 `json:"minavailable,omitempty" protobuf:"bytes,3,opt,name=minavailable"`

	// The number of allocated replicas from this resource type
	Allocated int32 `json:"allocated"`

	// The priority of this resource
	Priority float64 `json:"priority"`

	// The increasing rate of priority value for this resource
	PrioritySlope float64 `json:"priorityslope"`

	//The template for the resource; it is now a raw text because we don't know for what resource
	//it should be instantiated
	GenericTemplate runtime.RawExtension `json:"generictemplate"`

	//Optional section that specifies resource requirements for non-standard k8s resources, follows same format as that
	// of standard k8s resources
	CustomPodResources []CustomPodResourceTemplate `json:"custompodresources,omitempty"`
}

type CustomPodResourceTemplate struct {
	Replicas int             `json:"replicas"`
	//todo: replace with
	//Containers []Container Contain v1.ResourceRequirements
	Requests v1.ResourceList `json:"requests"`
	Limits   v1.ResourceList `json:"limits"`
}

// App Wrapper resources type
type ResourceType string

const (
	ResourceTypePod         			ResourceType = "Pod"
	ResourceTypeService     			ResourceType = "Service"
	ResourceTypeSecret      			ResourceType = "Secret"
	ResourceTypeStatefulSet 			ResourceType = "StatefulSet"
	ResourceTypeDeployment  			ResourceType = "Deployment"
	ResourceTypeReplicaSet  			ResourceType = "ReplicaSet"
	ResourceTypePersistentVolume			ResourceType = "PersistentVolume"
	ResourceTypePersistentVolumeClaim		ResourceType = "PersistentVolumeClaim"
	ResourceTypeNamespace				ResourceType = "Namespace"
	ResourceTypeConfigMap				ResourceType = "ConfigMap"
	ResourceTypeNetworkPolicy			ResourceType = "NetworkPolicy"
)

// AppWrapperStatus represents the current state of a AppWrapper
type AppWrapperStatus struct {
	// The number of pending pods.
	// +optional
	Pending int32 `json:"pending,omitempty" protobuf:"bytes,1,opt,name=pending"`

	// +optional
	Running int32 `json:"running,omitempty" protobuf:"bytes,1,opt,name=running"`

	// The number of resources which reached phase Succeeded.
	// +optional
	Succeeded int32 `json:"Succeeded,omitempty" protobuf:"bytes,2,opt,name=succeeded"`

	// The number of resources which reached phase Failed.
	// +optional
	Failed int32 `json:"failed,omitempty" protobuf:"bytes,3,opt,name=failed"`

	// The minimal available resources to run for this AppWrapper (is this different from the MinAvailable from JobStatus)
	// +optional
	MinAvailable int32 `json:"template,omitempty" protobuf:"bytes,4,opt,name=template"`
	
	//Can run?
	CanRun bool `json:"canrun,omitempty" protobuf:"bytes,1,opt,name=canrun"`

	//Is Dispatched?
	IsDispatched bool `json:"isdispatched,omitempty" protobuf:"bytes,1,opt,name=isdispatched"`

	//State - Running, Queued, Deleted ?
	State AppWrapperState `json:"state,omitempty"`

	Message string `json:"message,omitempty"`

	// System defined Priority
	SystemPriority float64 `json:"systempriority,omitempty"`

	// State of QueueJob - Init, Queueing, HeadOfLine, Rejoining, ...
	QueueJobState AppWrapperConditionType `json:"queuejobstate,omitempty"`

	// Microsecond level timestamp when controller first sees QueueJob (by Informer)
	ControllerFirstTimestamp metav1.MicroTime `json:"controllerfirsttimestamp,omitempty"`

	// Tell Informer to ignore this update message (do not generate a controller event)
	FilterIgnore bool `json:"filterignore,omitempty"`

	// Indicate sender of this message (extremely useful for debugging)
	Sender string `json:"sender,omitempty"`

	// Indicate if message is a duplicate (for Informer to recognize duplicate messages)
	Local bool `json:"local,omitempty"`

	// Represents the latest available observations of a appwrapper's current condition.
	Conditions []AppWrapperCondition `json:"conditions,omitempty"`

}

type AppWrapperState string

//enqueued, active, deleting, succeeded, failed
const (
	AppWrapperStateEnqueued AppWrapperState = "Pending"
	AppWrapperStateActive   AppWrapperState = "Running"
	AppWrapperStateDeleted  AppWrapperState = "Deleted"
	AppWrapperStateFailed   AppWrapperState = "Failed"
)

type AppWrapperConditionType string

const (
	AppWrapperCondInit              AppWrapperConditionType = "Init"
	AppWrapperCondQueueing          AppWrapperConditionType = "Queueing"
	AppWrapperCondHeadOfLine        AppWrapperConditionType = "HeadOfLine"
	AppWrapperCondBackoff           AppWrapperConditionType = "Backoff"
	AppWrapperCondDispatched        AppWrapperConditionType = "Dispatched"
	AppWrapperCondRunning           AppWrapperConditionType = "Running"
	AppWrapperCondPreemptCandidate  AppWrapperConditionType = "PreemptCandidate"
	AppWrapperCondPreempted         AppWrapperConditionType = "Preempted"
	AppWrapperCondDeleted           AppWrapperConditionType = "Deleted"
	AppWrapperCondFailed            AppWrapperConditionType = "Failed"
)

// DeploymentCondition describes the state of a deployment at a certain point.
type AppWrapperCondition struct {
	// Type of appwrapper condition.
	Type AppWrapperConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateMicroTime metav1.MicroTime `json:"lastUpdateMicroTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionMicroTime metav1.MicroTime `json:"lastTransitionMicroTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}
