/*
Copyright 2023 The Multi-Cluster App Dispatcher Authors.

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

package core

import (
	"unsafe"

	"k8s.io/klog/v2"
)

// TreeSnapshot : A snapshot of a quota tree that could be used to undo a consumer allocation.
//
// A snapshop of the tree state is taken before a consumer allocation trial, including only data
// that could potentially change during allocation of the target consumer. Reinstating the snapshot
// to the tree in case it is desired to undo the allocation.
type TreeSnapshot struct {
	// the target quota tree
	targetTree *QuotaTree
	// the target consumer of allocation
	targetConsumer *Consumer

	// all consumers that could potentially change due to the allocation of the target consumer
	allChangedConsumers []*Consumer
	// snapshot of nodes state
	nodeStates map[string]*NodeState
	// snapshot of consumers state
	consumerStates map[string]*ConsumerState
	// consumers already allocated (running)
	activeConsumers map[string]*Consumer

	// snapshot of preempted consumers
	preemptedConsumers []string
	// snapshot of list of preempted consumer objects
	preemptedConsumersArray []*Consumer
}

// NodeState : snapshot data about state of a node
type NodeState struct {
	// the node
	node *QuotaNode
	// amount allocated on the node
	allocated *Allocation
	// list of consumers allocated on the node
	consumers []*Consumer
}

// ConsumerState : snapshot data about state of a consumer
type ConsumerState struct {
	// the consumer
	consumer *Consumer
	// node the consumer is assigned to
	aNode *QuotaNode
}

// NewTreeSnapshot : create a new snapshot before allocating a consumer on a quota tree
func NewTreeSnapshot(tree *QuotaTree, consumer *Consumer) *TreeSnapshot {
	ts := &TreeSnapshot{
		targetTree:     tree,
		targetConsumer: consumer,
	}
	ts.Reset()
	ts.allChangedConsumers = append(ts.allChangedConsumers, consumer)
	return ts
}

// Take : take a snapshot
func (ts *TreeSnapshot) Take(controller *Controller, changedConsumers map[string]*Consumer) bool {
	klog.V(4).Infof("Taking snapshot of tree %s prior to allocation of consumer %s \n",
		ts.targetTree.GetName(), ts.targetConsumer.GetID())

	// add potentially altered consumers
	for _, c := range changedConsumers {
		ts.allChangedConsumers = append(ts.allChangedConsumers, c)
	}

	// make a copy of active consumers
	for cid, c := range controller.consumers {
		ts.activeConsumers[cid] = c
	}
	// make a copy of preempted consumers
	ts.preemptedConsumers = make([]string, len(controller.preemptedConsumers))
	copy(ts.preemptedConsumers, controller.preemptedConsumers)
	ts.preemptedConsumersArray = make([]*Consumer, len(controller.preemptedConsumersArray))
	copy(ts.preemptedConsumersArray, controller.preemptedConsumersArray)

	// copy node states for all potentially altered consumers
	for _, c := range ts.allChangedConsumers {

		// copy state of consumer; skip if already visited this consumer
		if taken := ts.takeConsumer(c); !taken {
			continue
		}

		// visit nodes along path from consumer leaf node to root
		groupID := c.GetGroupID()
		leafNode := ts.targetTree.GetLeafNode(groupID)
		if leafNode == nil {
			klog.V(4).Infof("Consumer %s member of unknown group %s \n", c.GetID(), groupID)
			ts.Reset()
			return false
		}
		path := leafNode.GetPathToRoot()
		for _, n := range path {
			node := (*QuotaNode)(unsafe.Pointer(n))
			// make copy of node state; skip remaining nodes along path if node already visited
			if taken := ts.takeNode(node); !taken {
				break
			}
			// make a copy of node consumers
			for _, c := range node.GetConsumers() {
				ts.takeConsumer(c)
			}
		}
	}
	return true
}

// Reinstate : reinstate the snapshot
func (ts *TreeSnapshot) Reinstate(controller *Controller) {
	// reinstate consumers state
	for _, cs := range ts.consumerStates {
		if c := cs.consumer; c != nil {
			c.SetNode(cs.aNode)
		}
	}

	// reinstate nodes state
	for _, ns := range ts.nodeStates {
		if n := ns.node; n != nil {
			n.SetAllocated(ns.allocated)
			n.SetConsumers(ns.consumers)
		}
	}

	// reinstate preempted and active consumers
	controller.consumers = ts.activeConsumers
	controller.preemptedConsumers = ts.preemptedConsumers
	controller.preemptedConsumersArray = ts.preemptedConsumersArray

	// reset the state of the snapshot
	ts.Reset()
}

// takeNode : take state of a node; return false if already taken
func (ts *TreeSnapshot) takeNode(node *QuotaNode) bool {
	nodeID := node.GetID()
	if _, exists := ts.nodeStates[nodeID]; exists {
		return false
	}
	allocated := node.GetAllocated().Clone()
	consumers := make([]*Consumer, len(node.GetConsumers()))
	copy(consumers, node.GetConsumers())
	ts.nodeStates[nodeID] = &NodeState{
		node:      node,
		allocated: allocated,
		consumers: consumers,
	}
	return true
}

// takeConsumer : take state of consumer; return false if already taken
func (ts *TreeSnapshot) takeConsumer(c *Consumer) bool {
	consumerID := c.GetID()
	if _, exists := ts.consumerStates[consumerID]; exists {
		return false
	}
	ts.consumerStates[consumerID] = &ConsumerState{
		consumer: c,
		aNode:    c.GetNode(),
	}
	return true
}

// Reset : reset the snapshot data
func (ts *TreeSnapshot) Reset() {
	ts.allChangedConsumers = make([]*Consumer, 0)
	ts.nodeStates = make(map[string]*NodeState)
	ts.consumerStates = make(map[string]*ConsumerState)
	ts.activeConsumers = make(map[string]*Consumer)

	ts.preemptedConsumers = make([]string, 0)
	ts.preemptedConsumersArray = make([]*Consumer, 0)
}
