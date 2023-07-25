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
package core

import (
	"bytes"
	"fmt"

	"k8s.io/klog/v2"
)

// ForestController : controller for multiple quota trees
type ForestController struct {
	// map of tree controllers, one per tree
	controllers map[string]*Controller
}

// NewForestController : create a multiple tree controller
func NewForestController() *ForestController {
	return &ForestController{
		controllers: make(map[string]*Controller),
	}
}

// AddController : add a quota tree controller
func (fc *ForestController) AddController(controller *Controller) bool {
	if controller != nil {
		treeName := controller.GetTreeName()
		if _, exists := fc.controllers[treeName]; !exists {
			fc.controllers[treeName] = controller
			return true
		}
	}
	return false
}

// DeleteController : delete a quota tree controller
func (fc *ForestController) DeleteController(treeName string) bool {
	return fc.DeleteTree(treeName)
}

// AddTree : add a quota tree
func (fc *ForestController) AddTree(tree *QuotaTree) bool {
	treeName := tree.GetName()
	if fc.controllers[treeName] != nil {
		return false
	}
	fc.controllers[treeName] = NewController(tree)
	return true
}

// DeleteTree : delete a quota tree
func (fc *ForestController) DeleteTree(treeName string) bool {
	if fc.controllers[treeName] == nil {
		return false
	}
	delete(fc.controllers, treeName)
	return true
}

// GetResourceNames : get resource names (null if no tree)
func (fc *ForestController) GetResourceNames() map[string][]string {
	rNmaesMap := make(map[string][]string)
	for treeName, controller := range fc.controllers {
		rNmaesMap[treeName] = controller.tree.GetResourceNames()
	}
	return rNmaesMap
}

// IsConsumerAllocated : check if a consumer is already allocated
func (fc *ForestController) IsConsumerAllocated(id string) bool {
	for _, controller := range fc.controllers {
		if !controller.IsConsumerAllocated(id) {
			return false
		}
	}
	return true
}

// IsAllocated : check if there are consumers already allocated
func (fc *ForestController) IsAllocated() bool {
	for _, controller := range fc.controllers {
		if controller.IsAllocated() {
			return true
		}
	}
	return false
}

// Allocate : allocate a consumer
func (fc *ForestController) Allocate(forestConsumer *ForestConsumer) *AllocationResponse {

	consumerID := forestConsumer.GetID()
	consumers := forestConsumer.GetConsumers()
	var b bytes.Buffer

	fmt.Fprintf(&b, "Multi-quota allocation for consumer %s:\n", consumerID)
	for treeName, consumer := range consumers {
		fmt.Fprintf(&b, treeName+": {")
		fmt.Fprintf(&b, "groupID=%s; ", consumer.GetGroupID())
		fmt.Fprintf(&b, "request=%v; ", consumer.GetRequest().GetValue())
		fmt.Fprintf(&b, "priority=%d; ", consumer.GetPriority())
		fmt.Fprintf(&b, "type=%d; ", consumer.GetType())
		fmt.Fprintf(&b, "unPreemptable=%v; ", consumer.IsUnPreemptable())
		fmt.Fprintln(&b, "}")
	}
	fmt.Fprintln(&b)
	klog.V(4).Info(b.String())

	allocResponse := NewAllocationResponse(consumerID)

	/*
	 * holders of progress through processing the trees
	 */
	processedTrees := make([]string, 0)
	deletedConsumers := make([]([]*Consumer), 0)
	preemptedConsumers := make([]([]string), 0)

	/*
	 * process trees sequentially
	 */
	for treeName, consumer := range consumers {

		controller := fc.controllers[treeName]
		groupID := consumer.GetGroupID()
		allocRequested := consumer.GetRequest()
		if controller == nil || len(groupID) == 0 || allocRequested.GetSize() != controller.GetQuotaSize() {
			var msg bytes.Buffer
			if controller == nil {
				fmt.Fprintf(&msg, "Unknown error processing quota designation '%s'", treeName)
			} else if len(groupID) == 0 {
				fmt.Fprintf(&msg, "No quota designations provided for '%s'", treeName)
			} else {
				fmt.Fprintf(&msg, "Expected %d resources for quota designations '%s', received %d",
					controller.GetQuotaSize(), treeName, allocRequested.GetSize())
			}
			return fc.failureRecover(consumerID, processedTrees, deletedConsumers, msg.String())
		}

		/*
		 * delete preempted consumers in previously processed trees from this tree
		 */
		treeDeletedConsumers := make([]*Consumer, 0)
		numProcessedTrees := len(processedTrees)
		if numProcessedTrees > 0 {
			for _, cj := range deletedConsumers[numProcessedTrees-1] {
				cjID := cj.GetID()
				c := controller.GetConsumer(cjID)
				if c != nil {
					treeDeletedConsumers = append(treeDeletedConsumers, c)
					controller.DeAllocate(cjID)
				}
			}
		}

		/*
		 * allocate consumer on this tree
		 */
		treeAllocResponse := controller.Allocate(consumer)

		if treeAllocResponse.IsAllocated() {

			/*
			 * allocation succeeded
			 */
			processedTrees = append(processedTrees, treeName)
			treeDeletedConsumers = append(treeDeletedConsumers, controller.GetPreemptedConsumersArray()...)
			deletedConsumers = append(deletedConsumers, treeDeletedConsumers)
			preemptedConsumers = append(preemptedConsumers, treeAllocResponse.GetPreemptedIds())
			allocResponse.Merge(treeAllocResponse)

		} else {

			/*
			 * allocation failed - undo deletions of prior preempted consumers and recover
			 */
			// TODO: make use of forest snapshot to recover
			for _, c := range treeDeletedConsumers {
				controller.Allocate(c)
			}
			return fc.failureRecover(consumerID, processedTrees, deletedConsumers, treeAllocResponse.GetMessage())
		}
	}

	/*
	 * delete preempted consumers (which had not been deleted) from all trees
	 */
	for i, treeName := range processedTrees {
		treeQM := fc.controllers[treeName]
		if treeQM == nil {
			continue
		}
		for j := i + 1; j < len(preemptedConsumers); j++ {
			pcs := preemptedConsumers[j]
			for _, pc := range pcs {
				treeQM.DeAllocate(pc)
			}
		}
	}

	b.Reset()
	fmt.Fprintf(&b, "Multi-quota allocation for consumer: %s\n", consumerID)
	fmt.Fprint(&b, allocResponse.String())
	fmt.Fprintln(&b)
	klog.V(4).Info(b.String())
	return allocResponse
}

// failureRecover : undo alterations to trees in case of allocation failure
func (fc *ForestController) failureRecover(consumerID string, processedTrees []string,
	deletedConsumers []([]*Consumer), msg string) *AllocationResponse {

	for i, treeName := range processedTrees {
		controller := fc.controllers[treeName]
		if controller != nil {
			controller.DeAllocate(consumerID)
			treeDeletedConsumers := deletedConsumers[i]
			for _, consumer := range treeDeletedConsumers {
				controller.Allocate(consumer)
			}
		}
	}

	failedResponse := NewAllocationResponse(consumerID)
	failedResponse.SetAllocated(false)

	// Pickup message from the allocation request.
	failedResponse.SetMessage(msg)

	klog.V(4).Infof("Multi-quota allocation for consumer: %s\n", consumerID)
	klog.V(4).Infoln(failedResponse)

	return failedResponse
}

// TryAllocate : try allocating a consumer by taking a snapshot before attempting allocation
func (fc *ForestController) TryAllocate(forestConsumer *ForestConsumer) *AllocationResponse {
	consumerID := forestConsumer.GetID()
	consumers := forestConsumer.GetConsumers()
	allocResponse := NewAllocationResponse(consumerID)

	// take a snapshot of the forest
	for treeName, consumer := range consumers {
		var msg bytes.Buffer
		controller := fc.controllers[treeName]
		controller.treeSnapshot = NewTreeSnapshot(controller.tree, consumer)
		// TODO: limit the number of potentially affected consumers by the allocation
		if !controller.treeSnapshot.Take(controller, controller.consumers) {
			fmt.Fprintf(&msg, "Failed to take a state snapshot of tree '%s'", controller.GetTreeName())
			treeAllocResponse := NewAllocationResponse(consumer.GetID())
			preemptedIds := make([]string, 0)
			treeAllocResponse.Append(false, msg.String(), &preemptedIds)
			allocResponse.Merge(treeAllocResponse)
			return allocResponse
		}
	}

	ar := fc.Allocate(forestConsumer)
	allocResponse.Merge(ar)
	return allocResponse
}

// UndoAllocate : undo the most recent allocation trial
func (fc *ForestController) UndoAllocate(forestConsumer *ForestConsumer) bool {
	klog.V(4).Infof("Multi-quota undo allocation of consumer: %s\n", forestConsumer.GetID())
	consumers := forestConsumer.GetConsumers()
	success := true
	for treeName, consumer := range consumers {
		controller := fc.controllers[treeName]
		treeSuccess := controller.UndoAllocate(consumer)
		success = success && treeSuccess
	}
	return success
}

// ForceAllocate : force allocate a consumer on a given set of nodes on trees;
// no recovery if not allocated on some trees; partial allocation allowed
func (fc *ForestController) ForceAllocate(forestConsumer *ForestConsumer, nodeIDs map[string]string) *AllocationResponse {
	consumerID := forestConsumer.GetID()
	consumers := forestConsumer.GetConsumers()
	allocResponse := NewAllocationResponse(consumerID)
	for treeName, controller := range fc.controllers {
		if consumer, consumerExists := consumers[treeName]; consumerExists {
			if nodeID, nodeExists := nodeIDs[treeName]; nodeExists {
				resp := controller.ForceAllocate(consumer, nodeID)
				allocResponse.Merge(resp)
			}
		}
	}
	return allocResponse
}

// DeAllocate : deallocate a consumer
func (fc *ForestController) DeAllocate(consumerID string) bool {
	status := true
	for _, controller := range fc.controllers {
		if !controller.DeAllocate(consumerID) {
			status = false
		}
	}
	return status
}

// GetControllers : get a map of tree controllers of the trees
func (fc *ForestController) GetControllers() map[string]*Controller {
	return fc.controllers
}

// GetTreeNames : get the names of the trees
func (fc *ForestController) GetTreeNames() []string {
	names := make([]string, 0, len(fc.controllers))
	for name := range fc.controllers {
		names = append(names, name)
	}
	return names
}

// GetQuotaTrees : get a list of the quota trees
func (fc *ForestController) GetQuotaTrees() []*QuotaTree {
	trees := make([]*QuotaTree, 0, len(fc.controllers))
	for _, controller := range fc.controllers {
		trees = append(trees, controller.GetTree())
	}
	return trees
}

// IsReady : check if tree(s) have been created
func (fc *ForestController) IsReady() bool {
	if len(fc.controllers) == 0 {
		return false
	}
	trees := fc.GetQuotaTrees()
	for _, tree := range trees {
		if tree == nil {
			return false
		}
	}
	return true
}

// UpdateTrees : update all trees from caches;
// returns nil if able to allocate all consumers onto updated trees,
// otherwise a list of the IDs of unallocated consumers is returned
func (fc *ForestController) UpdateTrees(treeCacheList []*TreeCache) (unAllocatedConsumerIDs []string,
	responseMap map[string]*TreeCacheCreateResponse) {

	// keep track of consumers unable to allocate on new tree
	unAllocatedConsumerIDs = make([]string, 0)
	mapUnAllocatedConsumerIDs := make(map[string]bool)
	noteUnAllocatedConsumer := func(cID string) {
		mapUnAllocatedConsumerIDs[cID] = true
		unAllocatedConsumerIDs = append(unAllocatedConsumerIDs, cID)
	}

	responseMap = make(map[string]*TreeCacheCreateResponse)

	// create map of tree caches (for ease of reference by tree name)
	treeCacheMap := make(map[string]*TreeCache)
	for _, treeCache := range treeCacheList {
		treeCacheMap[treeCache.GetTreeName()] = treeCache
	}

	// delete controllers with obsolete trees
	for _, treeName := range fc.GetTreeNames() {
		if _, exists := treeCacheMap[treeName]; !exists {
			fc.DeleteTree(treeName)
		}
	}

	// add controllers for new trees
	for treeName, treeCache := range treeCacheMap {
		if _, exists := fc.controllers[treeName]; !exists {
			tree, response := treeCache.CreateTree()
			fc.AddTree(tree)
			responseMap[treeName] = response
		}
	}

	// update controllers
	for _, controller := range fc.controllers {
		treeName := controller.GetTreeName()
		cache := treeCacheMap[treeName]
		if cache != nil {
			cIDs, response := controller.UpdateTree(cache)
			for _, id := range cIDs {
				if !mapUnAllocatedConsumerIDs[id] {
					noteUnAllocatedConsumer(id)
				}
			}
			responseMap[treeName] = response
		}
	}

	if len(unAllocatedConsumerIDs) == 0 {
		return nil, responseMap
	}

	// remove unallocated consumers
	for _, controller := range fc.controllers {
		for _, id := range unAllocatedConsumerIDs {
			controller.DeAllocate(id)
		}
	}
	return unAllocatedConsumerIDs, responseMap
}

// String : printout
func (fc *ForestController) String() string {
	var b bytes.Buffer
	b.WriteString("ForestController: \n")
	for treeName, controller := range fc.controllers {
		tree := controller.GetTree()
		if tree != nil {
			b.WriteString(tree.String())
		} else {
			b.WriteString("Tree " + treeName + " is null")
		}
		b.WriteString("\n")
	}
	return b.String()
}
