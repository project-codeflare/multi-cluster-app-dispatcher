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
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test taking a snapshot
func TestTreeSnapshot_Take(t *testing.T) {
	type fields struct {
		targetTree              *QuotaTree
		targetConsumer          *Consumer
		allChangedConsumers     []*Consumer
		nodeStates              map[string]*NodeState
		consumerStates          map[string]*ConsumerState
		activeConsumers         map[string]*Consumer
		preemptedConsumers      []string
		preemptedConsumersArray []*Consumer
	}
	type args struct {
		controller       *Controller
		changedConsumers map[string]*Consumer
	}
	type results struct {
		targetTree       *QuotaTree
		targetConsumer   *Consumer
		nodeStates       map[string]*NodeState
		consumerStates   map[string]*ConsumerState
		changedConsumers []*Consumer
	}

	// create a test controller
	treeName := "treeA"
	ctrl := createTestContoller(t, treeName)
	allNodes := ctrl.tree.GetNodes()
	allConsumers := ctrl.GetConsumers()

	/*
		TreeController:
		QuotaTree treeA:
		|A: isHard=false; quota=[3]; allocated=[2]; consumers={ C2 }
		--|B: isHard=false; quota=[1]; allocated=[1]; consumers={ C1 }
		--|C: isHard=false; quota=[1]; allocated=[0]; consumers={ }
		Consumers:
		Consumer: ID=C1; treeID=treeA; groupID=B; priority=0; type=0; unPreemptable=false; request=[1]; aNode=B;
		Consumer: ID=C2; treeID=treeA; groupID=B; priority=0; type=0; unPreemptable=false; request=[1]; aNode=A;
	*/

	// create consumers
	consumer3 := NewConsumer("C3", treeName, "B", &Allocation{x: []int{1}}, 0, 0, false)
	consumer4 := NewConsumer("C4", treeName, "C", &Allocation{x: []int{2}}, 1, 0, false)

	// define tests
	tests := []struct {
		name    string
		fields  fields
		args    args
		results results
		want    bool
	}{
		{
			name: "consumer in group B",
			fields: fields{
				targetTree:              ctrl.tree,
				targetConsumer:          consumer3,
				allChangedConsumers:     []*Consumer{consumer3},
				nodeStates:              make(map[string]*NodeState),
				consumerStates:          make(map[string]*ConsumerState),
				activeConsumers:         map[string]*Consumer{},
				preemptedConsumers:      []string{},
				preemptedConsumersArray: []*Consumer{},
			},
			args: args{
				controller:       ctrl,
				changedConsumers: nil,
			},
			results: results{
				targetTree:     ctrl.tree,
				targetConsumer: consumer3,
				changedConsumers: []*Consumer{
					consumer3,
				},
				nodeStates: map[string]*NodeState{
					"B": {
						node:      allNodes["B"],
						allocated: &Allocation{x: []int{1}},
						consumers: []*Consumer{allConsumers["C1"]},
					},
					"A": {
						node:      allNodes["A"],
						allocated: &Allocation{x: []int{2}},
						consumers: []*Consumer{allConsumers["C2"]},
					},
				},
				consumerStates: map[string]*ConsumerState{
					"C1": {
						consumer: allConsumers["C1"],
						aNode:    allNodes["B"],
					},
					"C2": {
						consumer: allConsumers["C2"],
						aNode:    allNodes["A"],
					},
					"C3": {
						consumer: consumer3,
						aNode:    nil,
					},
				},
			},
			want: true,
		},
		{
			name: "consumer in group C",
			fields: fields{
				targetTree:              ctrl.tree,
				targetConsumer:          consumer4,
				allChangedConsumers:     []*Consumer{consumer4},
				nodeStates:              make(map[string]*NodeState),
				consumerStates:          make(map[string]*ConsumerState),
				activeConsumers:         map[string]*Consumer{},
				preemptedConsumers:      []string{},
				preemptedConsumersArray: []*Consumer{},
			},
			args: args{
				controller:       ctrl,
				changedConsumers: nil,
			},
			results: results{
				targetTree:     ctrl.tree,
				targetConsumer: consumer4,
				changedConsumers: []*Consumer{
					consumer4,
				},
				nodeStates: map[string]*NodeState{
					"C": {
						node:      allNodes["C"],
						allocated: &Allocation{x: []int{0}},
						consumers: []*Consumer{},
					},
					"A": {
						node:      allNodes["A"],
						allocated: &Allocation{x: []int{2}},
						consumers: []*Consumer{allConsumers["C2"]},
					},
				},
				consumerStates: map[string]*ConsumerState{
					"C2": {
						consumer: allConsumers["C2"],
						aNode:    allNodes["A"],
					},
					"C4": {
						consumer: consumer4,
						aNode:    nil,
					},
				},
			},
			want: true,
		},
	}

	// perform tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TreeSnapshot{
				targetTree:              tt.fields.targetTree,
				targetConsumer:          tt.fields.targetConsumer,
				allChangedConsumers:     tt.fields.allChangedConsumers,
				nodeStates:              tt.fields.nodeStates,
				consumerStates:          tt.fields.consumerStates,
				activeConsumers:         tt.fields.activeConsumers,
				preemptedConsumers:      tt.fields.preemptedConsumers,
				preemptedConsumersArray: tt.fields.preemptedConsumersArray,
			}

			// the function under test
			got := ts.Take(tt.args.controller, tt.args.changedConsumers)

			// check side effects of tested function
			assert.True(t, reflect.DeepEqual(tt.fields.targetTree, tt.results.targetTree), "target tree altered")
			assert.True(t, reflect.DeepEqual(tt.fields.targetConsumer, tt.results.targetConsumer), "target consumer altered")
			assert.True(t, reflect.DeepEqual(tt.fields.allChangedConsumers, tt.results.changedConsumers), "all changed consumers unequal")
			assert.True(t, reflect.DeepEqual(tt.fields.nodeStates, tt.results.nodeStates), "invalid node states")
			assert.True(t, reflect.DeepEqual(tt.fields.consumerStates, tt.results.consumerStates), "invalid consumer states")

			assert.True(t, reflect.DeepEqual(tt.fields.preemptedConsumers, []string{}), "preempted consumers altered")
			assert.True(t, reflect.DeepEqual(tt.fields.preemptedConsumersArray, []*Consumer{}), "preempted consumers array altered")

			if got != tt.want {
				t.Errorf("TreeSnapshot.Take() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test reinstating a snapshot
func TestTreeSnapshot_Reinstate(t *testing.T) {

	// create a test controller
	treeName := "treeA"
	ctrlBefore := createTestContoller(t, treeName)
	/*
		TreeController:
		QuotaTree treeA:
		|A: isHard=false; quota=[3]; allocated=[2]; consumers={ C2 }
		--|B: isHard=false; quota=[1]; allocated=[1]; consumers={ C1 }
		--|C: isHard=false; quota=[1]; allocated=[0]; consumers={ }
		Consumers:
		Consumer: ID=C1; treeID=treeA; groupID=B; priority=0; type=0; unPreemptable=false; request=[1]; aNode=B;
		Consumer: ID=C2; treeID=treeA; groupID=B; priority=0; type=0; unPreemptable=false; request=[1]; aNode=A;
	*/

	// create consumers
	consumer3 := NewConsumer("C3", treeName, "B", &Allocation{x: []int{1}}, 0, 0, false)
	consumer4 := NewConsumer("C4", treeName, "C", &Allocation{x: []int{2}}, 1, 0, false)

	// define tests
	tests := []struct {
		name           string
		targetConsumer *Consumer
	}{
		{
			name:           "Test 1",
			targetConsumer: consumer3,
		},
		{
			name:           "Test 2",
			targetConsumer: consumer4,
		},
	}

	// perform tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// take snapshot
			ctrlAfter := createTestContoller(t, treeName)
			ts := NewTreeSnapshot(ctrlAfter.tree, tt.targetConsumer)
			ts.Take(ctrlAfter, nil)

			// allocate a consumer
			ctrlAfter.Allocate(tt.targetConsumer)

			// the function under test
			ts.Reinstate(ctrlAfter)

			// check side effects of tested function
			assert.True(t, EqualStateControllers(ctrlBefore, ctrlAfter),
				"invalid reinstated state, got: %v, expected: %v", ctrlAfter, ctrlBefore)
		})
	}
}
