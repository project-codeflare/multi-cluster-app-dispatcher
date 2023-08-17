/*
Copyright 2022, 2203 The Multi-Cluster App Dispatcher Authors.

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
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/eapache/go-resiliency/retrier"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	"github.com/stretchr/testify/assert"
)

// TestNewQuotaManagerConsumerAllocationRelease function emulates multiple threads adding quota consumers and removing them
func TestNewQuotaManagerConsumerAllocationRelease(t *testing.T) {
	forestName := "unit-test-2"
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
					"cpu": "2",
					"memory": "64"
				  }
				},
		
				"silver": {
				  "parent": "root",
				  "quota": {
					"cpu": "6",
					"memory": "64"
				  }
				},
		
				"bronze": {
				  "parent": "root",
				  "hard": "false",
				  "quota": {
					"cpu": "2",
					"memory": "128"
				  }
				}
		
			  }
			}
		}`)
	assert.NoError(t, err, "No error expected when adding a tree to forest")
	err = qmManagerUnderTest.AddTreeToForest(forestName, testTreeName)
	assert.NoError(t, err, "No error expected when adding a tree from forest")
	modeSet := qmManagerUnderTest.SetMode(quota.Normal)
	assert.True(t, modeSet, "Setting the mode should not fail.")

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
					Name: "gpld-consumer-data",
				},
				Spec: utils.JConsumerSpec{
					ID: "gold-consumer-1",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "gold",
							Request: map[string]int{
								"cpu":    1,
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
		{"Gold consumer 2",
			utils.JConsumer{
				Kind: "Consumer",
				MetaData: utils.JMetaData{
					Name: "gpld-consumer-data",
				},
				Spec: utils.JConsumerSpec{
					ID: "gold-consumer-2",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "gold",
							Request: map[string]int{
								"cpu":    1,
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
		{"Bronze consumer",
			utils.JConsumer{
				Kind: "Consumer",
				MetaData: utils.JMetaData{
					Name: "bronze-consumer-data",
				},
				Spec: utils.JConsumerSpec{
					ID: "bronze-consumer-1",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "bronze",
							Request: map[string]int{
								"cpu":    1,
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
		{"Silver consumer",
			utils.JConsumer{
				Kind: "Consumer",
				MetaData: utils.JMetaData{
					Name: "bronze-consumer-data",
				},
				Spec: utils.JConsumerSpec{
					ID: "silver-consumer-1",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "silver",
							Request: map[string]int{
								"cpu":    1,
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

			response, err := qmManagerUnderTest.AllocateForest(forestName, consumerInfo.GetID())
			assert.NoError(t, err, "No Error expected when allocating consumer to forest")
			assert.NotNil(t, response, "A non nill response is expected")
			assert.Equal(t, 0, len(strings.TrimSpace(response.GetMessage())), "A empty response is expected")

			deAllocated := qmManagerUnderTest.DeAllocateForest(forestName, consumerInfo.GetID())
			assert.True(t, deAllocated, "De-allocation expected to succeed")

			removed, err := qmManagerUnderTest.RemoveConsumer(consumerInfo.GetID())
			assert.NoError(t, err, "No Error expected when removing consumer")
			assert.True(t, removed, "Removal of consumer should succeed")
		})
	}
}
func TestQuotaManagerQuotaUsedLongRunningConsumers(t *testing.T) {
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
					Name: "gpld-consumer-data",
				},
				Spec: utils.JConsumerSpec{
					ID: "gold-consumer-1",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "gold",
							Request: map[string]int{
								"cpu":    10,
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
					Name: "gpld-consumer-data",
				},
				Spec: utils.JConsumerSpec{
					ID: "gold-consumer-2",
					Trees: []utils.JConsumerTreeSpec{
						{
							TreeName: testTreeName,
							GroupID:  "gold",
							Request: map[string]int{
								"cpu":    10,
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

			retryAllocation := retrier.New(retrier.LimitedExponentialBackoff(10, 10*time.Millisecond, 1*time.Second),
				&AllocationClassifier{})

			err = retryAllocation.Run(func() error {
				added, err2 := qmManagerUnderTest.AddConsumer(consumerInfo)
				assert.NoError(t, err2, "No error expected when adding consumer")
				assert.True(t, added, "Consumer is expected to be added")

				response, err2 := qmManagerUnderTest.AllocateForest(forestName, consumerInfo.GetID())
				if err2 == nil {
					assert.Equal(t, 0, len(strings.TrimSpace(response.GetMessage())), "A empty response is expected")
					assert.True(t, response.IsAllocated(), "The allocation should succeed")
				} else {
					removed, err3 := qmManagerUnderTest.RemoveConsumer(consumerInfo.GetID())
					assert.NoError(t, err3, "No Error expected when removing consumer")
					assert.True(t, removed, "Removal of consumer should succeed")
				}
				return err2

			})
			if err != nil {
				assert.Failf(t, "Allocation of quota should have succeed: '%s'", err.Error())
			}

			//simulate a long running consumer that has quota allocated
			time.Sleep(10 * time.Millisecond)

			deAllocated := qmManagerUnderTest.DeAllocateForest(forestName, consumerInfo.GetID())
			assert.True(t, deAllocated, "De-allocation expected to succeed")

			removed, err := qmManagerUnderTest.RemoveConsumer(consumerInfo.GetID())
			assert.NoError(t, err, "No Error expected when removing consumer")
			assert.True(t, removed, "Removal of consumer should succeed")

		})
	}

}

var (
	tree1String string = `{
	"kind": "QuotaTree",
	"metadata": {
	  "name": "tree-1"
	},
	"spec": {
	  "resourceNames": [
		"cpu",
		"memory"
	  ],
	  "nodes": {
		"A": {
		  "parent": "nil",
		  "hard": "true",
		  "quota": {
			"cpu": "10",
			"memory": "256"
		  }
		},
		"B": {
		  "parent": "A",
		  "hard": "true",
		  "quota": {
			"cpu": "2",
			"memory": "64"
		  }
		},
		"C": {
		  "parent": "A",
		  "quota": {
			"cpu": "6",
			"memory": "64"
		  }
		}
	  }
	}
}`

	tree2String string = `{
	"kind": "QuotaTree",
	"metadata": {
	  "name": "tree-2"
	},
	"spec": {
	  "resourceNames": [
		"gpu"
	  ],
	  "nodes": {
		"X": {
		  "parent": "nil",
		  "hard": "true",
		  "quota": {
			"gpu": "32"
		  }
		},
		"Y": {
		  "parent": "X",
		  "hard": "true",
		  "quota": {
			"gpu": "16"
		  }
		},
		"Z": {
		  "parent": "X",
		  "quota": {
			"gpu": "8"
		  }
		}
	  }
	}
}`

	tree3String string = `{
	"kind": "QuotaTree",
	"metadata": {
	  "name": "tree-3"
	},
	"spec": {
	  "resourceNames": [
		"count"
	  ],
	  "nodes": {
		"zone": {
		  "parent": "nil",
		  "hard": "true",
		  "quota": {
			"count": "100"
		  }
		},
		"rack": {
		  "parent": "zone",
		  "hard": "true",
		  "quota": {
			"count": "100"
		  }
		},
		"server": {
		  "parent": "rack",
		  "quota": {
			"count": "100"
		  }
		}
	  }
	}
}`
)

// TestAddDeleteTrees : test adding, retrieving, deleting trees
func TestAddDeleteTrees(t *testing.T) {
	qmTest := quota.NewManager()
	assert.NotNil(t, qmTest, "Expecting no error creating a quota manager")
	modeSet := qmTest.SetMode(quota.Normal)
	assert.True(t, modeSet, "Expecting no error setting mode to normal")
	mode := qmTest.GetMode()
	assert.True(t, mode == quota.Normal, "Expecting mode set to normal")
	modeString := qmTest.GetModeString()
	match := strings.Contains(modeString, "Normal")
	assert.True(t, match, "Expecting mode set to normal")
	// add a few trees by name
	treeNames := []string{"tree-1", "tree-2", "tree-3"}
	treeStrings := []string{tree1String, tree2String, tree3String}
	for i, treeString := range treeStrings {
		name, err := qmTest.AddTreeFromString(treeString)
		assert.NoError(t, err, "No error expected when adding a tree")
		assert.Equal(t, treeNames[i], name, "Returned name should match")
	}
	// get list of names
	names := qmTest.GetTreeNames()
	assert.ElementsMatch(t, treeNames, names, "Expecting retrieved tree names same as added names")
	// delete a name
	deletedTreeName := treeNames[0]
	remainingTreeNames := treeNames[1:]
	err := qmTest.DeleteTree(deletedTreeName)
	assert.NoError(t, err, "No error expected when deleting an existing tree")
	// get list of names after deletion
	names = qmTest.GetTreeNames()
	assert.ElementsMatch(t, remainingTreeNames, names, "Expecting retrieved tree names to reflect additions/deletions")
	// delete a non-existing name
	err = qmTest.DeleteTree(deletedTreeName)
	assert.Error(t, err, "Error expected when deleting a non-existing tree")
}

// TestAddDeleteForests : test adding, retrieving, deleting forests
func TestAddDeleteForests(t *testing.T) {
	var err error
	qmTest := quota.NewManager()
	assert.NotNil(t, qmTest, "Expecting no error creating a quota manager")
	modeSet := qmTest.SetMode(quota.Normal)
	assert.True(t, modeSet, "Expecting no error setting mode to normal")

	// add a few trees by name
	treeNames := []string{"tree-1", "tree-2", "tree-3"}
	treeStrings := []string{tree1String, tree2String, tree3String}
	for i, treeString := range treeStrings {
		name, err := qmTest.AddTreeFromString(treeString)
		assert.NoError(t, err, "No error expected when adding a tree")
		assert.Equal(t, treeNames[i], name, "Returned name should match")
	}
	// create two forests
	forestNames := []string{"forest-1", "forest-2"}
	for _, forestName := range forestNames {
		err = qmTest.AddForest(forestName)
		assert.NoError(t, err, "No error expected when adding a forest")
	}
	// assign trees to forests
	err = qmTest.AddTreeToForest("forest-1", "tree-1")
	assert.NoError(t, err, "No error expected when adding a tree to a forest")
	err = qmTest.AddTreeToForest("forest-2", "tree-2")
	assert.NoError(t, err, "No error expected when adding a tree to a forest")
	err = qmTest.AddTreeToForest("forest-2", "tree-3")
	assert.NoError(t, err, "No error expected when adding a tree to a forest")
	// get list of forest names
	fNames := qmTest.GetForestNames()
	assert.ElementsMatch(t, forestNames, fNames, "Expecting retrieved forest names same as added names")
	// get forests map
	forestTreeMap := qmTest.GetForestTreeNames()
	for _, v := range forestTreeMap {
		sort.Strings(v)
	}
	inputForestTreeMap := map[string][]string{"forest-1": {"tree-1"}, "forest-2": {"tree-2", "tree-3"}}
	assert.True(t, reflect.DeepEqual(forestTreeMap, inputForestTreeMap),
		"Expecting retrieved forest tree map same as input, got %v, want %v", forestTreeMap, inputForestTreeMap)
	// delete a forest
	deletedForestName := forestNames[0]
	remainingForestNames := forestNames[1:]
	err = qmTest.DeleteForest(deletedForestName)
	assert.NoError(t, err, "No error expected when deleting an existing forest")
	// get list of forest names after deletion
	fNames = qmTest.GetForestNames()
	assert.ElementsMatch(t, remainingForestNames, fNames, "Expecting retrieved forest names to reflect additions/deletions")
	// delete a non-existing forest name
	err = qmTest.DeleteForest(deletedForestName)
	assert.Error(t, err, "Error expected when deleting a non-existing forest")
	// delete a tree from a forest
	err = qmTest.DeleteTreeFromForest("forest-2", "tree-2")
	assert.NoError(t, err, "No error expected when deleting an existing tree from an existing forest")
	err = qmTest.DeleteTreeFromForest("forest-2", "tree-2")
	assert.Error(t, err, "Error expected when deleting an non-existing tree from an existing forest")
	// check remaining trees after deletions
	names := qmTest.GetTreeNames()
	assert.True(t, reflect.DeepEqual(treeNames, names),
		"Expecting all trees after forest deletions as trees are not deleted, got %v, want %v", names, treeNames)
}

type AllocationClassifier struct {
}

func (c *AllocationClassifier) Classify(err error) retrier.Action {
	if err == nil {
		return retrier.Succeed
	}
	if strings.TrimSpace(err.Error()) == "Failed to allocate quota on quota designation 'test-tree'" {
		return retrier.Retry
	}
	return retrier.Fail
}
