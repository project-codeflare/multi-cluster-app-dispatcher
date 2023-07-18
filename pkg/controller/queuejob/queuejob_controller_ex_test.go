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

package queuejob_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejob"
	"github.com/stretchr/testify/assert"
)

// TestNewQuotaManagerConsumerAllocationRelease function emulates multiple threads adding quota consumers and removing them
func TestIsJsonSyntaxError(t *testing.T) {
	// Define the test table
	var tests = []struct {
		name          string
		inputErr      error
		expectedValue bool
	}{
		{"Nill error", nil, false},
		{"Job resource template item not define as a PodTemplate", errors.New("Job resource template item not define as a PodTemplate"), true},
		{"unexpected end of JSON input", errors.New("unexpected end of JSON input"), true},
		{"json.SyntaxError", new(json.SyntaxError), true},
	}
	// Execute tests in parallel
	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedValue, queuejob.IsJsonSyntaxError(tc.inputErr))
		})
	}
}
