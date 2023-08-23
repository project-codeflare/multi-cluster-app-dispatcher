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
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
)

func GetXQJFullName(qj *arbv1.AppWrapper) string {
	// Use underscore as the delimiter because it is not allowed in qj name
	// (DNS subdomain format).
	return qj.Name + "_" + qj.Namespace
}

func HigherSystemPriorityQJ(qj1, qj2 *arbv1.AppWrapper) bool {
	return qj1.Status.SystemPriority > qj2.Status.SystemPriority
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

// PendingPodsFailedSchd checks if pods pending have failed scheduling
func PendingPodsFailedSchd(pods []v1.Pod) map[string][]v1.PodCondition {
	var podCondition = make(map[string][]v1.PodCondition)
	for i := range pods {
		if pods[i].Status.Phase == v1.PodPending {
			for _, cond := range pods[i].Status.Conditions {
				// Hack: ignore pending pods due to co-scheduler FailedScheduling event
				// this exists until coscheduler performance issue is resolved.
				if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse && cond.Reason == v1.PodReasonUnschedulable {
					if strings.Contains(cond.Message, "pgName") && strings.Contains(cond.Message, "last") && strings.Contains(cond.Message, "failed") && strings.Contains(cond.Message, "deny") {
						// ignore co-scheduled pending pods for coscheduler version:0.22.6
						continue
					} else if strings.Contains(cond.Message, "optimistic") && strings.Contains(cond.Message, "rejection") && strings.Contains(cond.Message, "PostFilter") ||
						strings.Contains(cond.Message, "cannot") && strings.Contains(cond.Message, "find") && strings.Contains(cond.Message, "enough") && strings.Contains(cond.Message, "sibling") {
						// ignore co-scheduled pending pods for coscheduler version:0.23.10
						continue
					} else {
						podName := pods[i].Name
						podCondition[podName] = append(podCondition[podName], *cond.DeepCopy())
					}
				}
			}
		}
	}
	return podCondition
}

// filterPods returns pods based on their phase.
func FilterPods(pods []v1.Pod, phase v1.PodPhase) int {
	result := 0
	for i := range pods {
		if phase == pods[i].Status.Phase {
			result++
		}
	}
	return result
}

//GetPodResourcesByPhase returns pods based on their phase.
func GetPodResourcesByPhase(phase v1.PodPhase, pods []v1.Pod) *clusterstateapi.Resource {
	req := clusterstateapi.EmptyResource()
	for i := range pods {
		if pods[i].Status.Phase == phase {
			for _, c := range pods[i].Spec.Containers {
				req.Add(clusterstateapi.NewResource(c.Resources.Requests))
			}
		}
	}
	return req
}

//GeneratePodFailedCondition returns condition of a AppWrapper condition.
func GeneratePodFailedCondition(podName string, podCondition []v1.PodCondition) arbv1.PendingPodSpec {
	return arbv1.PendingPodSpec{
		PodName:    podName,
		Conditions: podCondition,
	}
}
