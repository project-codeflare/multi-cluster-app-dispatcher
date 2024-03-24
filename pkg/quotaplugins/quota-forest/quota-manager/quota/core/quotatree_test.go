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
	"testing"
	"unsafe"

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/strings/slices"
)

// TestQuotaTreeAllocation : test quota tree allocation and de-allocation
func TestQuotaTreeAllocation(t *testing.T) {
	// create a test quota tree
	testTreeName := "test-tree"
	resourceNames := []string{"CPU"}
	testRootNode := createTestRootNode(t)
	testQuotaTree := NewQuotaTree(testTreeName, testRootNode, resourceNames)
	assert.NotNil(t, testQuotaTree, "Expecting non-nil quota tree")

	// create consumers
	consumer1 := NewConsumer("C1", testTreeName, "B", &Allocation{x: []int{2}}, 0, 0, false)
	assert.NotNil(t, consumer1, "Expecting non-nil consumer1")
	consumer2 := NewConsumer("C2", testTreeName, "B", &Allocation{x: []int{1}}, 0, 0, false)
	assert.NotNil(t, consumer2, "Expecting non-nil consumer2")
	consumer3 := NewConsumer("C3", testTreeName, "C", &Allocation{x: []int{1}}, 0, 0, false)
	assert.NotNil(t, consumer3, "Expecting non-nil consumer3")
	consumer4P := NewConsumer("C4P", testTreeName, "B", &Allocation{x: []int{2}}, 1, 0, false)
	assert.NotNil(t, consumer4P, "Expecting non-nil consumer4P")
	consumerLarge := NewConsumer("CL", testTreeName, "C", &Allocation{x: []int{4}}, 0, 0, false)
	assert.NotNil(t, consumerLarge, "Expecting non-nil consumerLarge")

	// allocate consumers
	preemptedConsumers := &[]string{}

	// C1 -> A (does not fit on requested node B)
	allocated1 := testQuotaTree.Allocate(consumer1, preemptedConsumers)
	assert.True(t, allocated1, "Expecting consumer 1 to be allocated")
	node1 := consumer1.GetNode().GetID()
	assert.Equal(t, "A", node1, "Expecting consumer 1 to be allocated on node A")

	// C2 -> B
	allocated2 := testQuotaTree.Allocate(consumer2, preemptedConsumers)
	assert.True(t, allocated2, "Expecting consumer 2 to be allocated")
	node2 := consumer2.GetNode().GetID()
	assert.Equal(t, "B", node2, "Expecting consumer 2 to be allocated on node B")

	// C3 -> C (preempts C1 as C3 fits on its requested node C)
	allocated3 := testQuotaTree.Allocate(consumer3, preemptedConsumers)
	assert.True(t, allocated3, "Expecting consumer 3 to be allocated")
	node3 := consumer3.GetNode().GetID()
	assert.Equal(t, "C", node3, "Expecting consumer 3 to be allocated on node C")
	consumer1Preempted := slices.Contains(*preemptedConsumers, "C1")
	assert.True(t, consumer1Preempted, "Expecting consumer 1 to get preempted")

	// C4P -> A (high priority C4P preempts C2)
	allocated4P := testQuotaTree.Allocate(consumer4P, preemptedConsumers)
	assert.True(t, allocated4P, "Expecting consumer 4P to be allocated")
	node4P := consumer4P.GetNode().GetID()
	assert.Equal(t, "A", node4P, "Expecting consumer 4P to be allocated on node A")
	consumer2Preempted := slices.Contains(*preemptedConsumers, "C2")
	assert.True(t, consumer2Preempted, "Expecting consumer 2 to get preempted")

	// CL large consumer does not fit
	allocatedLarge := testQuotaTree.Allocate(consumerLarge, preemptedConsumers)
	assert.False(t, allocatedLarge, "Expecting large consumer not to be allocated")

	// CL -> C (large consumer allocated by force on specified node C, no preemptions)
	forceAllocatedLarge := testQuotaTree.ForceAllocate(consumerLarge, "C")
	assert.True(t, forceAllocatedLarge, "Expecting large consumer to be allocated by force")
	nodeLarge := consumerLarge.GetNode().GetID()
	assert.Equal(t, "C", nodeLarge, "Expecting large consumer to be force allocated on node C")

	// C3 de-allocated
	quotaNode3 := consumer3.GetNode()
	deallocated3 := testQuotaTree.DeAllocate(consumer3)
	assert.True(t, deallocated3, "Expecting consumer 3 to be de-allocated")
	node3x := consumer3.GetNode()
	assert.Nil(t, node3x, "Expecting consumer 3 to have a nil node")
	consumer3OnNode := consumerInList(consumer3, quotaNode3.GetConsumers())
	assert.False(t, consumer3OnNode, "Expecting consumer 3 to be removed from its allocated node")

	// C3 de-allocated again
	deallocated3Again := testQuotaTree.DeAllocate(consumer3)
	assert.False(t, deallocated3Again, "Expecting non-existing consumer 3 not to be de-allocated")
}

// createTestRootNode : create a test quota node as a root for a tree
func createTestRootNode(t *testing.T) *QuotaNode {

	// create three quota nodes A[2], B[1], and C[1]
	quotaNodeA, err := NewQuotaNode("A", &Allocation{x: []int{3}})
	assert.NoError(t, err, "No error expected when creating quota node A")

	quotaNodeB, err := NewQuotaNode("B", &Allocation{x: []int{1}})
	assert.NoError(t, err, "No error expected when creating quota node B")

	quotaNodeC, err := NewQuotaNode("C", &Allocation{x: []int{1}})
	assert.NoError(t, err, "No error expected when creating quota node C")

	// connect nodes: A -> ( B C )
	addedB := quotaNodeA.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeB)))
	assert.True(t, addedB, "Expecting node B added as child to node A")
	addedC := quotaNodeA.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeC)))
	assert.True(t, addedC, "Expecting node C added as child to node A")

	return quotaNodeA
}

func consumerInList(consumer *Consumer, list []*Consumer) bool {
	consumerID := consumer.GetID()
	for _, c := range list {
		if c.GetID() == consumerID {
			return true
		}
	}
	return false
}
