/*
Copyright 2022, 2023 The Multi-Cluster App Dispatcher Authors.

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
	"bytes"
	"fmt"
	"sort"

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
	"k8s.io/klog/v2"
)

// Controller : controller of a quota tree
type Controller struct {
	// the quota tree
	tree *QuotaTree
	// consumers already allocated (running)
	consumers map[string]*Consumer

	// list of IDs of preempted consumers due to previous allocation request
	preemptedConsumers []string
	// list of preempted consumer objects due to previous allocation request
	preemptedConsumersArray []*Consumer

	// snapshot of tree state to undo most recent allocation
	treeSnapshot *TreeSnapshot
}

// NewController : create a quota controller
func NewController(tree *QuotaTree) *Controller {
	return &Controller{
		tree:                    tree,
		consumers:               make(map[string]*Consumer),
		preemptedConsumers:      make([]string, 0),
		preemptedConsumersArray: make([]*Consumer, 0),
		treeSnapshot:            nil,
	}
}

// Allocate : allocate a consumer
func (controller *Controller) Allocate(consumer *Consumer) *AllocationResponse {
	klog.V(4).Infof("Allocating consumer %s:\n", consumer.GetID())
	controller.preemptedConsumers = make([]string, 0)
	controller.preemptedConsumersArray = make([]*Consumer, 0)
	var allocationMessage bytes.Buffer
	fmt.Fprintf(&allocationMessage, "")
	allocated := controller.tree.Allocate(consumer, &controller.preemptedConsumers)
	if allocated {
		controller.consumers[consumer.GetID()] = consumer
		for _, id := range controller.preemptedConsumers {
			c := controller.consumers[id]
			if c != nil {
				controller.preemptedConsumersArray = append(controller.preemptedConsumersArray, c)
				controller.Remove(id)
			}
		}
	} else {
		fmt.Fprintf(&allocationMessage, "Failed to allocate quota on quota designation '%s'", controller.GetTreeName())
	}
	controller.PrintState(consumer, allocated)
	preemptedIds := make([]string, len(controller.preemptedConsumers))
	copy(preemptedIds, controller.preemptedConsumers)
	allocResponse := NewAllocationResponse(consumer.GetID())
	allocResponse.Append(allocated, allocationMessage.String(), &preemptedIds)
	return allocResponse
}

// TryAllocate : try allocating a consumer by taking a snapshot before attempting allocation
func (controller *Controller) TryAllocate(consumer *Consumer) *AllocationResponse {
	controller.treeSnapshot = NewTreeSnapshot(controller.tree, consumer)
	if !controller.treeSnapshot.Take(controller, nil) {
		var allocationMessage bytes.Buffer
		fmt.Fprintf(&allocationMessage, "Failed to take a state snapshot of tree '%s'", controller.GetTreeName())
		allocResponse := NewAllocationResponse(consumer.GetID())
		preemptedIds := make([]string, 0)
		allocResponse.Append(false, allocationMessage.String(), &preemptedIds)
		return allocResponse
	}
	return controller.Allocate(consumer)
}

// UndoAllocate : undo the most recent allocation trial
func (controller *Controller) UndoAllocate(consumer *Consumer) bool {
	success := true
	defer controller.PrintState(consumer, success)
	if ts := controller.treeSnapshot; ts != nil && ts.targetConsumer.GetID() == consumer.GetID() {
		ts.Reinstate(controller)
	} else {
		success = false
	}
	return success
}

// ForceAllocate : force allocate a consumer on a given node
func (controller *Controller) ForceAllocate(consumer *Consumer, nodeID string) *AllocationResponse {
	consumerID := consumer.GetID()
	klog.V(4).Infof("Force allocating consumer %s on node %s:\n", consumerID, nodeID)
	allocResponse := NewAllocationResponse(consumerID)
	allocated := controller.GetTree().ForceAllocate(consumer, nodeID)
	controller.PrintState(consumer, allocated)
	if !allocated {
		allocResponse.SetAllocated(false)
		msg := fmt.Sprintf("failed force allocate consumer %v on node %v",
			consumerID, nodeID)
		allocResponse.SetMessage(msg)
		return allocResponse
	}
	controller.consumers[consumer.GetID()] = consumer
	return allocResponse
}

// DeAllocate : deallocate a consumer
func (controller *Controller) DeAllocate(consumerID string) bool {
	klog.V(4).Infof("Deallocating consumer %s:\n", consumerID)
	controller.preemptedConsumers = make([]string, 0)
	consumer := controller.consumers[consumerID]
	if consumer != nil {
		delete(controller.consumers, consumerID)
		deallocated := controller.tree.DeAllocate(consumer)
		controller.PrintState(consumer, deallocated)
		return deallocated
	}
	klog.V(4).Infoln("Consumer " + consumerID + " is not allocated")
	return false
}

// GetConsumers : get a map of consumers in controller
func (controller *Controller) GetConsumers() map[string]*Consumer {
	return controller.consumers
}

// GetPreemptedConsumers : get a list of the preempted consumer IDs
func (controller *Controller) GetPreemptedConsumers() []string {
	return controller.preemptedConsumers
}

// GetPreemptedConsumersArray : get a list of the preempted consumer objects
func (controller *Controller) GetPreemptedConsumersArray() []*Consumer {
	return controller.preemptedConsumersArray
}

// Remove : remove a consumer
func (controller *Controller) Remove(id string) bool {
	_, exists := controller.consumers[id]
	if exists {
		delete(controller.consumers, id)
	}
	return exists
}

// GetTree : the quota tree
func (controller *Controller) GetTree() *QuotaTree {
	return controller.tree
}

// GetTreeName : the name of the quota tree, empty if nil
func (controller *Controller) GetTreeName() string {
	if controller.tree == nil {
		return ""
	}
	return controller.tree.GetName()
}

// GetConsumer : retrieve a consumer by id
func (controller *Controller) GetConsumer(id string) *Consumer {
	return controller.consumers[id]
}

// IsConsumerAllocated : check if a consumer is already allocated
func (controller *Controller) IsConsumerAllocated(id string) bool {
	return controller.consumers[id] != nil
}

// IsAllocated : check if there are consumers already allocated
func (controller *Controller) IsAllocated() bool {
	return len(controller.consumers) > 0
}

// GetConsumerIDs : get IDs of all consumers allocated
func (controller *Controller) GetConsumerIDs() []string {
	allIDs := make([]string, len(controller.consumers))
	i := 0
	for id := range controller.consumers {
		allIDs[i] = id
		i++
	}
	return allIDs
}

// GetQuotaSize : get dimension of quota array (number of resources)
func (controller *Controller) GetQuotaSize() int {
	if controller.tree == nil {
		return 0
	}
	return controller.tree.GetQuotaSize()
}

// GetResourceNames : get resource names (null if no tree)
func (controller *Controller) GetResourceNames() []string {
	if controller.tree == nil {
		return nil
	}
	return controller.tree.GetResourceNames()
}

// UpdateTree : update tree from cache;
// returns nil if able to allocate all consumers onto updated tree,
// otherwise a list of the IDs of unallocated consumers is returned
func (controller *Controller) UpdateTree(treeCache *TreeCache) (unAllocatedConsumerIDs []string, response *TreeCacheCreateResponse) {

	// get new tree from cache
	var newTree *QuotaTree
	newTree, response = treeCache.CreateTree()

	// keep track of consumers unable to allocate on new tree
	unAllocatedConsumerIDs = make([]string, 0)
	noteUnAllocatedConsumer := func(cID string) {
		unAllocatedConsumerIDs = append(unAllocatedConsumerIDs, cID)
	}

	// move all consumers to new tree
	for _, c := range controller.consumers {
		groupID := c.GetGroupID()
		// update name if changed
		renamedGroupID := treeCache.GetRenamedNode(groupID)
		if len(renamedGroupID) > 0 {
			groupID = renamedGroupID
			c.SetGroupID(groupID)
		}
		newGroupNode := newTree.GetNode(groupID)

		var newANode *tree.Node
		if aNode := c.GetNode(); aNode != nil {
			aNodeID := aNode.GetID()
			// update name if changed
			renamedANodeID := treeCache.GetRenamedNode(aNodeID)
			if len(renamedANodeID) > 0 {
				aNodeID = renamedANodeID
			}
			newANode = newTree.GetNode(aNodeID)
		}

		// newNode : the node to allocate the consumer
		var newNode *tree.Node
		if newGroupNode != nil {
			if newANode != nil && newANode.HasLeaf(groupID) {
				newNode = newANode
			} else {
				// aNode does not exist in the new tree, use group node instead
				newNode = newGroupNode
			}
		} else {
			// group node does not exist in the new tree, use root node
			newNode = newTree.GetRoot()
		}

		cID := c.GetID()
		if newNode == nil {
			// we should not have an empty tree
			noteUnAllocatedConsumer(cID)
			continue
		}

		if !newTree.ForceAllocate(c, newNode.GetID()) {
			noteUnAllocatedConsumer(cID)
			continue
		}
	}

	// set the tree as the new tree
	controller.tree = newTree

	// remove unallocated consumers
	if len(unAllocatedConsumerIDs) > 0 {
		for _, id := range unAllocatedConsumerIDs {
			delete(controller.consumers, id)
		}
		return unAllocatedConsumerIDs, response
	}
	return nil, response
}

// PrintState : print state of quota tree after consumer (de)allocation step
func (controller *Controller) PrintState(c *Consumer, status bool) {
	klog.V(4).Infoln(c)
	klog.V(4).Info(controller.tree)
	if status {
		klog.V(4).Infoln("Status = Success")
	} else {
		klog.V(4).Infoln("Status = Failure")
	}
	klog.V(4).Infof("Preempted Consumers: %v\n", controller.GetPreemptedConsumers())
	klog.V(4).Infoln()
}

// String : printout
func (controller *Controller) String() string {
	var b bytes.Buffer
	b.WriteString("TreeController: \n")

	if controller.tree != nil {
		b.WriteString(controller.tree.String())
	} else {
		b.WriteString("null")
	}

	// print consumers sorted list
	b.WriteString("Consumers: \n")
	ids := make([]string, len(controller.consumers))
	i := 0
	for id := range controller.consumers {
		ids[i] = id
		i++
	}
	sort.Strings(ids)
	for _, id := range ids {
		b.WriteString(controller.consumers[id].String() + "\n")
	}

	return b.String()
}
