// +build !private
// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// Copyright 2022, 2023 The Multi-Cluster App Dispatcher Authors.
// 
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// ------------------------------------------------------ {COPYRIGHT-END} ---

package quotamanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	listersv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/listers/controller/v1"
	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/util"

	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"math"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strings"
	"time"
)

// QuotaManager implements a QuotaManagerInterface.
type QuotaManager struct {
	url 			string
	appwrapperLister 	listersv1.AppWrapperLister
	preemptionEnabled 	bool
}

type QuotaGroup struct {
	GroupContext string  `json:"groupcontext"`
	GroupId	     string  `json:"groupid"`
}

type Request struct {
	Id          string       `json:"id"`
	Groups      []QuotaGroup `json:"groups"`
	Demand      []int        `json:"demand"`
	Priority    int          `json:"priority"`
	Preemptable bool         `json:"preemptable"`
}

type QuotaResponse struct {
	Id          string       `json:"id"`
	Groups      []QuotaGroup `json:"groups"`
	Demand      []int        `json:"demand"`
	Priority    int          `json:"priority"`
	Preemptable bool         `json:"preemptable"`
	PreemptIds  []string     `json:"preemptedIds"`
	CreateDate  string       `json:"dateCreated"`
}

type TreeNode struct {
	Allocation	string     `json:"allocation"`
	Quota		string  `json:"quota"`
	Name		string   `json:"name"`
	Hard		bool     `json:"hard"`
	Children	[]TreeNode   `json:"children"`
	Parent 		string `json:"parent"`
}


// Making sure that PriorityQueue implements SchedulingQueue.
var _ = quota.QuotaManagerInterface(&QuotaManager{})


func parseId(id string) (string, string) {
	ns := ""
	n := ""

	// Extract the namespace seperator
	nspSplit := strings.Split(id, util.NamespacePrefix)
	if len(nspSplit) == 2 {
		// Extract the appwrapper seperator
		awnpSplit := strings.Split(nspSplit[1], util.AppWrapperNamePrefix)
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
		id = fmt.Sprintf("%s%s%s%s", util.NamespacePrefix, ns, util.AppWrapperNamePrefix, n)
	}
	return id
}

func NewQuotaManager(dispatchedAWDemands map[string]*clusterstateapi.Resource, dispatchedAWs map[string]*arbv1.AppWrapper,
			awJobLister listersv1.AppWrapperLister, config *rest.Config,
				serverOptions *options.ServerOption) (*QuotaManager, error) {
	if serverOptions.QuotaEnabled == false {
		klog.Infof("[NewQuotaManager] Quota management is not enabled.")
		return nil, nil
	}

	qm := &QuotaManager{
		url:                 serverOptions.QuotaRestURL,
		appwrapperLister:    awJobLister,
		preemptionEnabled:   serverOptions.Preemption,
	}

	return qm, nil
}

// Recrusive call to add names of Tree
func (qm *QuotaManager) addChildrenNodes(parentNode TreeNode, treeIDs []string) ([]string) {
	if len(parentNode.Children) > 0 {
		for _, childNode := range parentNode.Children {
			klog.V(10).Infof("[getQuotaTreeIDs] Quota tree response child node from quota mananger: %s", childNode.Name)
			treeIDs = qm.addChildrenNodes(childNode, treeIDs)
		}
	}
	treeIDs = append(treeIDs, parentNode.Name)
	return treeIDs
}

func (qm *QuotaManager) getQuotaTreeIDs() ([]string) {
	var treeIDs []string
	// If a url does not exists then assume fits quota
	if len(qm.url) < 1 {
		return treeIDs
	}

	uri := qm.url + "/json"

	klog.V(10).Infof("[getQuotaTreeIDs] Sending GET request to uri: %s", uri)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		klog.Errorf("[getQuotaTreeIDs] Fail to create client to access quota manager: %s, err=%#v.", uri, err)
		return treeIDs
	}

	quotaRestClient := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}
	response, err := quotaRestClient.Do(req)
	if err != nil {
		klog.Errorf("[getQuotaTreeIDs] Fail to access quota manager: %s, err=%#v.", uri, err)
		return treeIDs
	} else {

		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			klog.Errorf("[getQuotaTreeIDs] Failed to read quota tree from the quota manager body: %s, error=%#v",
											string(body), err)
			return treeIDs
		}

		var quotaTreesResponse []TreeNode
		if err := json.Unmarshal(body, &quotaTreesResponse); err != nil {
			klog.Errorf("[getQuotaTreeIDs] Failed to decode json from quota manager body: %s, error=%#v", string(body), err)
		} else {
			// Loop over root nodes of trees and add names of each node in tree.
			for _, treeroot := range quotaTreesResponse {
				klog.V(6).Infof("[getQuotaTreeIDs] Quota tree response root node from quota mananger: %s", treeroot.Name)
				treeIDs = qm.addChildrenNodes(treeroot, treeIDs)
			}
		}

		klog.V(10).Infof("[getQuotaTreeIDs] Response from quota mananger status: %s", response.Status)
		statusCode := response.StatusCode
		klog.V(8).Infof("[getQuotaTreeIDs] Response from quota mananger status code: %v", statusCode)

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

func (qm *QuotaManager) getQuotaDesignation(aw *arbv1.AppWrapper) ([]QuotaGroup) {
	var groups []QuotaGroup

	// Get list of quota management tree IDs
	qmTreeIDs := qm.getQuotaTreeIDs()
	if len(qmTreeIDs) < 1 {
		klog.Warningf("[getQuotaDesignation] No valid quota management IDs found for AppWrapper Job: %s/%s",
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
				klog.V(8).Infof("[getQuotaDesignation] AppWrapper: %s/%s quota label: %v found.",
					aw.Namespace, aw.Name, quotaGroup)
			} else {
				klog.V(10).Infof("[getQuotaDesignation] AppWrapper: %s/%s label: %v ignored.  Not a valid quota ID.",
					aw.Namespace, aw.Name, quotaGroup)
			}

		}
	} else {
		klog.V(4).Infof("[getQuotaDesignation] AppWrapper: %s/%s does not any context quota labels.",
										aw.Namespace, aw.Name)
	}

	if len(groups) > 0 {
		klog.V(6).Infof("[getQuotaDesignation] AppWrapper: %s/%s quota labels: %v.", aw.Namespace,
			aw.Name, groups)
	} else {
		klog.V(4).Infof("[getQuotaDesignation] AppWrapper: %s/%s does not have any quota labels, using default.",
			aw.Namespace, aw.Name)
		var defaultGroup = QuotaGroup{
			GroupContext: 	"DEFAULTCONTEXT",
			GroupId:	"DEFAULT",
		}
		groups = append(groups, defaultGroup)
	}

	return groups
}

func (qm *QuotaManager) Fits(aw *arbv1.AppWrapper, awResDemands *clusterstateapi.Resource,
					proposedPreemptions []*arbv1.AppWrapper) (bool, []*arbv1.AppWrapper, string) {

	// Handle uninitialized quota manager
	if len(qm.url) <= 0 {
		return true, proposedPreemptions, ""
	}
	awId := createId(aw.Namespace, aw.Name)
	if len(awId) <= 0 {
		klog.Errorf("[Fits] Request failed due to invalid AppWrapper due to empty namespace: %s or name:%s.", aw.Namespace, aw.Name)
		return false, nil, ""
	}

	groups := qm.getQuotaDesignation(aw)
	preemptable := qm.preemptionEnabled
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
	if len(qm.url) < 1 {
		klog.V(4).Infof("[Fits] No quota manager exists, %#v meets quota by default.", awResDemands)
		return doesFit, nil, ""
	}

	uri := qm.url + "/quota/alloc"
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(req)
	if err != nil {
		klog.Errorf("[Fits] Failed encoding of request: %v, err=%#v.", req, err)
		return doesFit, nil, ""
	}

	var preemptIds []*arbv1.AppWrapper
	klog.V(4).Infof("[Fits] Sending request: %v in buffer: %v, buffer size: %v, to uri: %s", req, buf, buf.Len(), uri)
	response, err := http.Post(uri, "application/json; charset=utf-8", buf)
	defer response.Body.Close()
	dump, err := httputil.DumpResponse(response, true)
	klog.V(10).Infof("[getQuotaTreeIDs] POST Response dump: %q", dump)

	if err != nil {
		klog.Errorf("[Fits] Fail to add access quotaforestmanager: %s, err=%#v.", uri, err)
		preemptIds = nil
	} else {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			klog.Errorf("[Fits] Failed to read preemption Ids from the quota manager body: %s, error=%#v", string(body), err)
		}
		var quotaResponse QuotaResponse
		if err := json.Unmarshal(body, &quotaResponse); err != nil {
			klog.Errorf("[Fits] Failed to decode json from quota manager body: %s, error=%#v", string(body), err)
		} else {
			klog.V(6).Infof("[Fits] Quota response object from quota mananger body: %v", quotaResponse)
		}
		klog.V(4).Infof("[Fits] Response from quota mananger status: %s", response.Status)
		statusCode := response.StatusCode
		klog.V(4).Infof("[Fits] Response from quota mananger status code: %v", statusCode)
		if statusCode == 200 {
			doesFit = true
			preemptIds = qm.getAppWrappers(quotaResponse.PreemptIds)
		}
	}
	return doesFit, preemptIds, ""
}


func  (qm *QuotaManager) getAppWrappers(preemptIds []string) []*arbv1.AppWrapper{
	var aws []*arbv1.AppWrapper
	if len(preemptIds) <= 0 {
		return nil
	}

	for _, preemptId := range preemptIds {
		awNamespace, awName := parseId(preemptId)
		if len(awNamespace) <= 0 || len(awName) <= 0 {
			klog.Errorf("[getAppWrappers] Failed to parse AppWrapper id from quota manager, parse string: %s.  Preemption of this Id will be ignored.", preemptId)
			continue
		}
		aw, e := qm.appwrapperLister.AppWrappers(awNamespace).Get(awName)
		if e != nil {
			klog.Errorf("[getAppWrappers] Failed to get AppWrapper from API Cache %s/%s, err=%v.  Preemption of this Id will be ignored.",
				awNamespace, awName, e)
			continue
		}
		aws = append(aws, aw)
	}

	// Final validation check
	if len(preemptIds) != len(aws) {
		klog.Warningf("[getAppWrappers] Preemption list size of %d from quota manager does not match size of generated list of AppWrapper: %d", len(preemptIds), len(aws))
	}
	return aws
}
func (qm *QuotaManager) Release(aw *arbv1.AppWrapper) bool {

	// Handle uninitialized quota manager
	if len(qm.url) <= 0 {
		return true
	}

	released := false
	awId := createId(aw.Namespace, aw.Name)
	if len(awId) <= 0 {
		klog.Errorf("[Release] Request failed due to invalid AppWrapper due to empty namespace: %s or name:%s.", aw.Namespace, aw.Name)
		return false
	}

	uri := qm.url + "/quota/release/" + awId
	klog.V(4).Infof("[Release] Sending request to release resources for: %s ", uri)

	// Create client
	client := &http.Client{}

	// Create request
	req, err := http.NewRequest("DELETE", uri, nil)
	if err != nil {
		klog.Errorf("[Release] Failed to create http delete request for : %s, err=%#v.", awId, err)
		return released
	}

	// Fetch Request
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("[Release] Failed http delete request for: %s, err=%#v.", awId, err)
		return released
	}
	defer resp.Body.Close()

	// Read Response Body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.V(4).Infof("[Release] Failed to aquire response from http delete request id: %s, err=%#v.", awId, err)
	} else {
		klog.V(4).Infof("[Release] Response from quota mananger body: %s", string(respBody))
	}

	klog.V(4).Infof("[Release] Response from quota mananger status: %s", resp.Status)
	statusCode := resp.StatusCode
	klog.V(4).Infof("[Release] Response from quota mananger status code: %v", statusCode)
	if statusCode == 204 {
		released = true
	}

	return released
}
