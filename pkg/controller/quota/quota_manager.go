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
	"fmt"
	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	listersv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"github.com/golang/glog"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strings"
	"time"
)

const (
	// AW Namespace used for building unique name for AW job
	NamespacePrefix string = "NAMESPACE_"

	// AW Name used for building unique name for AW job
	AppWrapperNamePrefix string = "_AWNAME_"
)
var quotaContext 	    = "quota_context"

// QuotaManager is an interface quota management solutions
type QuotaManager interface {
	Fits(aw *arbv1.AppWrapper, resources *clusterstateapi.Resource, proposedPremptions []*arbv1.AppWrapper) (bool, []*arbv1.AppWrapper)
	Release(aw *arbv1.AppWrapper) bool
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
	url 			string
	appwrapperLister 	listersv1.AppWrapperLister
	preemptionEnabled 	bool
}

type QuotaGroup struct {
	GroupContext string  `json:"groupcontext"`
	GroupId	     string  `json:"groupid"`
}

type Request struct {
	Id          string   	  `json:"id"`
	Groups      []QuotaGroup  `json:"groups"`
	Demand      []int         `json:"demand"`
	Priority    int           `json:"priority"`
	Preemptable bool          `json:"preemptable"`
}

type QuotaResponse struct {
	Id          string   	   `json:"id"`
	Groups      []QuotaGroup   `json:"groups"`
	Demand      []int    	   `json:"demand"`
	Priority    int      	   `json:"priority"`
	Preemptable bool     	   `json:"preemptable"`
	PreemptIds  []string 	   `json:"preemptedIds"`
	CreateDate  string   	   `json:"dateCreated"`
}

type TreeNode struct {
	Allocation	string     	`json:"allocation"`
	Quota		string  	`json:"quota"`
	Name		string   	`json:"name"`
	Hard		bool     	`json:"hard"`
	Children	[]TreeNode   	`json:"children"`
	Parent 		string 		`json:"parent"`
}


// NewQuotaManager initializes a new scheduling queue.
func NewQuotaManager() QuotaManager {
		return NewResourcePlanManager( nil,"", false)
}

// Making sure that PriorityQueue implements SchedulingQueue.
var _ = QuotaManager(&ResourcePlanManager{})


func parseId(id string) (string, string) {
	ns := ""
	n := ""

	// Extract the namespace seperator
	nspSplit := strings.Split(id, NamespacePrefix)
	if len(nspSplit) == 2 {
		// Extract the appwrapper seperator
		awnpSplit := strings.Split(nspSplit[1], AppWrapperNamePrefix)
		if len(awnpSplit) == 2 {
			// What is left if the namespace value in the first slice
			if len(awnpSplit[0]) > 0 {
				ns = awnpSplit[0]
			}
			// And the names value in the second slice
			if len(awnpSplit[1]) > 0 {
				n = awnpSplit[1]
			}
		}
	}
	return ns, n
}

func createId(ns string, n string) string {
	id := ""
	if len(ns) > 0 && len(n) > 0 {
		id = fmt.Sprintf("%s%s%s%s", NamespacePrefix, ns, AppWrapperNamePrefix, n)
	}
	return id
}

func NewResourcePlanManager(awJobLister listersv1.AppWrapperLister, quotaManagerUrl string, preemptionEnabled bool) *ResourcePlanManager {
	rpm := &ResourcePlanManager{
		appwrapperLister: awJobLister,
		url: quotaManagerUrl,
		preemptionEnabled: preemptionEnabled,
	}
	return rpm
}

// Recrusive call to add names of Tree
func (rpm *ResourcePlanManager) addChildrenNodes(parentNode TreeNode, treeIDs []string) ([]string) {
	if len(parentNode.Children) > 0 {
		for _, childNode := range parentNode.Children {
			glog.V(10).Infof("[getQuotaTreeIDs] Quota tree response child node from quota mananger: %s", childNode.Name)
			treeIDs = rpm.addChildrenNodes(childNode, treeIDs)
		}
	}
	treeIDs = append(treeIDs, parentNode.Name)
	return treeIDs
}

func (rpm *ResourcePlanManager) getQuotaTreeIDs() ([]string) {
	var treeIDs []string
	// If a url does not exists then assume fits quota
	if len(rpm.url) < 1 {
		return treeIDs
	}

	uri := rpm.url + "/json"

	glog.V(10).Infof("[getQuotaTreeIDs] Sending GET request to uri: %s", uri)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		glog.Errorf("[getQuotaTreeIDs] Fail to create client to access quota manager: %s, err=%#v.", uri, err)
		return treeIDs
	}

	quotaRestClient := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}
	response, err := quotaRestClient.Do(req)
	if err != nil {
		glog.Errorf("[getQuotaTreeIDs] Fail to access quota manager: %s, err=%#v.", uri, err)
		return treeIDs
	} else {

		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			glog.Errorf("[getQuotaTreeIDs] Failed to read quota tree from the quota manager body: %s, error=%#v",
											string(body), err)
			return treeIDs
		}

		var quotaTreesResponse []TreeNode
		if err := json.Unmarshal(body, &quotaTreesResponse); err != nil {
			glog.Errorf("[getQuotaTreeIDs] Failed to decode json from quota manager body: %s, error=%#v", string(body), err)
		} else {
			// Loop over root nodes of trees and add names of each node in tree.
			for _, treeroot := range quotaTreesResponse {
				glog.V(6).Infof("[getQuotaTreeIDs] Quota tree response root node from quota mananger: %s", treeroot.Name)
				treeIDs = rpm.addChildrenNodes(treeroot, treeIDs)
			}
		}

		glog.V(10).Infof("[getQuotaTreeIDs] Response from quota mananger status: %s", response.Status)
		statusCode := response.StatusCode
		glog.V(8).Infof("[getQuotaTreeIDs] Response from quota mananger status code: %v", statusCode)

	}
	return treeIDs
}

func isValidQuota(quotaGroup QuotaGroup, qmTreeIDs []string) bool {
	for _, treeNodeID := range qmTreeIDs {
		if treeNodeID == quotaGroup.GroupId {
			return true
		}
	}
	return false
}

func (rpm *ResourcePlanManager) getQuotaDesignation(aw *arbv1.AppWrapper) ([]QuotaGroup) {
	var groups []QuotaGroup

	// Get list of quota management tree IDs
	qmTreeIDs := rpm.getQuotaTreeIDs()
	if len(qmTreeIDs) < 1 {
		glog.Warningf("[getQuotaDesignation] No valid quota management IDs found for AppWrapper Job: %s/%s",
									aw.Namespace, aw.Name)
	}

	labels := aw.GetLabels()
	if ( labels != nil) {
		keys := reflect.ValueOf(labels).MapKeys()
		for _,  key := range keys {
			strkey := key.String()
			quotaGroup := QuotaGroup{
				GroupContext: strkey,
				GroupId: labels[strkey],
			}
			if isValidQuota(quotaGroup, qmTreeIDs) {
				groups = append(groups, quotaGroup)
				glog.V(8).Infof("[getQuotaDesignation] AppWrapper: %s/%s quota label: %v found.",
					aw.Namespace, aw.Name, quotaGroup)
			} else {
				glog.V(10).Infof("[getQuotaDesignation] AppWrapper: %s/%s label: %v ignored.  Not a valid quota ID.",
					aw.Namespace, aw.Name, quotaGroup)
			}

		}
	} else {
		glog.V(4).Infof("[getQuotaDesignation] AppWrapper: %s/%s does not any context quota labels.",
										aw.Namespace, aw.Name)
	}

	if len(groups) > 0 {
		glog.V(6).Infof("[getQuotaDesignation] AppWrapper: %s/%s quota labels: %v.", aw.Namespace,
			aw.Name, groups)
	} else {
		glog.V(4).Infof("[getQuotaDesignation] AppWrapper: %s/%s does not have any quota labels, using default.",
			aw.Namespace, aw.Name)
		var defaultGroup = QuotaGroup{
			GroupContext: 	"DEFAULTCONTEXT",
			GroupId:	"DEFAULT",
		}
		groups = append(groups, defaultGroup)
	}

	return groups
}

func (rpm *ResourcePlanManager) Fits(aw *arbv1.AppWrapper, awResDemands *clusterstateapi.Resource,
					proposedPreemptions []*arbv1.AppWrapper) (bool, []*arbv1.AppWrapper) {

	// Handle uninitialized quota manager
	if len(rpm.url) <= 0 {
		return true, proposedPreemptions
	}
	awId := createId(aw.Namespace, aw.Name)
	if len(awId) <= 0 {
		glog.Errorf("[Fits] Request failed due to invalid AppWrapper due to empty namespace: %s or name:%s.", aw.Namespace, aw.Name)
		return false, nil
	}

	groups := rpm.getQuotaDesignation(aw)
	preemptable := rpm.preemptionEnabled
	awCPU_Demand := int(math.Trunc(awResDemands.MilliCPU))
	awMem_Demand := int(math.Trunc(awResDemands.Memory)/1000000)
	var demand []int
	demand = append(demand, awCPU_Demand)
	demand = append(demand, awMem_Demand)
	priority := int(aw.Spec.Priority)
	req := Request{
		Id:          awId,
		Groups:      groups,
		Demand:      demand,
		Priority:    priority,
		Preemptable: preemptable,
	}

	doesFit := false
	// If a url does not exists then assume fits quota
	if len(rpm.url) < 1 {
		glog.V(4).Infof("[Fits] No quota manager exists, %+v meets quota by default.", awResDemands)
		return doesFit, nil
	}

	uri := rpm.url + "/quota/alloc"
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(req)
	if err != nil {
		glog.Errorf("[Fits] Failed encoding of request: %v, err=%#v.", req, err)
		return doesFit, nil
	}

	var preemptIds []*arbv1.AppWrapper
	glog.V(4).Infof("[Fits] Sending request: %v in buffer: %v, buffer size: %v, to uri: %s", req, buf, buf.Len(), uri)
	response, err := http.Post(uri, "application/json; charset=utf-8", buf)
	defer response.Body.Close()
	dump, err := httputil.DumpResponse(response, true)
	glog.V(10).Infof("[getQuotaTreeIDs] POST Response dump: %q", dump)

	if err != nil {
		glog.Errorf("[Fits] Fail to add access quotamanager: %s, err=%#v.", uri, err)
		preemptIds = nil
	} else {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			glog.Errorf("[Fits] Failed to read preemption Ids from the quota manager body: %s, error=%#v", string(body), err)
		}
		var quotaResponse QuotaResponse
		if err := json.Unmarshal(body, &quotaResponse); err != nil {
			glog.Errorf("[Fits] Failed to decode json from quota manager body: %s, error=%#v", string(body), err)
		} else {
			glog.V(6).Infof("[Fits] Quota response object from quota mananger body: %v", quotaResponse)
		}
		glog.V(4).Infof("[Fits] Response from quota mananger status: %s", response.Status)
		statusCode := response.StatusCode
		glog.V(4).Infof("[Fits] Response from quota mananger status code: %v", statusCode)
		if statusCode == 200 {
			doesFit = true
			preemptIds = rpm.getAppWrappers(quotaResponse.PreemptIds)
		}
	}
	return doesFit, preemptIds
}


func  (rpm *ResourcePlanManager) getAppWrappers(preemptIds []string) []*arbv1.AppWrapper{
	var aws []*arbv1.AppWrapper
	if len(preemptIds) <= 0 {
		return nil
	}

	for _, preemptId := range preemptIds {
		awNamespace, awName := parseId(preemptId)
		if len(awNamespace) <= 0 || len(awName) <= 0 {
			glog.Errorf("[getAppWrappers] Failed to parse AppWrapper id from quota manager, parse string: %s.  Preemption of this Id will be ignored.", preemptId)
			continue
		}
		aw, e := rpm.appwrapperLister.AppWrappers(awNamespace).Get(awName)
		if e != nil {
			glog.Errorf("[getAppWrappers] Failed to get AppWrapper from API Cache %s/%s, err=%v.  Preemption of this Id will be ignored.",
				awNamespace, awName, e)
			continue
		}
		aws = append(aws, aw)
	}

	// Final validation check
	if len(preemptIds) != len(aws) {
		glog.Warningf("[getAppWrappers] Preemption list size of %d from quota manager does not match size of generated list of AppWrapper: %d", len(preemptIds), len(aws))
	}
	return aws
}
func (rpm *ResourcePlanManager) Release(aw *arbv1.AppWrapper) bool {

	// Handle uninitialized quota manager
	if len(rpm.url) < 0 {
		return true
	}

	released := false
	awId := createId(aw.Namespace, aw.Name)
	if len(awId) <= 0 {
		glog.Errorf("[Release] Request failed due to invalid AppWrapper due to empty namespace: %s or name:%s.", aw.Namespace, aw.Name)
		return false
	}

	uri := rpm.url + "/quota/release/" + awId
	glog.V(4).Infof("[Release] Sending request to release resources for: %s ", uri)

	// Create client
	client := &http.Client{}

	// Create request
	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		glog.Errorf("[Release] Failed to create http delete request for : %s, err=%#v.", awId, err)
		return released
	}

	// Fetch Request
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("[Release] Failed http delete request for: %s, err=%#v.", awId, err)
		return released
	}
	defer resp.Body.Close()

	// Read Response Body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.V(4).Infof("[Release] Failed to aquire response from http delete request id: %s, err=%#v.", awId, err)
	} else {
		glog.V(4).Infof("[Release] Response from quota mananger body: %s", string(respBody))
	}

	glog.V(4).Infof("[Release] Response from quota mananger status: %s", resp.Status)
	statusCode := resp.StatusCode
	glog.V(4).Infof("[Release] Response from quota mananger status code: %v", statusCode)
	if statusCode == 204 {
		released = true
	}

	return released
}