/*
Copyright 2022, 2023 The Multi-Cluster App Dispatcher Authors.

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

package quota

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	"k8s.io/klog/v2"
)

// Manager : A quota manager of quota trees;
// This should be the main interface to quota management
type Manager struct {
	// all managed agents: treeName -> agent(treeController, treeCache)
	agents map[string]*agent
	// all forests: forestName -> forestController
	forests map[string]*core.ForestController
	// mapping tree name to forest name: treeName -> forestName
	treeToForest map[string]string
	// all consumer information: consumerID -> ConsumerInfo
	consumerInfos map[string]*ConsumerInfo
	// mode of the quota mnager; initially in maintenance mode
	mode Mode

	// mutex to
	mutex sync.RWMutex
}

// agent : a pair of controller and its tree cache
type agent struct {
	// the tree controller; maintains a tree instance
	controller *core.Controller
	// the tree cache from which a tree instance is derived
	cache *core.TreeCache
}

// Mode : the mode of a quota tree
type Mode int

const (
	// Normal operation: consumers allocated/deallocated; tree updates in cache
	Normal = iota
	// Maintenance operation: consumers forced allocated; tree updates in cache
	Maintenance
)

// NewManager : create a new quota manager
func NewManager() *Manager {
	return &Manager{
		agents:        make(map[string]*agent),
		forests:       make(map[string]*core.ForestController),
		treeToForest:  make(map[string]string),
		consumerInfos: map[string]*ConsumerInfo{},
		mode:          Maintenance,
	}
}

// newAgent : create a new treeAgent(treeController, treeCache)
func newAgentByName(treeName string) (*agent, error) {
	treeCache := core.NewTreeCache()
	treeCache.SetTreeName(treeName)
	return newAgentFromCache(treeCache)
}

// newAgentFromCache : create a new tree agent from tree cache
func newAgentFromCache(treeCache *core.TreeCache) (*agent, error) {
	if treeName := treeCache.GetTreeName(); len(treeName) == 0 {
		return nil, fmt.Errorf("invalid tree name %v", treeName)
	}
	tree, response := treeCache.CreateTree()
	if !response.IsClean() {
		klog.Warningf("Warning: cache not clean after tree created: %v \n", response.String())
	}
	return &agent{
		controller: core.NewController(tree),
		cache:      treeCache,
	}, nil
}

// GetMode : get the mode of the quota manager
func (m *Manager) GetMode() Mode {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.mode
}

// SetMode : set the mode of the quota manager
func (m *Manager) SetMode(mode Mode) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.mode = mode
	// in the future we may restrict some mode transitions
	return true
}

// GetMode : get the mode of the quota manager
func (m *Manager) GetModeString() string {
	modeString := "mode set to "
	switch m.mode {
	case Normal:
		modeString += "Normal"
	case Maintenance:
		modeString += "Maintenance"
	default:
		modeString += "Unknown"
	}
	return modeString
}

// AddTreeByName : add an empty tree with a given name
func (m *Manager) AddTreeByName(treeName string) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, exists := m.agents[treeName]; exists {
		return treeName, fmt.Errorf("tree %v already exists; delete first", treeName)
	}
	agent, err := newAgentByName(treeName)
	if err != nil {
		return treeName, err
	}
	m.agents[treeName] = agent
	return treeName, nil
}

// AddTreeFromString : add a quota tree from the string JSON representation of the tree
func (m *Manager) AddTreeFromString(treeSring string) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	treeCache := core.NewTreeCache()
	if err := treeCache.FromString(treeSring); err != nil {
		return "", err
	}
	return m.addTree(treeCache)
}

// AddTreeFromStruct : add a quota tree from the JSON struct of the tree
func (m *Manager) AddTreeFromStruct(jQuotaTree utils.JQuotaTree) (string, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	treeCache := core.NewTreeCache()
	if err := treeCache.FromStruct(jQuotaTree); err != nil {
		return "", err
	}
	return m.addTree(treeCache)
}

// addTree : add a tree from tree cache; returns name of tree added
func (m *Manager) addTree(treeCache *core.TreeCache) (string, error) {
	treeName := treeCache.GetTreeName()
	if _, exists := m.agents[treeName]; exists {
		return treeName, fmt.Errorf("tree %v already exists; delete first", treeName)
	}
	agent, err := newAgentFromCache(treeCache)
	if err != nil {
		return treeName, err
	}
	m.agents[treeName] = agent
	return treeName, nil
}

// DeleteTree : delete a quota tree
func (m *Manager) DeleteTree(treeName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	agent := m.agents[treeName]
	if agent == nil {
		return fmt.Errorf("tree %v does not exist", treeName)
	}
	if agent.controller.IsAllocated() {
		return fmt.Errorf("consumer(s) allocated in tree %v", treeName)
	}
	delete(m.agents, treeName)
	return nil
}

// GetTreeCache : get the tree cache corresponding to a tree; nil if does not exist
func (m *Manager) GetTreeCache(treeName string) *core.TreeCache {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if agent, exists := m.agents[treeName]; exists {
		return agent.cache
	}
	return nil
}

// GetTreeNames : get the names of all the trees
func (m *Manager) GetTreeNames() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	names := make([]string, len(m.agents))
	i := 0
	for name := range m.agents {
		names[i] = name
		i++
	}
	return names
}

// UpdateTree : update tree from cache
func (m *Manager) UpdateTree(treeName string) (unallocatedConsumerIDs []string, response *core.TreeCacheCreateResponse, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if agent, exists := m.agents[treeName]; exists {
		unallocatedConsumerIDs, response = agent.controller.UpdateTree(agent.cache)
		return unallocatedConsumerIDs, response, nil
	}
	return nil, nil, fmt.Errorf("tree %v does not exist", treeName)
}

// AddConsumer : add a consumer info
func (m *Manager) AddConsumer(consumerInfo *ConsumerInfo) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	consumerID := consumerInfo.GetID()
	if _, exists := m.consumerInfos[consumerID]; exists {
		return false, fmt.Errorf("consumer %s already exists", consumerID)
	}
	m.consumerInfos[consumerID] = consumerInfo
	return true, nil
}

// RemoveConsumer : remove a consumer info, does not de-allocate any currently allocated consumers
func (m *Manager) RemoveConsumer(consumerID string) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, exists := m.consumerInfos[consumerID]; !exists {
		return false, fmt.Errorf("consumer %s does not exist", consumerID)
	}
	delete(m.consumerInfos, consumerID)
	return true, nil
}

// GetAllConsumerIDs : get IDs of all consumers added
func (m *Manager) GetAllConsumerIDs() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	allIDs := make([]string, len(m.consumerInfos))
	i := 0
	for id := range m.consumerInfos {
		allIDs[i] = id
		i++
	}
	return allIDs
}

// Allocate : allocate a consumer on a tree
func (m *Manager) Allocate(treeName string, consumerID string) (response *core.AllocationResponse, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	agent, consumer, err := m.preAllocate(treeName, consumerID)
	if err == nil && agent.controller.IsConsumerAllocated(consumerID) {
		err = fmt.Errorf("consumer %s already allocated on tree %s", consumerID, treeName)
	}
	if err != nil {
		return nil, err
	}
	if m.mode == Normal {
		response = agent.controller.Allocate(consumer)
	} else {
		response = agent.controller.ForceAllocate(consumer, consumer.GetGroupID())
	}
	if !response.IsAllocated() {
		return nil, fmt.Errorf(response.GetMessage())
	}
	return response, err
}

// TryAllocate : try allocating a consumer on a tree
func (m *Manager) TryAllocate(treeName string, consumerID string) (response *core.AllocationResponse, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.mode != Normal {
		return nil, fmt.Errorf("manager is not in normal mode")
	}
	agent, consumer, err := m.preAllocate(treeName, consumerID)
	if err == nil && agent.controller.IsConsumerAllocated(consumerID) {
		err = fmt.Errorf("consumer %s already allocated on tree %s", consumerID, treeName)
	}
	if err != nil {
		return nil, err
	}
	if response = agent.controller.TryAllocate(consumer); !response.IsAllocated() {
		return nil, fmt.Errorf(response.GetMessage())
	}
	return response, err
}

// UndoAllocate : undo the most recent allocation trial on a tree
func (m *Manager) UndoAllocate(treeName string, consumerID string) (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	agent, consumer, err := m.preAllocate(treeName, consumerID)
	if err != nil {
		return err
	}
	if !agent.controller.UndoAllocate(consumer) {
		return fmt.Errorf("failed undo allocate tree name %s", treeName)
	}
	return nil
}

// preAllocate : prepare for allocate
func (m *Manager) preAllocate(treeName string, consumerID string) (agent *agent, consumer *core.Consumer, err error) {
	agent = m.agents[treeName]
	if agent == nil {
		return nil, nil, fmt.Errorf("invalid tree name %s", treeName)
	}
	consumerInfo := m.consumerInfos[consumerID]
	if consumerInfo == nil {
		return nil, nil, fmt.Errorf("consumer %s does not exist, create and add first", consumerID)
	}
	resourceNames := agent.cache.GetResourceNames()
	consumer, err = consumerInfo.CreateTreeConsumer(treeName, resourceNames)
	if err != nil {
		return nil, nil, fmt.Errorf("failure creating consumer %s on tree %s", consumerID, treeName)
	}
	klog.V(4).Infoln(consumer)
	return agent, consumer, nil
}

// IsAllocated : check if a consumer is allocated on a tree
func (m *Manager) IsAllocated(treeName string, consumerID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if agent, exists := m.agents[treeName]; exists {
		return agent.controller.IsConsumerAllocated(consumerID)
	}
	return false
}

// DeAllocate : de-allocate a consumer from a tree
func (m *Manager) DeAllocate(treeName string, consumerID string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if agent, exists := m.agents[treeName]; exists {
		return agent.controller.DeAllocate(consumerID)
	}
	return false
}

// AddForest : add a new forest
func (m *Manager) AddForest(forestName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.forests[forestName]; exists {
		return fmt.Errorf("duplicate forest name %v", forestName)
	}
	m.forests[forestName] = core.NewForestController()
	return nil
}

// DeleteForest : delete a forest
func (m *Manager) DeleteForest(forestName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if forestController, exists := m.forests[forestName]; exists {
		treeNames := forestController.GetTreeNames()
		for i := 0; i < len(treeNames); i++ {
			delete(m.treeToForest, treeNames[i])
		}
		delete(m.forests, forestName)
		return nil
	}
	return fmt.Errorf("forest %v does not exist", forestName)
}

// AddTreeToForest : add an already defined tree to a forest
func (m *Manager) AddTreeToForest(forestName string, treeName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if forestController, exists := m.forests[forestName]; exists {
		if agent, exists := m.agents[treeName]; exists {
			forestController.AddController(agent.controller)
			m.treeToForest[treeName] = forestName
			return nil
		}
		return fmt.Errorf("unknown tree name %v", treeName)
	}
	return fmt.Errorf("unknown forest name %v", forestName)
}

// DeleteTreeFromForest : delete a tree from a forest
func (m *Manager) DeleteTreeFromForest(forestName string, treeName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if len(forestName) == 0 || m.treeToForest[treeName] != forestName {
		return fmt.Errorf("forest %v does not include tree %v", forestName, treeName)
	}
	delete(m.treeToForest, treeName)
	m.forests[forestName].DeleteController(treeName)
	return nil
}

// GetForestNames : get the names of all forests
func (m *Manager) GetForestNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, len(m.forests))
	i := 0
	for name := range m.forests {
		names[i] = name
		i++
	}
	return names
}

// GetForestTreeNames : get the tree names for all forests
func (m *Manager) GetForestTreeNames() map[string][]string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	forestTreeNames := make(map[string][]string)
	for forestName, controller := range m.forests {
		forestTreeNames[forestName] = controller.GetTreeNames()
	}
	return forestTreeNames
}

// UpdateForest : update forest from cache
func (m *Manager) UpdateForest(forestName string) (unallocatedConsumerIDs []string,
	response map[string]*core.TreeCacheCreateResponse, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if forestController, exists := m.forests[forestName]; exists {
		treeNames := forestController.GetTreeNames()
		treeCaches := make([]*core.TreeCache, len(treeNames))
		for i := 0; i < len(treeNames); i++ {
			treeCaches[i] = m.agents[treeNames[i]].cache
		}
		unallocatedConsumerIDs, response = forestController.UpdateTrees(treeCaches)
		return unallocatedConsumerIDs, response, nil
	}
	return nil, nil, fmt.Errorf("forest %v does not exist", forestName)
}

// AllocateForest : allocate a consumer on a forest
func (m *Manager) AllocateForest(forestName string, consumerID string) (response *core.AllocationResponse, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	forestController, forestConsumer, err := m.preAllocateForest(forestName, consumerID)
	if err == nil && forestController.IsConsumerAllocated(consumerID) {
		err = fmt.Errorf("consumer %s already allocated on forest %s", consumerID, forestName)
	}
	if err != nil {
		return nil, err
	}

	if m.mode == Normal {
		response = forestController.Allocate(forestConsumer)
	} else {
		groupIDs := make(map[string]string)
		for treeName, consumer := range forestConsumer.GetConsumers() {
			groupIDs[treeName] = consumer.GetGroupID()
		}
		response = forestController.ForceAllocate(forestConsumer, groupIDs)
	}
	if !response.IsAllocated() {
		return nil, fmt.Errorf(response.GetMessage())
	}
	return response, nil
}

// TryAllocateForest : allocate a consumer on a forest
func (m *Manager) TryAllocateForest(forestName string, consumerID string) (response *core.AllocationResponse, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.mode != Normal {
		return nil, fmt.Errorf("manager is not in normal mode")
	}
	forestController, forestConsumer, err := m.preAllocateForest(forestName, consumerID)
	if err == nil && forestController.IsConsumerAllocated(consumerID) {
		err = fmt.Errorf("consumer %s already allocated on forest %s", consumerID, forestName)
	}
	if err != nil {
		return nil, err
	}

	response = forestController.TryAllocate(forestConsumer)
	if !response.IsAllocated() {
		return nil, fmt.Errorf(response.GetMessage())
	}
	return response, nil
}

// UndoAllocate : undo the most recent allocation trial on a tree
func (m *Manager) UndoAllocateForest(forestName string, consumerID string) (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	forestController, forestConsumer, err := m.preAllocateForest(forestName, consumerID)
	if err != nil {
		return err
	}
	if !forestController.UndoAllocate(forestConsumer) {
		return fmt.Errorf("failed undo allocate forest name %s", forestName)
	}
	return nil
}

// preAllocateForest : prepare for allocate forest
func (m *Manager) preAllocateForest(forestName string, consumerID string) (forestController *core.ForestController,
	forestConsumer *core.ForestConsumer, err error) {
	forestController = m.forests[forestName]
	if forestController == nil {
		return nil, nil, fmt.Errorf("invalid forest name %s", forestName)
	}
	consumerInfo := m.consumerInfos[consumerID]
	if consumerInfo == nil {
		return nil, nil, fmt.Errorf("consumer %s does not exist, create and add first", consumerID)
	}
	resourceNames := forestController.GetResourceNames()
	forestConsumer, err = consumerInfo.CreateForestConsumer(forestName, resourceNames)
	if err != nil {
		return nil, nil, fmt.Errorf("failure creating forest consumer %s in forest %s", consumerID, forestName)
	}
	return forestController, forestConsumer, nil
}

// IsAllocatedForest : check if a consumer is allocated on a forest
func (m *Manager) IsAllocatedForest(forestName string, consumerID string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if forestController, exists := m.forests[forestName]; exists {
		return forestController.IsConsumerAllocated(consumerID)
	}
	return false
}

// DeAllocateForest : de-allocate a consumer from a forest
func (m *Manager) DeAllocateForest(forestName string, consumerID string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if forestController, exists := m.forests[forestName]; exists {
		return forestController.DeAllocate(consumerID)
	}
	return false
}

// UpdateAll : update all trees and forests from caches
func (m *Manager) UpdateAll() (treeUnallocatedConsumerIDs map[string][]string,
	treeResponse map[string]*core.TreeCacheCreateResponse,
	forestUnallocatedConsumerIDs map[string][]string,
	forestResponse map[string]map[string]*core.TreeCacheCreateResponse,
	err error) {

	m.mutex.Lock()
	defer m.mutex.Unlock()

	treeUnallocatedConsumerIDs = make(map[string][]string)
	treeResponse = make(map[string]*core.TreeCacheCreateResponse)
	forestUnallocatedConsumerIDs = make(map[string][]string)
	forestResponse = make(map[string]map[string]*core.TreeCacheCreateResponse)
	var erStr []string

	// update all single trees
	for treeName := range m.agents {
		if _, exists := m.treeToForest[treeName]; !exists {
			ids, resp, er := m.UpdateTree(treeName)
			treeUnallocatedConsumerIDs[treeName] = ids
			treeResponse[treeName] = resp
			if er != nil {
				erStr = append(erStr, er.Error())
			}
		}
	}
	// update all forests
	for forestName := range m.forests {
		ids, resp, er := m.UpdateForest(forestName)
		forestUnallocatedConsumerIDs[forestName] = ids
		forestResponse[forestName] = resp
		if er != nil {
			erStr = append(erStr, er.Error())
		}
	}

	if len(erStr) > 0 {
		err = fmt.Errorf(strings.Join(erStr, "\n"))
	}
	return treeUnallocatedConsumerIDs, treeResponse, forestUnallocatedConsumerIDs, forestResponse, err
}

// GetTreeController : get the tree controller for a given tree
func (m *Manager) GetTreeController(treeName string) *core.Controller {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if agent, exists := m.agents[treeName]; exists {
		return agent.controller
	}
	return nil
}

// GetForestController : get the forest controller for a given forest
func (m *Manager) GetForestController(forestName string) *core.ForestController {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.forests[forestName]
}

// String : printout
func (m *Manager) String() string {

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var b bytes.Buffer
	b.WriteString("QuotaManager: \n")
	b.WriteString("Mode: " + m.GetModeString() + "\n")
	b.WriteString("\n")

	b.WriteString("TreeControllers: \n")
	for _, agent := range m.agents {
		b.WriteString(agent.controller.String())
	}
	b.WriteString("\n")

	b.WriteString("ForestControllers: \n")
	for _, controller := range m.forests {
		b.WriteString(controller.String())
	}

	b.WriteString("Consumers: \n")
	b.WriteString("[ ")
	for _, consumerInfo := range m.consumerInfos {
		b.WriteString(consumerInfo.GetID() + " ")
	}
	b.WriteString("]")

	return b.String()
}
