/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

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

package api

import (
	v1 "k8s.io/api/core/v1"
)

func NewStringsMap(source map[string]string) map[string]string {
	target := make(map[string]string)

	for k, v := range source {
		target[k] = v
	}
	return target
}

func NewTaints(source []v1.Taint) []v1.Taint {
	var target []v1.Taint
	if source == nil {
		target = make([]v1.Taint, 0)
		return target
	}

	target = make([]v1.Taint, len(source))
	for _, t := range source {

		newTaint := v1.Taint{
			Key:       t.Key,
			Value:     t.Value,
			Effect:    t.Effect,
			TimeAdded: t.TimeAdded,
		}
		target = append(target, newTaint)
	}
	return target
}
