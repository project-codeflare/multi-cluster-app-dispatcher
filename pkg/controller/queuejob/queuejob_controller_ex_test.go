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
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejob"
	"github.com/stretchr/testify/assert"
)

// TestIsJsonSyntaxError function validates that the error is related to JSON parsing of generic items
func TestIsJsonSyntaxError(t *testing.T) {
	// Define the test table
	var tests = []struct {
		name          string
		inputErr      error
		expectedValue bool
	}{
		{"Nill error", nil, false},
		{"Job resource template item not define as a PodTemplate", errors.New("Job resource template item not define as a PodTemplate"), true},
		{"json.SyntaxError", new(json.SyntaxError), true},
		{"random errror", errors.New("some error"), false},
		{"wrapped JSON error", fmt.Errorf("wrapping syntax error, %w", new(json.SyntaxError)), true},
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
func TestCanIgnoreAPIError(t *testing.T) {

	invalidErr := apierrors.NewInvalid(schema.GroupKind{Group: arbv1.GroupName, Kind: arbv1.AppWrapperPlural}, "a test app wrapper", nil)
	notFoundErr := apierrors.NewNotFound(arbv1.Resource("appwrappers"), "a test app wrapper")
	conflictErr := apierrors.NewConflict(arbv1.Resource("appwrappers"), "a test app wrapper", errors.New("an appwrapper update conflict"))

	// Define the test table
	var tests = []struct {
		name          string
		inputErr      error
		expectedValue bool
	}{
		{"Nill error", nil, true},
		{"apierrors.IsInvalid", invalidErr, true},
		{"apierrors.IsNotFound", notFoundErr, true},
		{"apierrors.IsConflicted", conflictErr, false},
		{"generic error", errors.New("some error"), false},
		{"wrapped apierrors.IsInvalid", fmt.Errorf("wrapped invalid %w", invalidErr), true},
		{"wrapped apierrors.IsNotFound", fmt.Errorf("wrapped invalid %w", notFoundErr), true},
		{"wrapped apierrors.IsConflicted", fmt.Errorf("wrapped invalid %w", conflictErr), false},
	}
	// Execute tests in parallel
	for _, tc := range tests {
		tc := tc // Capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedValue, queuejob.CanIgnoreAPIError(tc.inputErr))
		})
	}

}
