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
	"reflect"
	"strings"
	"testing"
	"unsafe"

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
	"github.com/stretchr/testify/assert"
)

// TestControllerTryUndo : test try allocate (take snapshot) and undo allocate (reinstate)
func TestControllerTryUndo(t *testing.T) {
	treeName := "treeA"

	// define tests
	var tests = []struct {
		name          string
		consumer      *Consumer
		wantAllocated bool
		wantErr       bool
	}{
		{
			name: "invalid consumer wrong group",
			consumer: &Consumer{
				id:      "C",
				treeID:  treeName,
				groupID: "X",
				request: &Allocation{x: []int{1}},
			},
			wantAllocated: false,
			wantErr:       true,
		},
		{
			name: "insufficient quota",
			consumer: &Consumer{
				id:      "C",
				treeID:  treeName,
				groupID: "C",
				request: &Allocation{x: []int{5}},
			},
			wantAllocated: false,
			wantErr:       true,
		},
		{
			name: "consumer no preemption",
			consumer: &Consumer{
				id:      "C",
				treeID:  treeName,
				groupID: "C",
				request: &Allocation{x: []int{1}},
			},
			wantAllocated: true,
			wantErr:       false,
		},
		{
			name: "consumer preemption",
			consumer: &Consumer{
				id:      "C",
				treeID:  treeName,
				groupID: "B",
				request: &Allocation{x: []int{1}},
			},
			wantAllocated: true,
			wantErr:       false,
		},
		{
			name: "high priority consumer no preemption",
			consumer: &Consumer{
				id:       "C",
				treeID:   treeName,
				groupID:  "C",
				request:  &Allocation{x: []int{1}},
				priority: 1,
			},
			wantAllocated: true,
			wantErr:       false,
		},
		{
			name: "high priority consumer preemption",
			consumer: &Consumer{
				id:       "C",
				treeID:   treeName,
				groupID:  "B",
				request:  &Allocation{x: []int{2}},
				priority: 1,
			},
			wantAllocated: true,
			wantErr:       false,
		},
	}

	// Before keeps the state of the quota tree before applying test cases
	ctlrBefore := createTestContoller(t, treeName)

	// perform tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// After keeps the state of the quota tree after applying a test case
			ctlrAfter := createTestContoller(t, tt.consumer.treeID)

			response := ctlrAfter.TryAllocate(tt.consumer)
			if err := response == nil || len(strings.TrimSpace(response.GetMessage())) > 0; err != tt.wantErr {
				t.Errorf("TryAllocate() response error, want %v", tt.wantErr)
			}
			if treeChanged := !EqualStateControllers(ctlrBefore, ctlrAfter); treeChanged != tt.wantAllocated {
				t.Errorf("TryAllocate() tree changed = %v, want %v", treeChanged, tt.wantAllocated)
			}
			if undone := ctlrAfter.UndoAllocate(tt.consumer); !undone {
				t.Errorf("TryAllocate() undo failed, want %v", tt.wantErr)
			}
			if !EqualStateControllers(ctlrAfter, ctlrBefore) {
				t.Errorf("UndoAllocate() = %v, want %v", ctlrAfter, ctlrBefore)
			}
		})
	}
}

// createTestContoller : create an initial test quota tree with allocated consumers
func createTestContoller(t *testing.T, treeName string) *Controller {

	// create three quota nodes A[3], B[1], and C[1]
	quotaNodeA, err := NewQuotaNode("A", &Allocation{x: []int{3}})
	assert.NoError(t, err, "No error expected when creating quota node A")

	quotaNodeB, err := NewQuotaNode("B", &Allocation{x: []int{1}})
	assert.NoError(t, err, "No error expected when creating quota node B")

	quotaNodeC, err := NewQuotaNode("C", &Allocation{x: []int{1}})
	assert.NoError(t, err, "No error expected when creating quota node C")

	// create two consumers C1[1] and C2[1]
	consumer1 := NewConsumer("C1", treeName, "B", &Allocation{x: []int{1}}, 0, 0, false)
	consumer2 := NewConsumer("C2", treeName, "B", &Allocation{x: []int{1}}, 0, 0, false)

	// create quota tree: A -> ( B C )
	quotaNodeA.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeB)))
	quotaNodeA.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeC)))
	quotaTreeA := NewQuotaTree(treeName, quotaNodeA, []string{"count"})
	controller := NewController(quotaTreeA)

	// allocate consumers C1 and C2
	response1 := controller.Allocate(consumer1)
	assert.NotNil(t, response1, "A non nill response 1 is expected")
	assert.Equal(t, 0, len(strings.TrimSpace(response1.GetMessage())), "A empty response 1 is expected")

	response2 := controller.Allocate(consumer2)
	assert.NotNil(t, response2, "A non nill response 2 is expected")
	assert.Equal(t, 0, len(strings.TrimSpace(response2.GetMessage())), "A empty response 2 is expected")

	return controller
}

// EqualStateQuotaNodes : check if two quota nodes have similar allocation data
func EqualStateQuotaNodes(qn1 *QuotaNode, qn2 *QuotaNode) bool {
	return reflect.DeepEqual(qn1.GetQuota(), qn2.GetQuota()) &&
		reflect.DeepEqual(qn1.GetAllocated(), qn2.GetAllocated()) &&
		reflect.DeepEqual(qn1.GetConsumers(), qn2.GetConsumers())
}

// EqualStateQuotaTrees : check if two quota trees have similar allocation data
func EqualStateQuotaTrees(qt1 *QuotaTree, qt2 *QuotaTree) bool {
	nodeMap1 := qt1.GetNodes()
	nodeMap2 := qt2.GetNodes()
	if len(nodeMap1) != len(nodeMap2) {
		return false
	}
	for k, qn1 := range nodeMap1 {
		if qn2, exists := nodeMap2[k]; exists {
			if !EqualStateQuotaNodes(qn1, qn2) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// EqualStateControllers : check if two controllers have similar allocation state
func EqualStateControllers(c1 *Controller, c2 *Controller) bool {
	return EqualStateQuotaTrees(c1.tree, c2.tree) &&
		reflect.DeepEqual(c1.consumers, c1.consumers) &&
		reflect.DeepEqual(c1.preemptedConsumers, c2.preemptedConsumers) &&
		reflect.DeepEqual(c1.preemptedConsumersArray, c2.preemptedConsumersArray)
}
