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
