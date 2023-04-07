/*
Copyright 2019 The Kubernetes Authors.

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
package queuejobresources

import (
	"strings"

	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// filterPods returns pods based on their phase.
func FilterPods(pods []*v1.Pod, phase v1.PodPhase) int {
	result := 0
	for i := range pods {
		if phase == pods[i].Status.Phase {
			result++
		}
	}
	return result
}

//check if pods pending are failed scheduling
func PendingPodsFailedSchd(pods []*v1.Pod) map[string][]v1.PodCondition {
	var podCondition = make(map[string][]v1.PodCondition)
	for i := range pods {
		if pods[i].Status.Phase == v1.PodPending {
			for _, cond := range pods[i].Status.Conditions {
				// Hack: ignore pending pods due to co-scheduler FailedScheduling event
				// this exists until coscheduler performance issue is resolved.
				if cond.Type == v1.PodScheduled && cond.Status == v1.ConditionFalse && cond.Reason == v1.PodReasonUnschedulable {
					if strings.Contains(cond.Message, "pgName") && strings.Contains(cond.Message, "last") && strings.Contains(cond.Message, "failed") && strings.Contains(cond.Message, "deny") {
						//ignore co-scheduled pending pods for coscheduler version:0.22.6
						continue
					} else if strings.Contains(cond.Message, "optimistic") && strings.Contains(cond.Message, "rejection") && strings.Contains(cond.Message, "PostFilter") ||
						strings.Contains(cond.Message, "cannot") && strings.Contains(cond.Message, "find") && strings.Contains(cond.Message, "enough") && strings.Contains(cond.Message, "sibling") {
						//ignore co-scheduled pending pods for coscheduler version:0.23.10
						continue
					} else {
						podName := string(pods[i].Name)
						podCondition[podName] = append(podCondition[podName], *cond.DeepCopy())
					}
				}
			}
		}
	}
	return podCondition
}

// filterPods returns pods based on their phase.
func GetPodResourcesByPhase(phase v1.PodPhase, pods []*v1.Pod) *clusterstateapi.Resource {
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

func GetPodResources(template *v1.PodTemplateSpec) *clusterstateapi.Resource {
	total := clusterstateapi.EmptyResource()
	req := clusterstateapi.EmptyResource()
	limit := clusterstateapi.EmptyResource()
	spec := template.Spec

	if &spec == nil {
		klog.Errorf("Pod Spec not found in Pod Template: %+v.  Aggregated resources set to 0.", template)
		return total
	}

	for _, c := range template.Spec.Containers {
		req.Add(clusterstateapi.NewResource(c.Resources.Requests))
		limit.Add(clusterstateapi.NewResource(c.Resources.Limits))
	}
	if req.MilliCPU < limit.MilliCPU {
		req.MilliCPU = limit.MilliCPU
	}
	if req.Memory < limit.Memory {
		req.Memory = limit.Memory
	}
	if req.GPU < limit.GPU {
		req.GPU = limit.GPU
	}
	total = total.Add(req)
	return total
}
