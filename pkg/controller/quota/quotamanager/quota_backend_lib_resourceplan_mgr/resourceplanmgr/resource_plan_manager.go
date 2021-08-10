// +build private

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

package resourceplanmgr

import (
	"github.ibm.com/ai-foundation/quota-manager/quota/core"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
	"sync"

	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/quota/quotamanager/quota_backend_lib_resourceplan_mgr/resourceplanmgr/util"
	qmlib "github.ibm.com/ai-foundation/quota-manager/quota"
	qmlibutils "github.ibm.com/ai-foundation/quota-manager/quota/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	rpv1 "sigs.k8s.io/scheduler-plugins/pkg/apis/resourceplan/v1"

	"k8s.io/client-go/rest"
	rp "sigs.k8s.io/scheduler-plugins/pkg/client/resourceplan/clientset/versioned"

	rpinformer "sigs.k8s.io/scheduler-plugins/pkg/client/resourceplan/informers/externalversions"
	rpinformers "sigs.k8s.io/scheduler-plugins/pkg/client/resourceplan/informers/externalversions/resourceplan/v1"
)


// New returns a new implementation.
func NewResourcePlanManager(config *rest.Config, quotaManagerBackend *qmlib.Manager) (*ResourcePlanManager, error) {
	return newResourcePlanManager(config, quotaManagerBackend)
}

type ResourcePlanManager struct {
	mutex sync.Mutex

	kubeclient *kubernetes.Clientset


	beMutex                sync.Mutex
	quotaManagerBackend    *qmlib.Manager

	/* Information about Resource Plans */
	resourcePlanInformer rpinformers.ResourcePlanInformer
	rpMutex              sync.Mutex
	rpMap                map[string]*rpv1.ResourcePlan

	rpChanged bool
	rpSynced func() bool
}

func newResourcePlanManager(config *rest.Config, quotaManagerBackend *qmlib.Manager) (*ResourcePlanManager, error) {
	rpm := &ResourcePlanManager{
		quotaManagerBackend: quotaManagerBackend,
		rpMap:    make(map[string]*rpv1.ResourcePlan),
	}
	// ResourcePlan informer setup
	rpClient := rp.NewForConfigOrDie(config)

	rpInformerFactory := rpinformer.NewSharedInformerFactoryWithOptions(rpClient, 0,
							rpinformer.WithTweakListOptions(func(opt *metav1.ListOptions) {
		opt.LabelSelector = util.URMTreeLabel
	}))
	rpm.resourcePlanInformer = rpInformerFactory.Resourceplan().V1().ResourcePlans()


	// Add event handle for resource plans
	rpm.resourcePlanInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rpm.addRP,
		UpdateFunc: rpm.updateRP,
		DeleteFunc: rpm.deleteRP,
	})

	// Start resource plan informers
	neverStop := make(chan struct{})
	klog.V(10).Infof("[newResourcePlanManager] Starting ResourcePlan Informer.")
	go rpm.resourcePlanInformer.Informer().Run(neverStop)

	// Wait for cache sync
	klog.V(10).Infof("[newResourcePlanManager] Waiting for ResourcePlan informer cache sync. to complete.")
	rpm.rpSynced = rpm.resourcePlanInformer.Informer().HasSynced
	cache.WaitForCacheSync(neverStop, rpm.rpSynced)

	// Initialize Quota Trees
	rpm.initializeQuotaTreeBackend()
	klog.V(10).Infof("[newResourcePlanManager] ResourcePlan Manager initialization complete.")
	return rpm, nil
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

func (rpm *ResourcePlanManager) setResplanChanged() {
	rpm.rpChanged = true
}

func (rpm *ResourcePlanManager) clearResplanChanged() {
	rpm.rpChanged = false
}

func (rpm *ResourcePlanManager) IsResplanChanged() bool {
	return rpm.rpChanged
}



func (rpm *ResourcePlanManager) createTreeNodesFromRP(rp *rpv1.ResourcePlan) (map[string]*qmlibutils.JNodeSpec, []string) {
	nodeSpecs := make(map[string]*qmlibutils.JNodeSpec)
	var resourceTypes []string

	for _, rpChild := range rp.Spec.Children {
		// Generate node key
		child_key := rpChild.Name

		// Get the quota demands
		quota := make(map[string]string)
		reqs := rpChild.RunPodQuotas.Requests
		for k, v := range reqs {
			resourceName := string(k)
			if len(resourceName) <= 0 {
				klog.Errorf("[createTreeNodesFromRP] Resource Name can not be empty, ResourcePlan %s request quota: %v will be ignored.",
					rp.Name, v)
				continue
			}
			resourceTypes = appendIfNotPresent(resourceName, resourceTypes)
			amount, success := v.AsInt64()
			if !success {
				klog.Errorf("[createTreeNodesFromRP] Failure converting ResourcePlan request demand quota to int64, ResourcePlan %s request quota: %v will be ignored.",
					rp.Name, v)
				continue
			}

			// Add new quota demand
			quota[resourceName] = strconv.FormatInt(amount, 10)
		}

		// Build a node
		node := &qmlibutils.JNodeSpec{
			Parent: rp.Spec.Parent,
			Quota:  quota,
			Hard:   strconv.FormatBool(rpChild.RunPodQuotas.HardLimit),
		}
		klog.V(10).Infof("[createTreeNodesFromRP] Created node: %s=%#v for ResourcePlan  %s completed.",
			child_key, *node, rp.Name)

		//Add to the list of nodes from this resourceplan
		nodeSpecs[child_key] = node
	}

	return nodeSpecs, resourceTypes
}

func (rpm *ResourcePlanManager) addResourcePlansIntoBackend(rp *rpv1.ResourcePlan, treeCache *core.TreeCache) {
	treeNodes, resourceTypes := rpm.createTreeNodesFromRP(rp)
	for childKey, nodeInfo := range treeNodes {
		treeCache.AddNodeSpec(childKey, *nodeInfo)
	}
	klog.V(4).Infof("[addResourcePlansIntoBackend] Processing ResourcePlan  %s completed.",
		rp.Name)

	// Add all the resource names found in the ResourcePlan to the tree cache
	for _, resourceName := range resourceTypes {
		treeCache.AddResourceName(resourceName)
	}
}

func (rpm *ResourcePlanManager) createTreeCache(forestName string, rp *rpv1.ResourcePlan) *core.TreeCache {
	// Create new tree in backend
	rpTreeName := rp.Labels[util.URMTreeLabel]
	_, err := rpm.quotaManagerBackend.AddTreeByName(rpTreeName)
	if err != nil {
		klog.Errorf("[LoadResourcePlansIntoBackend] Failure adding tree name %s to quota tree backend err=%#v. ResourcePlan %s will be ignored.",
			rpTreeName, err, rp.Name)
		return nil
	}

	// Add new tree to forest in backend
	err = rpm.quotaManagerBackend.AddTreeToForest(forestName, rpTreeName)

	if err != nil {
		klog.Errorf("[LoadResourcePlansIntoBackend] Failure adding tree name %s to forest %s in quota tree backend failed err=%#v, ResourcePlan %s will be ignored.",
			rpTreeName, forestName, err, rp.Name)
		return nil
	}

	// Add new tree to local cache
	return rpm.quotaManagerBackend.GetTreeCache(rpTreeName)
}

func (rpm *ResourcePlanManager) LoadResourcePlansIntoBackend() {

	rpm.rpMutex.Lock()
	defer rpm.rpMutex.Unlock()

	// Get the list of forests names (should be only one for MCAD
	forestNames := rpm.quotaManagerBackend.GetForestNames()
	if len(forestNames) != 1 {
		klog.Errorf("[LoadResourcePlansIntoBackend] ResourcePlan initialization requires only one forest to be defined in quota tree backend, found %v defined.", len(forestNames))
		return
	}

	forestName := forestNames[0]

	// Get the list of trees names in the forest
	treeNames := rpm.quotaManagerBackend.GetTreeNames()

	// Function cache map of Tree Name to Tree Cache
	treeNameToTreeCache := make(map[string]*core.TreeCache)
	for _, treeName := range treeNames {
		treeNameToTreeCache[treeName] = rpm.quotaManagerBackend.GetTreeCache(treeName)
	}

	// Process all resourceplans to the tree caches
	for _, rp := range rpm.rpMap {
		klog.V(4).Infof("[LoadResourcePlansIntoBackend] Processing ResourcePlan  %s.",
											rp.Name)
		rpTreeName := rp.Labels[util.URMTreeLabel]
		if len(rpTreeName) <= 0 {
			klog.Errorf("[LoadResourcePlansIntoBackend] ResourcePlan %s does not contain the proper 'tree' label will be ignored.",
											rp.Name)
			continue
		}

		treeCache := treeNameToTreeCache[rpTreeName]

		// Handle new tree
		if treeCache == nil {
			// Add new tree to function cache
			treeNameToTreeCache[rpTreeName] = rpm.createTreeCache(forestName, rp)

			// Validate cache exists in backend
			treeCache = treeNameToTreeCache[rpTreeName]
			if treeCache == nil {
				klog.Errorf("[LoadResourcePlansIntoBackend] Tree cache not found for tree: %s. ResourcePlan %s will be ignored.",
					rpTreeName, rp.Name)
				continue
			}
			treeNames = append(treeNames, rpTreeName)
		}

		// Add resourceplan to quota tree backend
		rpm.addResourcePlansIntoBackend(rp, treeCache)
	}

	for _, treeName := range treeNames {
		klog.V(10).Infof("[LoadResourcePlansIntoBackend] Processing Quota Manager Backend tree %s completed.", treeName)
	}

	rpm.clearResplanChanged()

}

func (rpm *ResourcePlanManager) initializeQuotaTreeBackend() {
	if !rpm.IsResplanChanged() {
		klog.V(4).Infof("[initializeQuotaTreeBackend] No ResourcePlans to process.")
		return
	}

	if rpm.quotaManagerBackend.GetMode() !=  qmlib.Maintenance {
		klog.Warningf("[initializeQuotaTreeBackend] Forcing Quota Manager into maintenance mode.")
		rpm.quotaManagerBackend.SetMode(qmlib.Maintenance)
	}

	rpm.LoadResourcePlansIntoBackend()
}