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

package core

import (
	"reflect"
	"sort"
)

// EqualStateQuotaNodes : check if two quota nodes have similar allocation data
func EqualStateQuotaNodes(qn1 *QuotaNode, qn2 *QuotaNode) bool {
	consumers1 := make([]*Consumer, len(qn1.GetConsumers()))
	copy(consumers1, qn1.GetConsumers())
	sort.Sort(ByID(consumers1))

	consumers2 := make([]*Consumer, len(qn2.GetConsumers()))
	copy(consumers2, qn2.GetConsumers())
	sort.Sort(ByID(consumers2))

	return reflect.DeepEqual(qn1.GetQuota(), qn2.GetQuota()) &&
		reflect.DeepEqual(qn1.GetAllocated(), qn2.GetAllocated()) &&
		reflect.DeepEqual(consumers1, consumers2)
}

// EqualStateQuotaTrees : check if two quota trees have similar allocation data
func EqualStateQuotaTrees(qt1 *QuotaTree, qt2 *QuotaTree) bool {
	nodeMap1 := qt1.GetNodes()
	nodeMap2 := qt2.GetNodes()
	if len(nodeMap1) != len(nodeMap2) {
		return false
	}
	for k, qn1 := range nodeMap1 {
		if qn2, exists := nodeMap2[k]; exists {
			if !EqualStateQuotaNodes(qn1, qn2) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// EqualStateControllers : check if two controllers have similar allocation state
func EqualStateControllers(c1 *Controller, c2 *Controller) bool {
	pc1 := c1.GetPreemptedConsumers()
	sort.Strings(pc1)
	pc2 := c2.GetPreemptedConsumers()
	sort.Strings(pc2)

	pca1 := c1.GetPreemptedConsumersArray()
	sort.Sort(ByID(pca1))
	pca2 := c2.GetPreemptedConsumersArray()
	sort.Sort(ByID(pca2))

	return EqualStateQuotaTrees(c1.GetTree(), c2.GetTree()) &&
		reflect.DeepEqual(c1.GetConsumers(), c1.GetConsumers()) &&
		reflect.DeepEqual(pc1, pc2) &&
		reflect.DeepEqual(pca1, pca2)
}

// EqualStateControllers : check if two forest controllers have similar allocation state
func EqualStateForestControllers(fc1 *ForestController, fc2 *ForestController) bool {
	c1Map := fc1.GetControllers()
	c2Map := fc2.GetControllers()
	if len(c1Map) != len(c2Map) {
		return false
	}
	for k, c1 := range c1Map {
		if c2, exists := c2Map[k]; exists {
			if !EqualStateControllers(c1, c2) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
