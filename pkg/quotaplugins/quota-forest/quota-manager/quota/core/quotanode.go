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
	"strings"
	"unsafe"

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
	"k8s.io/klog/v2"
)

// QuotaNode : a node in the quota tree
type QuotaNode struct {
	//extends Node
	tree.Node
	// quota associated with the node
	quota *Allocation
	// quota limit is hard
	isHard bool
	// amount allocated on the node
	allocated *Allocation
	// list of consumers allocated on the node
	consumers []*Consumer
}

// NewQuotaNode : create a quota node
func NewQuotaNode(id string, quota *Allocation) (*QuotaNode, error) {
	if len(id) == 0 || quota == nil {
		return nil, fmt.Errorf("invalid parameters")
	}
	alloc, _ := NewAllocation(quota.GetSize())
	qn := &QuotaNode{
		Node:      *tree.NewNode(id),
		quota:     quota,
		isHard:    false,
		allocated: alloc,
		consumers: make([]*Consumer, 0),
	}
	return qn, nil
}

// NewQuotaNodeHard : create a quota node
func NewQuotaNodeHard(id string, quota *Allocation, isHard bool) (*QuotaNode, error) {
	qn, err := NewQuotaNode(id, quota)
	if err != nil {
		return nil, err
	}
	qn.isHard = isHard
	return qn, nil
}

// CanFit : check if a consumer request can fit on this node
func (qn *QuotaNode) CanFit(c *Consumer) bool {
	return c.GetRequest().Fit(qn.allocated, qn.quota)
}

// AddRequest : add request of consumer to allocated amount on this node (acquire)
func (qn *QuotaNode) AddRequest(c *Consumer) {
	qn.allocated.Add(c.GetRequest())
}

// SubtractRequest : subtract request of consumer from allocated amount on this node (release)
func (qn *QuotaNode) SubtractRequest(c *Consumer) {
	qn.allocated.Subtract(c.GetRequest())
}

// AddConsumer : add a consumer to the consumers list
func (qn *QuotaNode) AddConsumer(c *Consumer) bool {
	// TODO: need more efficient data structure
	cid := c.GetID()
	for _, ci := range qn.consumers {
		if ci.GetID() == cid {
			return false
		}
	}
	qn.consumers = append(qn.consumers, c)
	return true
}

// RemoveConsumer : remove a consumer from the consumers list
func (qn *QuotaNode) RemoveConsumer(c *Consumer) bool {
	// TODO: need more efficient data structure
	cid := c.GetID()
	for i, ci := range qn.consumers {
		if ci.GetID() == cid {
			qn.consumers = append(qn.consumers[:i], qn.consumers[i+1:]...)
			return true
		}
	}
	return false
}

// Allocate : allocate a consumer on this node, assuming consumer request fits
func (qn *QuotaNode) Allocate(c *Consumer) {
	qn.AddRequest(c)
	qn.AddConsumer(c)
	c.SetNode(qn)
}

// SlideDown : slide down potential consumers from parent node
func (qn *QuotaNode) SlideDown() {
	parent := qn.GetParent()
	if parent != nil {
		p := (*QuotaNode)(unsafe.Pointer(parent))
		// make copy of parent consumers list, as we may make changes to it
		list := make([]*Consumer, len(p.GetConsumers()))
		copy(list, p.GetConsumers())
		qnID := qn.GetID()
		for _, c := range list {
			if qn.HasLeaf(c) && qn.CanFit(c) {
				p.RemoveConsumer(c)
				qn.Allocate(c)
				klog.V(4).Infof("*** Consumer %s slid down to node %s \n", c.GetID(), qnID)
			}
		}
	}
}

// SlideUp : slide up potential consumers to parent node; return true if able to fit given
// consumer on this node after sliding up other consumers
func (qn *QuotaNode) SlideUp(c *Consumer, applyPriority bool, allocationRecovery *AllocationRecovery,
	preemptedConsumers *[]string) bool {

	if qn.isHard && !qn.IsRoot() {
		return false
	}

	success := false
	requested := c.GetRequest()
	priority := c.GetPriority()
	cType := c.GetType()
	candidates := make([]*Consumer, 0)
	scratch := qn.allocated.Clone()

	// TODO: ordering of consumers to slide up
	for _, consumer := range qn.consumers {
		if !applyPriority || priority > consumer.GetPriority() {

			if (consumer.IsUnPreemptable() || consumer.GetType() != cType) && qn.IsRoot() {
				continue
			}

			scratch.Subtract(consumer.GetRequest())
			candidates = append(candidates, consumer)
			if requested.Fit(scratch, qn.quota) {
				success = true
				break
			}
		}
	}

	if success {
		p := qn.GetParent()
		parent := (*QuotaNode)(unsafe.Pointer(p))
		for _, consumer := range candidates {
			allocationRecovery.AlteredConsumer(consumer)
			qn.SubtractRequest(consumer)
			qn.RemoveConsumer(consumer)
			consumer.SetNode(parent)
			if parent != nil {
				parent.AddConsumer(consumer)
				klog.V(4).Infof("*** Consumer %s slid up to node %s \n", consumer.GetID(), parent.GetID())
			} else {
				consumerID := consumer.GetID()
				*preemptedConsumers = append(*preemptedConsumers, consumerID)
				klog.V(4).Infof("*** Consumer %s is preempted! \n", consumerID)
			}
		}
	}
	return success
}

// HasLeaf : check if the leaf node of a consumer is also a leaf of the subtree formed from this node as a root
func (qn *QuotaNode) HasLeaf(c *Consumer) bool {
	groupID := c.GetGroupID()
	for _, leaf := range qn.GetLeaves() {
		if leaf.GetID() == groupID {
			return true
		}
	}
	return false
}

// IsHard :
func (qn *QuotaNode) IsHard() bool {
	return qn.isHard
}

// GetQuota :
func (qn *QuotaNode) GetQuota() *Allocation {
	return qn.quota
}

// SetQuota :
func (qn *QuotaNode) SetQuota(quota *Allocation) {
	qn.quota = quota
}

// GetAllocated :
func (qn *QuotaNode) GetAllocated() *Allocation {
	return qn.allocated
}

// SetAllocated :
func (qn *QuotaNode) SetAllocated(alloc *Allocation) {
	qn.allocated = alloc
}

// GetConsumers :
func (qn *QuotaNode) GetConsumers() []*Consumer {
	return qn.consumers
}

// SetConsumers :
func (qn *QuotaNode) SetConsumers(consumers []*Consumer) {
	qn.consumers = consumers
}

// String : print node with a specified level of indentation
func (qn *QuotaNode) String(level int) string {
	var b bytes.Buffer
	prefix := strings.Repeat("--", level)
	prefix += "|"
	fmt.Fprintf(&b, "%s: ", prefix+qn.ID)
	fmt.Fprintf(&b, "isHard=%v; ", qn.isHard)
	fmt.Fprintf(&b, "quota=%s; ", qn.quota)
	fmt.Fprintf(&b, "allocated=%s; ", qn.allocated)

	fmt.Fprintf(&b, "consumers={ ")
	ids := make([]string, 0, len(qn.consumers))
	for _, c := range qn.consumers {
		ids = append(ids, c.GetID())
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&b, "%s ", id)
	}
	fmt.Fprintf(&b, "}")
	fmt.Fprintf(&b, "\n")

	if !qn.IsLeaf() {
		children := qn.GetChildren()
		list := make([]string, 0, len(children))
		childrenMap := make(map[string]*QuotaNode)
		for _, cn := range children {
			id := cn.GetID()
			qcn := (*QuotaNode)(unsafe.Pointer(cn))
			list = append(list, id)
			childrenMap[id] = qcn
		}
		sort.Strings(list)
		for _, id := range list {
			fmt.Fprintf(&b, "%s", childrenMap[id].String(level+1))
		}
	}
	return b.String()
}
