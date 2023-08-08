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

package queuejob

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
)

func GetXQJFullName(qj *arbv1.AppWrapper) string {
	// Use underscore as the delimiter because it is not allowed in qj name
	// (DNS subdomain format).
	return qj.Name + "_" + qj.Namespace
}

func HigherSystemPriorityQJ(qj1, qj2 interface{}) bool {
	return qj1.(*arbv1.AppWrapper).Status.SystemPriority > qj2.(*arbv1.AppWrapper).Status.SystemPriority
}

// GenerateAppWrapperCondition returns condition of a AppWrapper condition.
func GenerateAppWrapperCondition(condType arbv1.AppWrapperConditionType, condStatus corev1.ConditionStatus, condReason string, condMsg string) arbv1.AppWrapperCondition {
	return arbv1.AppWrapperCondition{
		Type:                    condType,
		Status:                  condStatus,
		LastUpdateMicroTime:     metav1.NowMicro(),
		LastTransitionMicroTime: metav1.NowMicro(),
		Reason:                  condReason,
		Message:                 condMsg,
	}
}

func isLastConditionDuplicate(aw *arbv1.AppWrapper, condType arbv1.AppWrapperConditionType, condStatus corev1.ConditionStatus, condReason string, condMsg string) bool {
	if aw.Status.Conditions == nil {
		return false
	}

	lastIndex := len(aw.Status.Conditions) - 1

	if lastIndex < 0 {
		return false
	}

	lastCond := aw.Status.Conditions[lastIndex]
	if (lastCond.Type == condType) &&
		(lastCond.Status == condStatus) &&
		(lastCond.Reason == condReason) &&
		(lastCond.Message == condMsg) {
		return true
	} else {
		return false
	}
}

func getIndexOfMatchedCondition(aw *arbv1.AppWrapper, condType arbv1.AppWrapperConditionType, condReason string) int {
	var index = -1

	for i, cond := range aw.Status.Conditions {
		if cond.Type == condType && cond.Reason == condReason {
			return i
		}
	}
	return index
}
