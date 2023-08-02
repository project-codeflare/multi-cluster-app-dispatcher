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
	"unsafe"

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
	"k8s.io/klog/v2"
)

// QuotaTree : a quota tree
type QuotaTree struct {
	// extends tree
	tree.Tree
	// unique name of the tree
	name string
	// names of quota resources
	resourceNames []string
}

// NewQuotaTree : create a quota tree
func NewQuotaTree(name string, root *QuotaNode, resourceNames []string) *QuotaTree {
	rootNode := (*tree.Node)(unsafe.Pointer(root))
	qt := &QuotaTree{
		Tree:          *tree.NewTree(rootNode),
		name:          name,
		resourceNames: resourceNames,
	}
	return qt
}

// Allocate : allocate a consumer request
func (qt *QuotaTree) Allocate(c *Consumer, preemptedConsumers *[]string) bool {
	groupID := c.GetGroupID()
	leafNode := qt.GetLeafNode(groupID)
	if leafNode == nil {
		klog.V(4).Infof("Consumer %s member of unknown group %s \n", c.GetID(), groupID)
		return false
	}

	allocationRecovery := NewAllocationRecovery(c)

	path := leafNode.GetPathToRoot()
	allocated := false
	hitHard := false
	attemptedNode := (*QuotaNode)(unsafe.Pointer(leafNode))
	for _, n := range path {
		node := (*QuotaNode)(unsafe.Pointer(n))
		attemptedNode = node
		hitHard = hitHard || node.IsHard()

		if !allocated {
			if node.CanFit(c) || node.SlideUp(c, true, allocationRecovery, preemptedConsumers) {
				node.Allocate(c)
				allocationRecovery.AlteredNode(node)
				allocated = true
			} else {
				if node.IsHard() {
					break
				}
			}
		} else {
			if node.CanFit(c) || node.SlideUp(c, false, allocationRecovery, preemptedConsumers) {
				node.AddRequest(c)
				allocationRecovery.AlteredNode(node)
			} else {
				allocationRecovery.Recover()
				*preemptedConsumers = make([]string, 0)
				allocated = false

				if hitHard {
					break
				} else {
					continue
				}
			}
		}
	}

	if allocated {
		klog.V(4).Infof("*** Consumer %s allocated on node %s \n", c.GetID(), c.GetNode().GetID())
	} else {
		klog.V(4).Infof("*** Consumer %s failed allocating on node %s \n", c.GetID(), attemptedNode.GetID())
	}

	/*
	 * If not allocated, attempt preempting lower priority consumers
	 */

	priority := c.GetPriority()
	cType := c.GetType()
	if !allocated && priority > 0 {
		klog.V(4).Infoln("*** Attempting to preempt lower priority consumers ...")
		allocationRecovery.Reset()
		n := len(path)
		foundit := false
		// traverse nodes from root down to leaf node
		for i := n - 1; i >= 0; i-- {
			node := (*QuotaNode)(unsafe.Pointer(path[i]))
			if !foundit {
				if node == attemptedNode {
					foundit = true
				} else {
					continue
				}
			}

			// make copy of consumers list, as we may make changes to it
			list := make([]*Consumer, len(node.GetConsumers()))
			copy(list, node.GetConsumers())
			for _, consumer := range list {
				if priority > consumer.GetPriority() && !consumer.IsUnPreemptable() &&
					consumer.GetType() == cType {

					node.RemoveConsumer(consumer)
					for j := i; j < n; j++ {
						qn := (*QuotaNode)(unsafe.Pointer(path[j]))
						qn.SubtractRequest(consumer)
					}
					allocationRecovery.AlteredConsumer(consumer)
					consumer.SetNode(nil)
					consumerId := consumer.GetID()
					*preemptedConsumers = append(*preemptedConsumers, consumerId)
					klog.V(4).Infof("*** Consumer %s is preempted! \n", consumerId)

					if attemptedNode.CanFit(c) {
						return qt.Allocate(c, preemptedConsumers)
					}
				}
			}
		}

		allocationRecovery.Recover()
		*preemptedConsumers = make([]string, 0)
		allocated = false
	}

	return allocated
}

// ForceAllocate : force allocate a consumer request on a given node
func (qt *QuotaTree) ForceAllocate(c *Consumer, nodeID string) bool {
	node := qt.GetNode(nodeID)
	if node == nil {
		klog.V(4).Infof("Attemting to force allocate consumer %s on unkown node %s \n", c.GetID(), nodeID)
		return false
	}

	// place consumer on node
	qNode := (*QuotaNode)(unsafe.Pointer(node))
	qNode.AddConsumer(c)
	c.SetNode(qNode)

	// update allocated resources from node to root
	path := node.GetPathToRoot()
	for _, n := range path {
		qn := (*QuotaNode)(unsafe.Pointer(n))
		qn.AddRequest(c)
	}
	return true
}

// DeAllocate : deallocate a consumer request
func (qt *QuotaTree) DeAllocate(c *Consumer) bool {
	node := c.GetNode()
	if node == nil || !node.RemoveConsumer(c) {
		klog.V(4).Infof("Consumer %s has no or inconsistent allocation node assignment \n", c.GetID())
		return false
	}

	path := node.GetPathToRoot()
	for _, n := range path {
		qn := (*QuotaNode)(unsafe.Pointer(n))
		qn.SubtractRequest(c)
		qn.SlideDown()
	}
	c.SetNode(nil)
	return true
}

// GetName :
func (qt *QuotaTree) GetName() string {
	return qt.name
}

// GetResourceNames :
func (qt *QuotaTree) GetResourceNames() []string {
	return qt.resourceNames
}

// GetQuotaSize :
func (qt *QuotaTree) GetQuotaSize() int {
	return len(qt.resourceNames)
}

// GetNodes : get a map of all quota nodes in the tree
func (qt *QuotaTree) GetNodes() map[string]*QuotaNode {
	nodeMap := make(map[string]*QuotaNode)
	for _, n := range qt.Tree.GetNodeListBFS() {
		nodeMap[n.GetID()] = (*QuotaNode)(unsafe.Pointer(n))
	}
	return nodeMap
}

// StringSimply :
func (qt *QuotaTree) StringSimply() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "Tree %s: ", qt.name)
	t := (*tree.Tree)(unsafe.Pointer(qt))
	b.WriteString(t.String())
	return b.String()
}

// String :
func (qt *QuotaTree) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "QuotaTree %s: \n", qt.name)
	t := (*tree.Tree)(unsafe.Pointer(qt))
	r := (*QuotaNode)(unsafe.Pointer(t.GetRoot()))
	if r != nil {
		b.WriteString(r.String(0))
	} else {
		b.WriteString("null")
	}
	return b.String()
}
