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
	"testing"

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
	qmManagerUnderTest.SetMode(quota.Normal)

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
			assert.NoError(t, err, "No Error expected when adding consumer to forest")
			assert.NotNil(t, response, "A non nill response is expected")

			deallocated := qmManagerUnderTest.DeAllocateForest(forestName, consumerInfo.GetID())
			assert.True(t, deallocated, "Deallocation expected to succeed")
		})
	}
}
