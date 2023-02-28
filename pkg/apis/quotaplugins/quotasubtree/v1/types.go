package v1
/*
Copyright 2022 The Multi-Cluster App Dispatcher Authors.

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

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceName string
type ResourceList map[ResourceName]resource.Quantity

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaSubtree is a specification for a quota subtree resource
type QuotaSubtree struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuotaSubtreeSpec   `json:"spec"`
	Status QuotaSubtreeStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaSubtreeList is a collection of resource plan
type QuotaSubtreeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []QuotaSubtree `json:"items"`
}

// QuotaSubtreeSpec is the spec for a resource plan
type QuotaSubtreeSpec struct {
	Parent          string  `json:"parent,omitempty" protobuf:"bytes,1,opt,name=parent"`
	ParentNamespace string  `json:"parentNamespace,omitempty" protobuf:"bytes,2,opt,name=parentNamespace"`
	Children        []Child `json:"children,omitempty" protobuf:"bytes,3,opt,name=children"`
}

// Child is the spec for a QuotaSubtree resource
type Child struct {
	Name         string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Namespace    string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	Quotas       Quota  `json:"quotas,omitempty" protobuf:"bytes,4,opt,name=quotas"`
	Path         string `json:"path,omitempty" protobuf:"bytes,5,opt,name=path"`
}

// Quota is the spec for a QuotaSubtree resource
type Quota struct {
	Disabled  bool         `json:"disabled,omitempty" protobuf:"bytes,1,opt,name=disabled"`
	Requests  ResourceList `json:"requests,omitempty" protobuf:"bytes,2,rep,name=requests,casttype=ResourceList,castkey=ResourceName"`
	HardLimit bool         `json:"hardLimit,omitempty" protobuf:"bytes,4,opt,name=hardLimit"`
}

// QuotaSubtreeStatus is the status for a QuotaSubtree resource
type QuotaSubtreeStatus struct {
	TotalAllocation ResourceAllocation   `json:"totalAllocation,omitempty protobuf:"bytes,1,opt,name=totalAllocation"`
	Children        []ResourceAllocation `json:"children,omitempty protobuf:"bytes,2,opt,name=children"`
}

// ResourceAllocation is the spec for the child status
type ResourceAllocation struct {
	Name      string                   `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Namespace string                   `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	Path      string                   `json:"path,omitempty" protobuf:"bytes,3,opt,name=path"`
	Allocated ResourceAllocationStatus `json:"allocated,omitempty" protobuf:"bytes,5,opt,name=allocated"`
}

// ResourceAllocationStatus is the spec for the child resource usage
type ResourceAllocationStatus struct {
	Requests map[string]string `json:"requests,omitempty" protobuf:"bytes,2,opt,name=requests"`
}
