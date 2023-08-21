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

package api

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// NodeInfo is node level aggregated information.
type NodeInfo struct {
	Name string
	Node *v1.Node

	// The releasing resource on that node
	Releasing *Resource

	// The idle resource on that node
	Idle *Resource

	// The used resource on that node, including running and terminating
	// pods
	Used *Resource

	Allocatable *Resource
	Capability  *Resource

	// Track labels for potential filtering
	Labels map[string]string

	// Track Schedulable flag for potential filtering
	Unschedulable bool

	// Taints for potential filtering
	Taints []v1.Taint
}

func NewNodeInfo(node *v1.Node) *NodeInfo {
	if node == nil {
		return &NodeInfo{
			Releasing: EmptyResource(),
			Idle:      EmptyResource(),
			Used:      EmptyResource(),

			Allocatable: EmptyResource(),
			Capability:  EmptyResource(),

			Labels:        make(map[string]string),
			Unschedulable: false,
			Taints:        []v1.Taint{},
		}
	}

	return &NodeInfo{
		Name: node.Name,
		Node: node,

		Releasing: EmptyResource(),
		Idle:      NewResource(node.Status.Allocatable),
		Used:      EmptyResource(),

		Allocatable: NewResource(node.Status.Allocatable),
		Capability:  NewResource(node.Status.Capacity),

		Labels:        node.GetLabels(),
		Unschedulable: node.Spec.Unschedulable,
		Taints:        node.Spec.Taints,
	}
}

func (ni *NodeInfo) Clone() *NodeInfo {
	res := NewNodeInfo(ni.Node)

	return res
}

func (ni *NodeInfo) SetNode(node *v1.Node) {
	if ni.Node == nil {
		ni.Idle = NewResource(node.Status.Allocatable)
	}

	ni.Name = node.Name
	ni.Node = node
	ni.Allocatable = NewResource(node.Status.Allocatable)
	ni.Capability = NewResource(node.Status.Capacity)
	ni.Labels = NewStringsMap(node.Labels)
	ni.Unschedulable = node.Spec.Unschedulable
	ni.Taints = NewTaints(node.Spec.Taints)
}

func (ni NodeInfo) String() string {
	res := ""

	return fmt.Sprintf("Node (%s): idle <%v>, used <%v>, releasing <%v>%s",
		ni.Name, ni.Idle, ni.Used, ni.Releasing, res)
}
