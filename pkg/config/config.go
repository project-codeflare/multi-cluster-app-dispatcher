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

package config

// MCADConfiguration defines the core MCAD configuration.
type MCADConfiguration struct {
	// dynamicPriority sets the controller to use dynamic priority.
	// If false, it sets the controller to use static priority.
	// It defaults to false.
	// +optional
	DynamicPriority *bool `json:"dynamicPriority,omitempty"`

	// preemption sets the controller to allow preemption.
	// Note: when set to true, the Kubernetes scheduler must be configured
	// to enable preemption. It defaults to false.
	// +optional
	Preemption *bool `json:"preemption,omitempty"`

	// backoffTime defines the duration, in seconds, a job will go away,
	// if it can not be scheduled.
	// +optional
	BackoffTime *int32 `json:"backoffTime,omitempty"`

	// headOfLineHoldingTime defines the duration in seconds a job can stay at the
	// Head Of Line without being bumped.
	// It defaults to 0.
	// +optional
	HeadOfLineHoldingTime *int32 `json:"headOfLineHoldingTime,omitempty"`

	// quotaEnabled sets whether quota management is enabled.
	// It defaults to false.
	// +optional
	QuotaEnabled *bool `json:"quotaEnabled,omitempty"`
}

// MCADConfigurationExtended defines the extended MCAD configuration, e.g.,
// for experimental features.
type MCADConfigurationExtended struct {
	// dispatcher sets the controller in dispatcher mode, of in agent mode.
	// It defaults to false.
	// +optional
	Dispatcher *bool `json:"dispatcher,omitempty"`

	// agentConfigs contains paths to agent config file
	AgentConfigs []string `json:"agentConfigs,omitempty"`
}
