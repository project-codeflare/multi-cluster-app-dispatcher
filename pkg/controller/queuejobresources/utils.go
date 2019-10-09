package queuejobresources

import(
    //schedulerapi "github.com/kubernetes-sigs/kube-batch/pkg/scheduler/api"
    clusterstateapi "github.com/kubernetes-sigs/kube-batch/pkg/controller/clusterstate/api"
	"k8s.io/api/core/v1"
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

func GetPodResources(template *v1.PodTemplateSpec) *clusterstateapi.Resource {
        total := clusterstateapi.EmptyResource()
        req := clusterstateapi.EmptyResource()
        limit := clusterstateapi.EmptyResource()
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
