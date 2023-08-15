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

package quota_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	"github.com/stretchr/testify/assert"
)

var (
	testTreeSpec string = `{
		"kind": "QuotaTree",
		"metadata": {
		  "name": "test-tree"
		},
		"spec": {
		  "resourceNames": [
			"cpu",
			"memory"
		  ],
		  "nodes": {		
			"A": {
			  "parent": "nil",
			  "quota": {
				"cpu": "12",
				"memory": "256"
			  }
			},
			"B": {
			  "parent": "A",
			  "quota": {
				"cpu": "4",
				"memory": "64"
			  }
			},
			"C": {
			  "parent": "A",
			  "quota": {
				"cpu": "4",
				"memory": "64"
			  }
			}
		  }
		}
	}`

	testConsumersSpecMap = map[string]string{
		"J-1": `{
			"kind": "Consumer",
			"metadata": {
			  "name": "J-1"
			},
			"spec": {
			  "id": "J-1",
			  "trees": [
				 {
				   "treeName": "test-tree",
				  "groupID": "B",
				  "request": {
					"cpu": 4,
					"memory": 32
				  }
				}
			  ]
			}
		  }`,
		"J-2": `{
			"kind": "Consumer",
			"metadata": {
			  "name": "J-2"
			},
			"spec": {
			  "id": "J-2",
			  "trees": [
				 {
				   "treeName": "test-tree",
				  "groupID": "C",
				  "request": {
					"cpu": 4,
					"memory": 32
				  }
				}
			  ]
			}
		  }`,
		"J-3": `{
			"kind": "Consumer",
			"metadata": {
			  "name": "J-3"
			},
			"spec": {
			  "id": "J-3",
			  "trees": [
				 {
				   "treeName": "test-tree",
				  "groupID": "C",
				  "request": {
					"cpu": 4,
					"memory": 32
				  }
				}
			  ]
			}
		  }`,
	}

	serviceTreeSpec string = `{
		"kind": "QuotaTree",
		"metadata": {
		  "name": "service-tree"
		},
		"spec": {
		  "resourceNames": [
			"count"
		  ],
		  "nodes": {
			"root": {
				"parent": "nil",
				"quota": {
				  "count": "12"
				}
			  },
			"X": {
			  "parent": "root",
			  "quota": {
				"count": "5"
			  }
			},
			"Y": {
			  "parent": "root",
			  "quota": {
				"count": "5"
			  }
			},
			"Z": {
				"parent": "Y",
				"quota": {
				  "count": "6"
				}
			  }
		  }
		}
	}`

	testForestConsumersSpecMap = map[string]string{
		"F-1": `{
			"kind": "Consumer",
			"metadata": {
			  "name": "F-1"
			},
			"spec": {
			  "id": "F-1",
			  "trees": [
				 {
				  "treeName": "test-tree",
				  "groupID": "B",
				  "request": {
					"cpu": 4,
					"memory": 32
				  }
				},
				{
				   "treeName": "service-tree",
				   "groupID": "X",
				   "request": {
					 "count": 3
				   }
				 }
			  ]
			}
		  }`,
	}
)

// TestTreeAllocateTryAndUndo : test TryAllocate() and UndoAllocate() on a tree
func TestTreeAllocateTryAndUndo(t *testing.T) {
	// define tests
	var tests = []struct {
		name         string
		consumerSpec string
		wantErr      bool
	}{
		{
			name: "low priority consumer",
			consumerSpec: `{
				"kind": "Consumer",
				"metadata": {
				  "name": "J-4"
				},
				"spec": {
				  "id": "J-4",
				  "trees": [
					 {
					   "treeName": "test-tree",
					  "groupID": "C",
					  "request": {
						"cpu": 2,
						"memory": 16
					  }
					}
				  ]
				}
			  }`,
			wantErr: false,
		},
		{
			name: "high priority consumer",
			consumerSpec: `{
				  "kind": "Consumer",
				  "metadata": {
					"name": "J-5"
				  },
				  "spec": {
					"id": "J-5",
					"trees": [
					   {
						 "treeName": "test-tree",
						"groupID": "C",
						"request": {
						  "cpu": 4,
						  "memory": 32
						},
						"priority": 1
					  }
					]
				  }
				}`,
			wantErr: false,
		},
		{
			name: "invalid consumer",
			consumerSpec: `{
				"kind": "Consumer",
				"metadata": {
				  "name": "J-6"
				},
				"spec": {
				  "id": "J-6",
				  "trees": [
					 {
					   "treeName": "other-tree",
					  "groupID": "X",
					  "request": {
						"cpu": 2,
						"memory": 16
					  }
					}
				  ]
				}
			  }`,
			wantErr: true,
		},
	}

	consumersToplace := []string{"J-1", "J-2", "J-3"}

	// Before keeps the state of the quota manager before applying test cases
	qmBefore := quota.NewManager()
	treeName := initializeQuotaManager(t, qmBefore, testTreeSpec, testConsumersSpecMap, consumersToplace)
	ctlrBefore := qmBefore.GetTreeController(treeName)

	// perform tests
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// After keeps the state of the quota manager after applying a test case
			qmAfter := quota.NewManager()
			initializeQuotaManager(t, qmAfter, testTreeSpec, testConsumersSpecMap, consumersToplace)
			ctlrAfter := qmAfter.GetTreeController(treeName)
			assert.True(t, core.EqualStateControllers(ctlrBefore, ctlrAfter),
				"Initial tree allocation states should be same; got: %v; want: %v",
				ctlrAfter, ctlrBefore)

			// try allocate test consumer
			consumerID, err := createConsumer(qmAfter, tt.consumerSpec)
			if err != nil && !tt.wantErr {
				t.Errorf("Error creating consumer %s", consumerID)
			}
			qmAfter.TryAllocate(treeName, consumerID)
			if qmAfter.IsAllocated(treeName, consumerID) {
				assert.False(t, core.EqualStateControllers(ctlrBefore, ctlrAfter),
					"Tree allocation states should be different after test consumer %s allocation; got: %v; notWant: %v",
					consumerID, ctlrAfter, ctlrBefore)
			} else {
				assert.True(t, core.EqualStateControllers(ctlrBefore, ctlrAfter),
					"Tree allocation states should be same if consumer %s is not allocated; got: %v; want: %v",
					consumerID, ctlrAfter, ctlrBefore)
			}

			// simulate computation delay between try allocating and undoing the allocation
			time.Sleep(10 * time.Millisecond)

			// undo allocate of test consumer
			err = qmAfter.UndoAllocate(treeName, consumerID)
			if err != nil && !tt.wantErr {
				t.Errorf("Error in undo allocate consumer %s", consumerID)
			}
			assert.True(t, core.EqualStateControllers(ctlrBefore, ctlrAfter),
				"Unequal tree allocation state after undo allocation %s; got: %v; want: %v",
				consumerID, ctlrAfter, ctlrBefore)
		})
	}
}

// TestForestAllocateTryAndUndo : test TryAllocate() and UndoAllocate() on a forest
func TestForestAllocateTryAndUndo(t *testing.T) {
	forestName := "testForest"
	forestTreesSpecs := []string{testTreeSpec, serviceTreeSpec}

	// define tests
	var tests = []struct {
		name         string
		consumerSpec string
		wantErr      bool
	}{
		{
			name: "valid consumer",
			consumerSpec: `{
				"kind": "Consumer",
				"metadata": {
				  "name": "F-5"
				},
				"spec": {
				  "id": "F-5",
				  "trees": [
					 {
					  "treeName": "test-tree",
					  "groupID": "B",
					  "request": {
						"cpu": 4,
						"memory": 32
					  }
					},
					{
					   "treeName": "service-tree",
					   "groupID": "X",
					   "request": {
						 "count": 3
					   }
					 }
				  ]
				}
			  }`,
			wantErr: false,
		},
		{
			name: "invalid consumer",
			consumerSpec: `{
				"kind": "Consumer",
				"metadata": {
				  "name": "F-6"
				},
				"spec": {
				  "id": "F-6",
				  "trees": [
					 {
					  "treeName": "test-tree",
					  "groupID": "F",
					  "request": {
						"cpu": 2,
						"memory": 16
					  }
					},
					{
					   "treeName": "unknown-tree",
					   "groupID": "X",
					   "request": {
						 "count": 3
					   }
					 }
				  ]
				}
			  }`,
			wantErr: true,
		},
	}

	consumersToplace := []string{"F-1"}

	// Before keeps the state of the quota manager before applying test cases
	qmBefore := quota.NewManager()
	initializeForestQuotaManager(t, qmBefore, forestName, forestTreesSpecs, testForestConsumersSpecMap,
		consumersToplace)
	ctlrBefore := qmBefore.GetForestController(forestName)

	// perform tests
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var err error
			// After keeps the state of the quota manager after applying a test case
			qmAfter := quota.NewManager()
			initializeForestQuotaManager(t, qmAfter, forestName, forestTreesSpecs, testForestConsumersSpecMap,
				consumersToplace)
			ctlrAfter := qmAfter.GetForestController(forestName)

			// try allocate test consumer
			var cinfo *quota.ConsumerInfo
			cinfo, err = quota.NewConsumerInfoFromString(tt.consumerSpec)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Error creating consumer from spec")
				}
				return
			}
			consumerID := cinfo.GetID()
			var added bool
			added, err = qmAfter.AddConsumer(cinfo)
			assert.True(t, added && err == nil, "Error adding consumer %s", consumerID)
			_, err = qmAfter.TryAllocateForest(forestName, consumerID)
			assert.True(t, err == nil || tt.wantErr,
				"Error allocating consumer %s on forest %s", consumerID, forestName)
			if qmAfter.IsAllocatedForest(forestName, consumerID) {
				assert.False(t, core.EqualStateForestControllers(ctlrBefore, ctlrAfter),
					"Forest allocation states should be different after test consumer %s allocation", consumerID)
			} else {
				assert.True(t, core.EqualStateForestControllers(ctlrBefore, ctlrAfter),
					"Forest allocation states should be same if consumer %s is not allocated, want %v, got %v",
					consumerID, ctlrBefore, ctlrAfter)
			}

			// simulate computation delay between try allocating and undoing the allocation
			time.Sleep(10 * time.Millisecond)

			// undo allocate of test consumer
			err = qmAfter.UndoAllocateForest(forestName, consumerID)
			if err != nil {
				t.Errorf("Error in undo forest allocate consumer %s", consumerID)
			}
			assert.True(t, core.EqualStateForestControllers(ctlrBefore, ctlrAfter),
				"Unequal forest allocation state after undo allocation %s", consumerID)
		})
	}
}

// initializeQuotaManager : helper to create an initial test quota tree and allocate a set of consumers
func initializeQuotaManager(t *testing.T, qm *quota.Manager, treeSpec string, testConsumersSpecMap map[string]string,
	consumersToplace []string) string {
	treeName, err := qm.AddTreeFromString(treeSpec)
	assert.NoError(t, err, "Error creating tree from spec %s", treeName)
	qm.SetMode(quota.Normal)

	// place some consumers
	for _, consumerID := range consumersToplace {
		cSpec := testConsumersSpecMap[consumerID]
		assert.True(t, len(cSpec) != 0, "Error creating consumer %s, not in testForestConsumersSpecMap", consumerID)
		_, err := createConsumer(qm, cSpec)
		assert.NoError(t, err, "Error creating consumer from spec %s", treeName)
		_, err = qm.Allocate(treeName, consumerID)
		assert.NoError(t, err,
			"Error allocating consumer %s on tree %s", consumerID, treeName)
	}
	return treeName
}

// initializeForestQuotaManager : helper to create an initial test quota forest and allocate a set of forest consumers
func initializeForestQuotaManager(t *testing.T, qm *quota.Manager, forestName string, treeSpecs []string,
	testConsumersSpecMap map[string]string, consumersToplace []string) {

	// add forest
	qm.AddForest(forestName)
	for _, treeSpec := range treeSpecs {
		treeName, err := qm.AddTreeFromString(treeSpec)
		assert.NoError(t, err, "Error creating tree from spec %s", treeName)
		qm.AddTreeToForest(forestName, treeName)
	}
	qm.SetMode(quota.Normal)

	// create some consumers
	for _, cID := range consumersToplace {
		cSpec := testForestConsumersSpecMap[cID]
		assert.True(t, len(cSpec) != 0, "Error creating consumer %s, not in testForestConsumersSpecMap", cID)
		cinfo, err := quota.NewConsumerInfoFromString(cSpec)
		assert.NoError(t, err, "Error creating consumer from spec %s", cID)

		consumerID := cinfo.GetID()
		added, err := qm.AddConsumer(cinfo)
		assert.True(t, added && err == nil, "Error adding consumer %s", consumerID)

		_, err = qm.AllocateForest(forestName, consumerID)
		assert.NoError(t, err,
			"Error allocating consumer %s on forest %s", consumerID, forestName)
	}
}

// createConsumer : create a consumer from specification and add to quota manager
func createConsumer(qm *quota.Manager, consumerSpec string) (string, error) {
	cinfo, err := quota.NewConsumerInfoFromString(consumerSpec)
	if err != nil {
		return "", err
	}
	consumerID := cinfo.GetID()
	if added, err := qm.AddConsumer(cinfo); !added || err != nil {
		return consumerID, err
	}
	return consumerID, nil
}

func TestQuotaManagerParallelTryUndoAllocation(t *testing.T) {
	forestName := "unit-test-1"
	qmManagerUnderTest := quota.NewManager()

	err := qmManagerUnderTest.AddForest(forestName)
	assert.NoError(t, err, "No error expected when adding a forest")
	testTreeName, err := qmManagerUnderTest.AddTreeFromString(
		`{
			"kind": "QuotaTree",
			"metadata": {
			  "name": "test-tree"
			},
			"spec": {
			  "resourceNames": [
				"cpu",
				"memory"
			  ],
			  "nodes": {		
				"root": {
				  "parent": "nil",
				  "hard": "true",
				  "quota": {
					"cpu": "10",
					"memory": "256"
				  }
				},		
				"gold": {
				  "parent": "root",
				  "hard": "true",
				  "quota": {
					"cpu": "10",
					"memory": "256"
				  }
				}
			  }
			}
		}`)
	if err != nil {
		assert.Fail(t, "No error expected when adding a tree to forest")
	}
	err = qmManagerUnderTest.AddTreeToForest(forestName, testTreeName)
	assert.NoError(t, err, "No error expected when adding a tree from forest")
	modeSet := qmManagerUnderTest.SetMode(quota.Normal)
	assert.True(t, modeSet, "Setting the mode should not fail.")

	tryUndoLock := sync.RWMutex{}

	// Define the test table
	var tests = []struct {
		name     string
		consumer utils.JConsumer
	}{
		// the table itself
		{"Gold consumer 1",
			utils.JConsumer{
				Kind: "Consumer",
				MetaData: utils.JMetaData{
					Name: "gold-consumer-data-1",
				},
				Spec: utils.JConsumerSpec{
					ID: "gold-consumer-1",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "gold",
							Request: map[string]int{
								"cpu":    4,
								"memory": 4,
								"gpu":    0,
							},
							Priority:      0,
							CType:         0,
							UnPreemptable: false,
						},
					},
				},
			},
		},
		// the table itself
		{"Gold consumer 2",
			utils.JConsumer{
				Kind: "Consumer",
				MetaData: utils.JMetaData{
					Name: "gold-consumer-data-2",
				},
				Spec: utils.JConsumerSpec{
					ID: "gold-consumer-2",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "gold",
							Request: map[string]int{
								"cpu":    4,
								"memory": 4,
								"gpu":    0,
							},
							Priority:      0,
							CType:         0,
							UnPreemptable: false,
						},
					},
				},
			},
		},
	}
	// Execute tests in parallel
	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Get list of quota management tree IDs
			qmTreeIDs := qmManagerUnderTest.GetTreeNames()

			consumerInfo, err := quota.NewConsumerInfo(tc.consumer)
			assert.NoError(t, err, "No error expected when building consumer")
			assert.Contains(t, qmTreeIDs, tc.consumer.Spec.Trees[0].TreeName)

			added, err := qmManagerUnderTest.AddConsumer(consumerInfo)
			assert.NoError(t, err, "No error expected when adding consumer")
			assert.True(t, added, "Consumer is expected to be added")

			// TryAllocate() and UndoAllocate() should be atomic
			tryUndoLock.Lock()
			defer tryUndoLock.Unlock()

			response, err := qmManagerUnderTest.TryAllocate(tc.consumer.Spec.Trees[0].TreeName, consumerInfo.GetID())
			if !assert.NoError(t, err, "No error expected when calling TryAllocate consumer") {
				assert.Equal(t, 0, len(strings.TrimSpace(response.GetMessage())), "A empty response is expected")
				assert.True(t, response.IsAllocated(), "The allocation should succeed")
			}

			//simulate a long running consumer that has quota allocated
			time.Sleep(10 * time.Millisecond)

			err = qmManagerUnderTest.UndoAllocate(tc.consumer.Spec.Trees[0].TreeName, consumerInfo.GetID())
			assert.NoError(t, err, "No error expected when calling UndoAllocate consumer")
		})
	}
}
