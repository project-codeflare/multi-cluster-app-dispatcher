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

// Allocation : an allocation of an (ordered) array of resources
// (names of resources are left out for efficiency)
type Allocation struct {
	// values of the allocation
	x []int
}

// NewAllocation : create an empty allocation of a given size (length)
func NewAllocation(size int) (*Allocation, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid size %d", size)
	}
	return &Allocation{
		x: make([]int, size),
	}, nil
}

// NewAllocationCopy : create an allocation given an array of values
func NewAllocationCopy(value []int) (*Allocation, error) {
	a, err := NewAllocation(len(value))
	if err != nil {
		return nil, err
	}
	copy(a.x, value)
	return a, nil
}

// GetSize : get the size (length) of the values array
func (a *Allocation) GetSize() int {
	return len(a.x)
}

// GetValue : get the array of values
func (a *Allocation) GetValue() []int {
	return a.x
}

// SetValue : set the array of values (overwites previous values)
func (a *Allocation) SetValue(value []int) {
	a.x = make([]int, len(value))
	copy(a.x, value)
}

// Clone : create a copy
func (a *Allocation) Clone() *Allocation {
	alloc, _ := NewAllocationCopy(a.x)
	return alloc
}

// Add : add another allocation to this one (false if unequal lengths)
func (a *Allocation) Add(other *Allocation) bool {
	if !a.SameSize(other) {
		return false
	}
	v := other.GetValue()
	for i := 0; i < len(a.x); i++ {
		a.x[i] += v[i]
	}
	return true
}

// Subtract : subtract another allocation to this one (false if unequal lengths)
func (a *Allocation) Subtract(other *Allocation) bool {
	if !a.SameSize(other) {
		return false
	}
	v := other.GetValue()
	for i := 0; i < len(a.x); i++ {
		a.x[i] -= v[i]
	}
	return true
}

// Fit : check if this allocation fits on an entity with a given capacity
// and already allocated values (false if unequal lengths)
func (a *Allocation) Fit(allocated *Allocation, capacity *Allocation) bool {
	available := capacity.Clone()
	if available.Subtract(allocated) {
		return a.LessOrEqual(available)
	}
	return false
}

// SameSize : check if same size (length) as another allocation
func (a *Allocation) SameSize(other *Allocation) bool {
	return a.GetSize() == other.GetSize()
}

// IsZero : check if values are zeros (all element values)
func (a *Allocation) IsZero() bool {
	for i := 0; i < len(a.x); i++ {
		if a.x[i] != 0 {
			return false
		}
	}
	return true
}

// Equal : check if equals another allocation (false if unequal lengths)
func (a *Allocation) Equal(other *Allocation) bool {
	return a.comp(other, false)
}

// LessOrEqual : check if less or equal to another allocation (false if unequal lengths)
func (a *Allocation) LessOrEqual(other *Allocation) bool {
	return a.comp(other, true)
}

// comp : compare this to another allocation (false if unequal lengths);
// lessOrEqual: true => 'less or equal'; false => 'equal'
func (a *Allocation) comp(other *Allocation, lessOrEqual bool) bool {
	if a.SameSize(other) {
		v := other.GetValue()
		condition := true
		for i := 0; i < len(a.x) && condition; i++ {
			if lessOrEqual {
				condition = a.x[i] <= v[i]
			} else {
				condition = a.x[i] == v[i]
			}
		}
		return condition
	}
	return false
}

// String : a print out of the allocation
func (a *Allocation) String() string {
	return fmt.Sprint(a.x)
}

// StringPretty : a print out of the allocation with resource names (empty if unequal lengths)
func (a *Allocation) StringPretty(resourceNames []string) string {
	n := len(a.x)
	if len(resourceNames) != n {
		return ""
	}
	var b bytes.Buffer
	b.WriteString("[")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s:%d", resourceNames[i], a.x[i])
		if i < n-1 {
			b.WriteString(", ")
		}
	}
	b.WriteString("]")
	return b.String()
}
