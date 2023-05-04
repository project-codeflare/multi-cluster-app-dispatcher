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
)

// Consumer : A tree consumer
type Consumer struct {
	//  a unique id for the consumer
	id string
	// the tree ID for the consumer
	treeID string
	// the group ID for the consumer (ID of the leaf node)
	groupID string
	// resources requested by the consumer
	request *Allocation
	// priority of the consumer
	priority int
	// type of the consumer
	cType int
	// cannot preempt this consumer
	unPreemptable bool
	// node the consumer is assigned to
	aNode *QuotaNode
}

// NewConsumer : create a consumer
func NewConsumer(id string, treeID string, groupID string, request *Allocation, priority int,
	cType int, unPreemptable bool) *Consumer {
	return &Consumer{
		id:            id,
		treeID:        treeID,
		groupID:       groupID,
		request:       request,
		priority:      priority,
		cType:         cType,
		unPreemptable: unPreemptable,
	}
}

// GetID : get the id of the consumer
func (c *Consumer) GetID() string {
	return c.id
}

// GetTreeID : get the treeID of the consumer
func (c *Consumer) GetTreeID() string {
	return c.treeID
}

// SetTreeID : set the groupID of the consumer
func (c *Consumer) SetTreeID(treeID string) {
	c.treeID = treeID
}

// GetGroupID : get the treeID of the consumer
func (c *Consumer) GetGroupID() string {
	return c.groupID
}

// SetGroupID : set the groupID of the consumer
func (c *Consumer) SetGroupID(groupID string) {
	c.groupID = groupID
}

// GetRequest : get the request demand of the consumer
func (c *Consumer) GetRequest() *Allocation {
	return c.request
}

// GetPriority : get the priority of the consumer
func (c *Consumer) GetPriority() int {
	return c.priority
}

// GetType : get the type of the consumer
func (c *Consumer) GetType() int {
	return c.cType
}

// IsUnPreemptable : is the consumer preemptable
func (c *Consumer) IsUnPreemptable() bool {
	return c.unPreemptable
}

// SetUnPreemptable : set the consumer unpreemptability
func (c *Consumer) SetUnPreemptable(unPreemptable bool) {
	c.unPreemptable = unPreemptable
}

// GetNode : get the allocated node for the consumer
func (c *Consumer) GetNode() *QuotaNode {
	return c.aNode
}

// SetNode : set the allocated node for the consumer
func (c *Consumer) SetNode(aNode *QuotaNode) {
	c.aNode = aNode
}

// IsAllocated : is consumer allocated on tree
func (c *Consumer) IsAllocated() bool {
	return c.aNode != nil
}

// String : a print out of the consumer
func (c *Consumer) String() string {
	var b bytes.Buffer
	b.WriteString("Consumer: ")
	fmt.Fprintf(&b, "ID=%s; ", c.id)
	fmt.Fprintf(&b, "treeID=%s; ", c.treeID)
	fmt.Fprintf(&b, "groupID=%s; ", c.groupID)
	fmt.Fprintf(&b, "priority=%d; ", c.priority)
	fmt.Fprintf(&b, "type=%d; ", c.cType)
	fmt.Fprintf(&b, "unPreemptable=%v; ", c.unPreemptable)
	fmt.Fprintf(&b, "request=%s; ", c.request)
	var nodeID string
	if c.aNode != nil {
		nodeID = c.aNode.GetID()
	} else {
		nodeID = "null"
	}
	fmt.Fprintf(&b, "aNode=%s; ", nodeID)
	return b.String()
}
