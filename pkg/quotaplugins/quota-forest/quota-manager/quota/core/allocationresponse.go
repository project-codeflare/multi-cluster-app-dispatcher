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

// AllocationResponse : A response object to a consumer allocation request
type AllocationResponse struct {
	// ID of subject consumer
	consumerID string
	// result of allocation request
	allocated bool
	// any message in relation to the allocation request
	message string
	// resulting set of (unique) IDs of preempted consumers due to the allocation request
	preemptedIDs map[string]bool
}

// NewAllocationResponse : create an allocation response for a consumer
func NewAllocationResponse(consumerID string) *AllocationResponse {
	return &AllocationResponse{
		consumerID:   consumerID,
		allocated:    true,
		message:      "",
		preemptedIDs: make(map[string]bool),
	}
}

// Append : append to the response
func (ar *AllocationResponse) Append(allocated bool, message string, preemptedIds *[]string) {
	ar.allocated = ar.allocated && allocated
	ar.message += " " + message
	for _, id := range *preemptedIds {
		ar.preemptedIDs[id] = true
	}
}

// Merge : merge another response into this
func (ar *AllocationResponse) Merge(other *AllocationResponse) {
	if ar.consumerID != other.GetConsumerID() {
		return
	}
	pids := other.GetPreemptedIds()
	ar.Append(
		other.IsAllocated(),
		other.GetMessage(),
		&pids,
	)
}

// GetConsumerID :
func (ar *AllocationResponse) GetConsumerID() string {
	return ar.consumerID
}

// IsAllocated :
func (ar *AllocationResponse) IsAllocated() bool {
	return ar.allocated
}

// SetAllocated :
func (ar *AllocationResponse) SetAllocated(allocated bool) {
	ar.allocated = allocated
}

// GetMessage :
func (ar *AllocationResponse) GetMessage() string {
	return ar.message
}

// SetMessage :
func (ar *AllocationResponse) SetMessage(message string) {
	ar.message = message
}

// GetPreemptedIds :
func (ar *AllocationResponse) GetPreemptedIds() []string {
	pids := make([]string, len(ar.preemptedIDs))
	i := 0
	for k := range ar.preemptedIDs {
		pids[i] = k
		i++
	}
	return pids
}

// String
func (ar *AllocationResponse) String() string {
	var b bytes.Buffer
	b.WriteString("AllocationResponse: ")
	fmt.Fprintf(&b, "consumer=%s; ", ar.consumerID)
	fmt.Fprintf(&b, "allocated=%v; ", ar.allocated)
	fmt.Fprintf(&b, "message=\"%s\"; ", ar.message)
	fmt.Fprintf(&b, "preemptedIds=")
	fmt.Fprintf(&b, "%v", ar.GetPreemptedIds())
	fmt.Fprintf(&b, "\n")
	return b.String()
}
