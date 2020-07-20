/*
Copyright 2017 The Kubernetes Authors.

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

// This file contains structures that implement scheduling queue types.
// Scheduling queues hold pods waiting to be scheduled. This file has two types
// of scheduling queue: 1) a FIFO, which is mostly the same as cache.FIFO, 2) a
// priority queue which has two sub queues. One sub-queue holds pods that are
// being considered for scheduling. This is called activeQ. Another queue holds
// pods that are already tried and are determined to be unschedulable. The latter
// is called unschedulableQ.
// FIFO is here for flag-gating purposes and allows us to use the traditional
// scheduling queue when util.PodPriorityEnabled() returns false.

package quotamanager


import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math"
	"net/http"

	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	"github.com/golang/glog"
)

// QuotaManager is an interface quota management solutions
type QuotaManager interface {
	Fits(aw *arbv1.AppWrapper, resources *clusterstateapi.Resource) bool
}

// PriorityQueue implements a scheduling queue. It is an alternative to FIFO.
// The head of PriorityQueue is the highest priority pending QJ. This structure
// has two sub queues. One sub-queue holds QJ that are being considered for
// scheduling. This is called activeQ and is a Heap. Another queue holds
// pods that are already tried and are determined to be unschedulable. The latter
// is called unschedulableQ.
// Heap is already thread safe, but we need to acquire another lock here to ensure
// atomicity of operations on the two data structures..
type ResourcePlanManager struct {
	url string
}

type Request struct {
	id          string
	group       string
	demand      int
	priority    int
	preemptable bool
}

// NewSchedulingQueue initializes a new scheduling queue. If pod priority is
// enabled a priority queue is returned. If it is disabled, a FIFO is returned.
func NewQuotaManager() QuotaManager {
		return NewResourcePlanManager()
}

// Making sure that PriorityQueue implements SchedulingQueue.
var _ = QuotaManager(&ResourcePlanManager{})


func NewResourcePlanManager(host string, port string) *ResourcePlanManager {
	url := host + ":" + port
	rpm := &ResourcePlanManager{
		url: url,
	}
	return rpm
}

func (rpm *ResourcePlanManager) Fits(aw *arbv1.AppWrapper, resources *clusterstateapi.Resource) bool {

	awId := aw.Namespace + aw.Name
	awCPU_Demand := int(math.Trunc(resources.MilliCPU / 1000))
	req := Request{
		id:          awId,
		group:       "M",
		demand:      awCPU_Demand,
		priority:    0,
		preemptable: false,
	}

	doesFit := true
	// If a url does not exists then assume fits quota
	if len(rpm.url) < 1 {
		glog.V(6).Infof("[Fits] No quota manager exists, %+v meets quota by default.", resources)
		return doesFit
	}

	//jsonData := map[string]string{"firstname": "Nic", "lastname": "Raboy"}
	jsonValue, _ := json.Marshal(req)
	uri := rpm.url + "/quota/alloc"
	response, err := http.Post(uri, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		glog.Errorf("[Fits] Fail to add access quotamanager: %s, err=%#v.", rpm.url, err)

	} else {
		data, _ := ioutil.ReadAll(response.Body)
		glog.V(4).Infof("[Fits] Response from quota mananger body: %s", string(data))
		glog.V(4).Infof("[Fits] Response from quota mananger status: %s", response.Status)
		glog.V(4).Infof("[Fits] Response from quota mananger status code: %v", response.StatusCode)
	}
	return true
}
