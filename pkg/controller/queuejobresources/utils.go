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
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
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

// filterPods returns pods based on their phase.
func GetPodResourcesByPhase(phase v1.PodPhase, pods []*v1.Pod) *clusterstateapi.Resource {
    req := clusterstateapi.EmptyResource()
    for i := range pods {
        if  pods[i].Status.Phase == phase {
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
