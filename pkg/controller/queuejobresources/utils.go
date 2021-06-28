package queuejobresources

import (
	//schedulerapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/scheduler/api"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
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
