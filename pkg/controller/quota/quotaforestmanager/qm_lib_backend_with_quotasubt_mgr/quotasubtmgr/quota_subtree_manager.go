// b private
// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// Copyright 2019, 2021, 2022, 2023 The Multi-Cluster App Dispatcher Authors.
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

package quotasubtmgr

import (
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"k8s.io/klog/v2"

	qstv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/quotaplugins/quotasubtree/v1"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/quota/quotaforestmanager/qm_lib_backend_with_quotasubt_mgr/quotasubtmgr/util"
	qmlib "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota"
	qmlibutils "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	qst "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/clientset/versioned"
	"k8s.io/client-go/rest"

	qstinformers "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions"
	qstinformer "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/client/informers/externalversions/quotasubtree/v1"
)

// New returns a new implementation.
func NewQuotaSubtreeManager(config *rest.Config, quotaManagerBackend *qmlib.Manager) (*QuotaSubtreeManager, error) {
	return newQuotaSubtreeManager(config, quotaManagerBackend)
}

type QuotaSubtreeManager struct {
	quotaManagerBackend *qmlib.Manager

	/* Information about Quota Subtrees */
	quotaSubtreeInformer qstinformer.QuotaSubtreeInformer
	qstMutex             sync.RWMutex
	qstMap               map[string]*qstv1.QuotaSubtree

	qstChanged bool
	qstSynced  func() bool
}

func newQuotaSubtreeManager(config *rest.Config, quotaManagerBackend *qmlib.Manager) (*QuotaSubtreeManager, error) {
	qstm := &QuotaSubtreeManager{
		quotaManagerBackend: quotaManagerBackend,
		qstMap:              make(map[string]*qstv1.QuotaSubtree),
		qstChanged:          true,
	}
	// QuotaSubtree informer setup
	qstClient, err := qst.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	qstInformerFactory := qstinformers.NewSharedInformerFactoryWithOptions(qstClient, 0,
		qstinformers.WithTweakListOptions(func(opt *metav1.ListOptions) {
			opt.LabelSelector = util.URMTreeLabel
		}))
	qstm.quotaSubtreeInformer = qstInformerFactory.Ibm().V1().QuotaSubtrees()

	// Add event handle for resource plans
	qstm.quotaSubtreeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    qstm.addQST,
		UpdateFunc: qstm.updateQST,
		DeleteFunc: qstm.deleteQST,
	})

	// Start resource plan informers
	neverStop := make(chan struct{})
	klog.V(4).Infof("[newQuotaSubtreeManager] Starting QuotaSubtree Informer.")
	go qstm.quotaSubtreeInformer.Informer().Run(neverStop)

	// Wait for cache sync
	klog.V(4).Infof("[newQuotaSubtreeManager] Waiting for QuotaSubtree informer cache sync. to complete.")
	qstm.qstSynced = qstm.quotaSubtreeInformer.Informer().HasSynced
	if !cache.WaitForCacheSync(neverStop, qstm.qstSynced) {
		return nil, errors.New("failed to wait for the quota sub tree informer to synch")
	}

	// Initialize Quota Trees
	qstm.initializeQuotaTreeBackend()
	klog.V(4).Infof("[newQuotaSubtreeManager] QuotaSubtree Manager initialization complete.")
	return qstm, nil
}

func appendIfNotPresent(candidate string, source []string) []string {
	if source == nil {
		var retval []string
		return retval
	}

	if len(candidate) <= 0 {
		return source
	}

	for _, val := range source {
		if strings.Compare(val, candidate) == 0 {
			return source
		}
	}

	return append(source, candidate)
}

func (qstm *QuotaSubtreeManager) setQuotasubtreeChanged() {
	qstm.qstChanged = true
}

func (qstm *QuotaSubtreeManager) clearQuotasubtreeChanged() {
	qstm.qstChanged = false
}

func (qstm *QuotaSubtreeManager) IsQuotasubtreeChanged() bool {
	qstm.qstMutex.RLock()
	defer qstm.qstMutex.RUnlock()
	klog.V(4).Infof("[IsQuotasubtreeChanged] QuotaSubtree Manager changed %t.", qstm.qstChanged)
	return qstm.qstChanged
}

func (qstm *QuotaSubtreeManager) createTreeNodesFromQST(qst *qstv1.QuotaSubtree) (map[string]*qmlibutils.JNodeSpec, []string) {
	nodeSpecs := make(map[string]*qmlibutils.JNodeSpec)
	var resourceTypes []string

	for _, qstChild := range qst.Spec.Children {
		// Generate node key
		child_key := qstChild.Name

		// Get the quota demands
		quota := make(map[string]string)
		reqs := qstChild.Quotas.Requests
		for k, v := range reqs {
			resourceName := string(k)
			if len(resourceName) <= 0 {
				klog.Errorf("[createTreeNodesFromQST] Resource Name can not be empty, QuotaSubtree %s request quota: %v will be ignored.",
					qst.Name, v)
				continue
			}
			resourceTypes = appendIfNotPresent(resourceName, resourceTypes)
			var amount int64
			var success bool
			switch resourceName {
			case "cpu":
				amount = v.MilliValue()
			case "memory":
				amount = v.Value()
			default:
				amount, success = v.AsInt64()
				if !success {
					klog.Errorf("[createTreeNodesFromQST] Failure converting QuotaSubtree request demand quota to int64, QuotaSubtree %s request quota: %v will be ignored.",
						qst.Name, v)
					continue
				}
			}

			// Add new quota demand
			quota[resourceName] = strconv.FormatInt(amount, 10)
		}

		// Build a node
		node := &qmlibutils.JNodeSpec{
			Parent: qst.Spec.Parent,
			Quota:  quota,
			Hard:   strconv.FormatBool(qstChild.Quotas.HardLimit),
		}
		klog.V(4).Infof("[createTreeNodesFromQST] Created node: %s=%#v for QuotaSubtree  %s completed.",
			child_key, *node, qst.Name)

		//Add to the list of nodes from this quotasubtree
		nodeSpecs[child_key] = node
	}

	return nodeSpecs, resourceTypes
}

func (qstm *QuotaSubtreeManager) addQuotaSubtreesIntoBackend(qst *qstv1.QuotaSubtree, treeCache *core.TreeCache) {
	treeNodes, resourceTypes := qstm.createTreeNodesFromQST(qst)
	for childKey, nodeInfo := range treeNodes {
		treeCache.AddNodeSpec(childKey, *nodeInfo)
	}
	klog.V(4).Infof("[addQuotaSubtreesIntoBackend] Processing QuotaSubtree  %s completed.",
		qst.Name)

	// Add all the resource names found in the QuotaSubtree to the tree cache
	for _, resourceName := range resourceTypes {
		treeCache.AddResourceName(resourceName)
	}
}

func (qstm *QuotaSubtreeManager) createTreeCache(forestName string, qst *qstv1.QuotaSubtree) *core.TreeCache {
	// Create new tree in backend
	qstTreeName := qst.Labels[util.URMTreeLabel]
	_, err := qstm.quotaManagerBackend.AddTreeByName(qstTreeName)
	if err != nil {
		klog.Errorf("[createTreeCache] Failure adding tree name %s to quota tree backend err=%#v. QuotaSubtree %s will be ignored.",
			qstTreeName, err, qst.Name)
		return nil
	}

	// Add new tree to forest in backend
	err = qstm.quotaManagerBackend.AddTreeToForest(forestName, qstTreeName)

	if err != nil {
		klog.Errorf("[createTreeCache] Failure adding tree name %s to forest %s in quota tree backend failed err=%#v, QuotaSubtree %s will be ignored.",
			qstTreeName, forestName, err, qst.Name)
		return nil
	}

	// Add new tree to local cache
	return qstm.quotaManagerBackend.GetTreeCache(qstTreeName)
}

func (qstm *QuotaSubtreeManager) LoadQuotaSubtreesIntoBackend() {
	qstm.qstMutex.Lock()
	defer qstm.qstMutex.Unlock()

	// Get the list of forests names (should be only one for MCAD
	forestNames := qstm.quotaManagerBackend.GetForestNames()
	if len(forestNames) != 1 {
		klog.Errorf("[LoadQuotaSubtreesIntoBackend] QuotaSubtree initialization requires only one forest to be defined in quota tree backend, found %v defined.", len(forestNames))
		return
	}

	forestName := forestNames[0]

	// Get the list of trees names in the forest
	treeNames := qstm.quotaManagerBackend.GetTreeNames()

	// Function cache map of Tree Name to Tree Cache
	treeNameToTreeCache := make(map[string]*core.TreeCache)
	for _, treeName := range treeNames {
		treeNameToTreeCache[treeName] = qstm.quotaManagerBackend.GetTreeCache(treeName)
	}

	// Process all quotasubtrees to the tree caches
	for _, qst := range qstm.qstMap {
		klog.V(4).Infof("[LoadQuotaSubtreesIntoBackend] Processing QuotaSubtree  %s.",
			qst.Name)
		qstTreeName := qst.Labels[util.URMTreeLabel]
		if len(qstTreeName) <= 0 {
			klog.Errorf("[LoadQuotaSubtreesIntoBackend] QuotaSubtree %s does not contain the proper 'tree' label will be ignored.",
				qst.Name)
			continue
		}

		treeCache := treeNameToTreeCache[qstTreeName]

		// Handle new tree
		if treeCache == nil {
			// Add new tree to function cache
			treeNameToTreeCache[qstTreeName] = qstm.createTreeCache(forestName, qst)

			// Validate cache exists in backend
			treeCache = treeNameToTreeCache[qstTreeName]
			if treeCache == nil {
				klog.Errorf("[LoadQuotaSubtreesIntoBackend] Tree cache not found for tree: %s. QuotaSubtree %s will be ignored.",
					qstTreeName, qst.Name)
				continue
			}
			treeNames = append(treeNames, qstTreeName)
		}

		// Add quotasubtree to quota tree backend
		qstm.addQuotaSubtreesIntoBackend(qst, treeCache)
	}

	for _, treeName := range treeNames {
		klog.V(10).Infof("[LoadQuotaSubtreesIntoBackend] Processing Quota Manager Backend tree %s completed.", treeName)
	}

	qstm.clearQuotasubtreeChanged()

}

func (qstm *QuotaSubtreeManager) initializeQuotaTreeBackend() {
	if !qstm.IsQuotasubtreeChanged() {
		klog.V(4).Infof("[initializeQuotaTreeBackend] No QuotaSubtrees to process.")
		return
	}

	if qstm.quotaManagerBackend.GetMode() != qmlib.Maintenance {
		klog.Warningf("[initializeQuotaTreeBackend] Forcing Quota Manager into maintenance mode.")
		qstm.quotaManagerBackend.SetMode(qmlib.Maintenance)
	}

	qstm.LoadQuotaSubtreesIntoBackend()
}
