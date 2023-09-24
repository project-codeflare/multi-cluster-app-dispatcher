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
package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const AppWrapperPlural string = "appwrappers"

// AppWrapperAnnotationKey is the annotation key of Pod to identify
// which AppWrapper it belongs to.
const AppWrapperAnnotationKey = "appwrapper.mcad.ibm.com/appwrapper-name"

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Definition of AppWrapper class
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppWrapper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppWrapperSpec   `json:"spec"`
	Status            AppWrapperStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AppWrapperList is a collection of AppWrappers.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppWrapperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []AppWrapper `json:"items"`
}

// AppWrapperSpec describes how the App Wrapper will look like.
type AppWrapperSpec struct {
	// +optional
	Priority int32 `json:"priority,omitempty"`

	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=float
	// +optional
	PrioritySlope float64 `json:"priorityslope,omitempty"`

	// +optional
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

	// +optional
	Items []AppWrapperResource `json:"Items"`
	// +optional
	GenericItems []AppWrapperGenericResource `json:"GenericItems"`
}

// AppWrapperService is App Wrapper service definition
type AppWrapperService struct {
	Spec v1.ServiceSpec `json:"spec"`
}

// AppWrapperResource is App Wrapper aggregation resource
// todo: To be depricated
type AppWrapperResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Replicas is the number of desired replicas
	Replicas int32 `json:"replicas,omitempty" protobuf:"bytes,2,opt,name=replicas"`

	// The minimal available pods to run for this AppWrapper; the default value is nil
	MinAvailable *int32 `json:"minavailable,omitempty" protobuf:"bytes,3,opt,name=minavailable"`

	// The number of allocated replicas from this resource type
	// +optional
	AllocatedReplicas int32 `json:"allocatedreplicas"`

	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=float
	// +optional
	Priority float64 `json:"priority,omitempty"`

	// The increasing rate of priority value for this resource
	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=float
	PrioritySlope float64 `json:"priorityslope"`

	//The type of the resource (is the resource a Pod, a ReplicaSet, a ... ?)
	// +optional
	Type ResourceType `json:"type"`

	//The template for the resource; it is now a raw text because we don't know for what resource
	//it should be instantiated
	// +kubebuilder:pruning:PreserveUnknownFields
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
	// +optional
	Allocated int32 `json:"allocated"`

	// The priority of this resource
	// +optional
	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=float
	Priority float64 `json:"priority"`

	// The increasing rate of priority value for this resource
	// +optional
	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=float
	PrioritySlope float64 `json:"priorityslope"`

	//The template for the resource; it is now a raw text because we don't know for what resource
	//it should be instantiated
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	GenericTemplate runtime.RawExtension `json:"generictemplate"`

	//Optional section that specifies resource requirements for non-standard k8s resources, follows same format as that
	// of standard k8s resources
	CustomPodResources []CustomPodResourceTemplate `json:"custompodresources,omitempty"`

	//Optional field for users to determine completion status of item
	CompletionStatus string `json:"completionstatus,omitempty"`
}

type CustomPodResourceTemplate struct {
	Replicas int `json:"replicas"`
	//todo: replace with
	//Containers []Container Contain v1.ResourceRequirements
	Requests v1.ResourceList `json:"requests"`

	// +optional
	Limits v1.ResourceList `json:"limits"`
}

// App Wrapper resources type
type ResourceType string

const (
	ResourceTypePod                   ResourceType = "Pod"
	ResourceTypeService               ResourceType = "Service"
	ResourceTypeSecret                ResourceType = "Secret"
	ResourceTypeStatefulSet           ResourceType = "StatefulSet"
	ResourceTypeDeployment            ResourceType = "Deployment"
	ResourceTypeReplicaSet            ResourceType = "ReplicaSet"
	ResourceTypePersistentVolume      ResourceType = "PersistentVolume"
	ResourceTypePersistentVolumeClaim ResourceType = "PersistentVolumeClaim"
	ResourceTypeNamespace             ResourceType = "Namespace"
	ResourceTypeConfigMap             ResourceType = "ConfigMap"
	ResourceTypeNetworkPolicy         ResourceType = "NetworkPolicy"
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

	//State - Pending, Running, Failed, Deleted
	State AppWrapperState `json:"state,omitempty"`

	Message string `json:"message,omitempty"`

	// System defined Priority
	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=float
	SystemPriority float64 `json:"systempriority,omitempty"`

	// State of QueueJob - Init, Queueing, HeadOfLine, Rejoining, ...
	QueueJobState AppWrapperConditionType `json:"queuejobstate,omitempty"`

	// Microsecond level timestamp when controller first sees QueueJob (by Informer)
	ControllerFirstTimestamp metav1.MicroTime `json:"controllerfirsttimestamp,omitempty"`

	// Microsecond level timestamp when controller first sets appwrapper in state Running
	ControllerFirstDispatchTimestamp metav1.MicroTime `json:"controllerfirstdispatchtimestamp,omitempty"`

	// Tell Informer to ignore this update message (do not generate a controller event)
	FilterIgnore bool `json:"filterignore,omitempty"`

	// Indicate sender of this message (extremely useful for debugging)
	Sender string `json:"sender,omitempty"`

	// Indicate if message is a duplicate (for Informer to recognize duplicate messages)
	Local bool `json:"local,omitempty"`

	// Represents the latest available observations of a appwrapper's current condition.
	Conditions []AppWrapperCondition `json:"conditions,omitempty"`

	// Represents the latest available observations of pods under appwrapper
	PendingPodConditions []PendingPodSpec `json:"pendingpodconditions"`

	ItemCompletionStatus GenericItemCompletionStatus `json:"itemCompletionStatus,omitempty"`
}

type GenericItemCompletionStatus struct {
	GenericItems [] GenericItem `json:"genericItems,omitempty"`
}

type GenericItem struct {
	schema.GroupVersionKind `json:"gvk"`
	Name string `json:"name"`
	Namespace string `json:"namespace"`
	Condition string `json:"condition"`
}

type AppWrapperState string

// enqueued, active, deleting, succeeded, failed
const (
	AppWrapperStateEnqueued              AppWrapperState = "Pending"
	AppWrapperStateActive                AppWrapperState = "Running"
	AppWrapperStateDeleted               AppWrapperState = "Deleted"
	AppWrapperStateFailed                AppWrapperState = "Failed"
	AppWrapperStateCompleted             AppWrapperState = "Completed"
	AppWrapperStateRunningHoldCompletion AppWrapperState = "RunningHoldCompletion"
)

type AppWrapperConditionType string

const (
	AppWrapperCondInit                  AppWrapperConditionType = "Init"
	AppWrapperCondQueueing              AppWrapperConditionType = "Queueing"
	AppWrapperCondHeadOfLine            AppWrapperConditionType = "HeadOfLine"
	AppWrapperCondBackoff               AppWrapperConditionType = "Backoff"
	AppWrapperCondDispatched            AppWrapperConditionType = "Dispatched"
	AppWrapperCondRunning               AppWrapperConditionType = "Running"
	AppWrapperCondPreemptCandidate      AppWrapperConditionType = "PreemptCandidate"
	AppWrapperCondPreempted             AppWrapperConditionType = "Preempted"
	AppWrapperCondDeleted               AppWrapperConditionType = "Deleted"
	AppWrapperCondFailed                AppWrapperConditionType = "Failed"
	AppWrapperCondCompleted             AppWrapperConditionType = "Completed"
	AppWrapperCondRunningHoldCompletion AppWrapperConditionType = "RunningHoldCompletion"
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

type PendingPodSpec struct {
	PodName    string            `json:"podname,omitempty"`
	Conditions []v1.PodCondition `json:"conditions,omitempty"`
}
