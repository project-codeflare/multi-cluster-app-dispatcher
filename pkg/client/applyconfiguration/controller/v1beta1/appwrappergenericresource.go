/*
Copyright 2019, 2021, 2022, 2023 The Multi-Cluster App Dispatcher Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// AppWrapperGenericResourceApplyConfiguration represents an declarative configuration of the AppWrapperGenericResource type for use
// with apply.
type AppWrapperGenericResourceApplyConfiguration struct {
	DesiredAvailable   *int32                                        `json:"replicas,omitempty"`
	MinAvailable       *int32                                        `json:"minavailable,omitempty"`
	Allocated          *int32                                        `json:"allocated,omitempty"`
	Priority           *int32                                        `json:"priority,omitempty"`
	PrioritySlope      *float64                                      `json:"priorityslope,omitempty"`
	GenericTemplate    *runtime.RawExtension                         `json:"generictemplate,omitempty"`
	CustomPodResources []CustomPodResourceTemplateApplyConfiguration `json:"custompodresources,omitempty"`
	CompletionStatus   *string                                       `json:"completionstatus,omitempty"`
}

// AppWrapperGenericResourceApplyConfiguration constructs an declarative configuration of the AppWrapperGenericResource type for use with
// apply.
func AppWrapperGenericResource() *AppWrapperGenericResourceApplyConfiguration {
	return &AppWrapperGenericResourceApplyConfiguration{}
}

// WithDesiredAvailable sets the DesiredAvailable field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the DesiredAvailable field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithDesiredAvailable(value int32) *AppWrapperGenericResourceApplyConfiguration {
	b.DesiredAvailable = &value
	return b
}

// WithMinAvailable sets the MinAvailable field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MinAvailable field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithMinAvailable(value int32) *AppWrapperGenericResourceApplyConfiguration {
	b.MinAvailable = &value
	return b
}

// WithAllocated sets the Allocated field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Allocated field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithAllocated(value int32) *AppWrapperGenericResourceApplyConfiguration {
	b.Allocated = &value
	return b
}

// WithPriority sets the Priority field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Priority field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithPriority(value int32) *AppWrapperGenericResourceApplyConfiguration {
	b.Priority = &value
	return b
}

// WithPrioritySlope sets the PrioritySlope field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the PrioritySlope field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithPrioritySlope(value float64) *AppWrapperGenericResourceApplyConfiguration {
	b.PrioritySlope = &value
	return b
}

// WithGenericTemplate sets the GenericTemplate field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the GenericTemplate field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithGenericTemplate(value runtime.RawExtension) *AppWrapperGenericResourceApplyConfiguration {
	b.GenericTemplate = &value
	return b
}

// WithCustomPodResources adds the given value to the CustomPodResources field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the CustomPodResources field.
func (b *AppWrapperGenericResourceApplyConfiguration) WithCustomPodResources(values ...*CustomPodResourceTemplateApplyConfiguration) *AppWrapperGenericResourceApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithCustomPodResources")
		}
		b.CustomPodResources = append(b.CustomPodResources, *values[i])
	}
	return b
}

// WithCompletionStatus sets the CompletionStatus field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CompletionStatus field is set to the value of the last call.
func (b *AppWrapperGenericResourceApplyConfiguration) WithCompletionStatus(value string) *AppWrapperGenericResourceApplyConfiguration {
	b.CompletionStatus = &value
	return b
}
