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

func (c *MCADConfiguration) IsQuotaEnabled() bool {
	return isTrue(c.QuotaEnabled)
}

func (c *MCADConfiguration) HasPreemption() bool {
	return isTrue(c.Preemption)
}

func (c *MCADConfiguration) HasDynamicPriority() bool {
	return isTrue(c.DynamicPriority)
}

func (c *MCADConfiguration) BackoffTimeOrDefault(val int32) int32 {
	if c.BackoffTime == nil {
		return val
	}
	return *c.BackoffTime
}

func (e *MCADConfigurationExtended) IsDispatcher() bool {
	return isTrue(e.Dispatcher)
}

func isTrue(v *bool) bool {
	return v != nil && *v
}
