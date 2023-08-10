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
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"

	utils "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	"k8s.io/klog/v2"
)

// TreeCache : cache area for the parts of the quota tree (resources, nodes, topology)
// which may be updated; a quota tree may be created from the cache at any point
type TreeCache struct {
	// the unique name of the tree
	treeName string
	// map of resource names: resourceName -> bool
	resourceNamesMap map[string]bool
	// map of node specs: nodeName -> nodeSpec
	nodeSpecMap map[string]utils.JNodeSpec

	// map of renamed nodes: oldName -> newName
	renamedNodesMap map[string]string
}

// NewTreeCache : create a tree cache
func NewTreeCache() *TreeCache {
	tc := &TreeCache{}
	tc.Clear()
	return tc
}

// SetTreeName : set the name of the tree; overrides earlier name
func (tc *TreeCache) SetTreeName(name string) {
	tc.treeName = name
}

// GetTreeName : get the name of the tree
func (tc *TreeCache) GetTreeName() string {
	return tc.treeName
}

// SetDefaultTreeName : set the name of the tree to the default name
func (tc *TreeCache) SetDefaultTreeName() {
	tc.SetTreeName(utils.DefaultTreeName)
}

// clearTreeName : clear the name of the tree
func (tc *TreeCache) clearTreeName() {
	tc.treeName = ""
}

// AddResourceName : add a resource name; overrides earlier name
func (tc *TreeCache) AddResourceName(name string) {
	if len(name) != 0 {
		tc.resourceNamesMap[name] = true
	}
}

// AddResourceNames : add a list of resource names
func (tc *TreeCache) AddResourceNames(names []string) {
	for _, name := range names {
		tc.AddResourceName(name)
	}
}

// DeleteResourceName : delete a resource name
func (tc *TreeCache) DeleteResourceName(name string) {
	delete(tc.resourceNamesMap, name)
}

// GetResourceNames : get a sorted list of resource names
func (tc *TreeCache) GetResourceNames() []string {
	names := make([]string, 0, len(tc.resourceNamesMap))
	for name := range tc.resourceNamesMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetNumResourceNames : the number of resource names
func (tc *TreeCache) GetNumResourceNames() int {
	return len(tc.resourceNamesMap)
}

// clearResourceNames : delete all resource names
func (tc *TreeCache) clearResourceNames() {
	tc.resourceNamesMap = make(map[string]bool)
}

// SetDefaultResourceNames : set resource names to the default ones
func (tc *TreeCache) SetDefaultResourceNames() {
	for _, name := range utils.DefaultResourceNames {
		tc.resourceNamesMap[name] = true
	}
}

// AddTreeInfoFromString : add tree name and resource names by parsing tree information string;
/* { "name": "ExampleTree", "resourceNames": [ "cpu", "memory" ] } */
func (tc *TreeCache) AddTreeInfoFromString(treeInfo string) error {
	var jTreeInfo utils.JTreeInfo
	err := json.Unmarshal([]byte(treeInfo), &jTreeInfo)
	if err != nil {
		return err
	}
	tc.SetTreeName(jTreeInfo.Name)
	tc.AddResourceNames(jTreeInfo.ResourceNames)
	return nil
}

// AddNodeSpec : add a node spec to the cache; overrides earlier node spec with same name
/* {"parent": "Org-A", "hard": "true", "quota": { "cpu": "1" } } */
func (tc *TreeCache) AddNodeSpec(nodeName string, nodeSpec utils.JNodeSpec) error {
	if reflect.DeepEqual(nodeSpec, utils.JNodeSpec{}) {
		return fmt.Errorf("node " + nodeName + ": node spec is invalid; not added")
	}
	if nodeSpec.Quota == nil {
		return fmt.Errorf("node " + nodeName + ": quota is missing; not added")
	}
	tc.removeRenamedNode(nodeName)
	tc.nodeSpecMap[nodeName] = nodeSpec
	return nil
}

// AddNodeSpecs : add a map of node specs to the cache; invalid node specs are not added
func (tc *TreeCache) AddNodeSpecs(nodeSpecs map[string]utils.JNodeSpec) error {
	errString := ""
	for name, spec := range nodeSpecs {
		errNode := tc.AddNodeSpec(name, spec)
		if errNode != nil {
			errString += errNode.Error()
		}
	}
	if len(errString) > 0 {
		return fmt.Errorf(errString)
	}
	return nil
}

// AddNodeSpecsFromString : add node specs by parsing string with one or more node information
/* { "Root": { "parent": "nil", "quota": { "cpu": "10" } }, "Org-A": {"parent": "Root", "quota": { "cpu": "4" } } } */
/* { "Context-1": {"parent": "Org-A", "hard": "true", "quota": { "cpu": "1" } } } */
func (tc *TreeCache) AddNodeSpecsFromString(nodesInfo string) error {
	jNodes := make(map[string]utils.JNodeSpec)
	err := json.Unmarshal([]byte(nodesInfo), &jNodes)
	if err != nil {
		return err
	}
	return tc.AddNodeSpecs(jNodes)
}

// clearNodeSpecs : clear all node specs
func (tc *TreeCache) clearNodeSpecs() {
	tc.nodeSpecMap = make(map[string]utils.JNodeSpec)
}

// RenameNode : rename a node in the cache
func (tc *TreeCache) RenameNode(nodeName string, newNodeName string) error {
	if len(nodeName) == 0 || len(newNodeName) == 0 || nodeName == newNodeName {
		return fmt.Errorf("invalid nodeName %s and/or newNodeName %s", nodeName, newNodeName)
	}
	var nodeSpec utils.JNodeSpec
	var exists bool
	if nodeSpec, exists = tc.nodeSpecMap[nodeName]; !exists {
		return fmt.Errorf("node with name %s does not exist", nodeName)
	}
	delete(tc.nodeSpecMap, nodeName)
	tc.nodeSpecMap[newNodeName] = nodeSpec

	// modify nodes which have the renamed node as parent
	nodeNames := tc.GetNodeNames()
	for _, n := range nodeNames {
		spec := tc.nodeSpecMap[n]
		if spec.Parent == nodeName {
			spec.Parent = newNodeName
			tc.nodeSpecMap[n] = spec
		}
	}

	// make a note of the renaming
	tc.setRenamedNode(nodeName, newNodeName)
	return nil
}

// GetRenamedNode : get new name of node if renamed, otherwise emty
func (tc *TreeCache) GetRenamedNode(nodeName string) string {
	return tc.renamedNodesMap[nodeName]
}

// setRenamedNode : set the new name of node
func (tc *TreeCache) setRenamedNode(nodeName string, newNodeName string) {
	tc.removeRenamedNode(newNodeName)
	tc.renamedNodesMap[nodeName] = newNodeName
}

// removeRenamedNode : remove a node from the renamed nodes map
func (tc *TreeCache) removeRenamedNode(nodeName string) {
	delete(tc.renamedNodesMap, nodeName)
}

// clearRenamedNodes : delete map of renamed nodes
func (tc *TreeCache) clearRenamedNodes() {
	tc.renamedNodesMap = make(map[string]string)
}

// DeleteNode : delete a node from the cache
func (tc *TreeCache) DeleteNode(nodeName string) {
	delete(tc.nodeSpecMap, nodeName)
}

// GetNodeNames : get a sorted list of node names in the cache
func (tc *TreeCache) GetNodeNames() []string {
	names := make([]string, 0, len(tc.nodeSpecMap))
	for name := range tc.nodeSpecMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// FromFile : fill cache from a JSON file
func (tc *TreeCache) FromFile(treeFileName string) error {
	/*
	 * parse JSON file
	 */
	byteValue, err := os.ReadFile(treeFileName)
	if err != nil {
		return fmt.Errorf("error reading file: %s", err.Error())
	}
	return tc.FromString(string(byteValue))
}

// FromString : fill cache from the JSON string representation of the tree
func (tc *TreeCache) FromString(treeString string) error {
	var jQuotaTree utils.JQuotaTree
	err := json.Unmarshal([]byte(treeString), &jQuotaTree)
	if err != nil {
		return fmt.Errorf("error parsing tree: %s", err.Error())
	}
	return tc.FromStruct(jQuotaTree)
}

// FromStruct : fill cache from the JSON struct of the tree
func (tc *TreeCache) FromStruct(jQuotaTree utils.JQuotaTree) error {
	/*
	 * process all fields
	 */
	klog.V(4).Infoln("kind=" + jQuotaTree.Kind)
	if jQuotaTree.Kind != utils.DefaultTreeKind {
		return fmt.Errorf("invalid kind: %s", jQuotaTree.Kind)
	}

	klog.V(4).Info("treeName=" + jQuotaTree.MetaData.Name + "; ")
	tc.SetTreeName(jQuotaTree.MetaData.Name)

	tc.AddResourceNames(jQuotaTree.Spec.ResourceNames)
	resourceNames := tc.GetResourceNames()
	klog.V(4).Infof("numResources=%d\n", len(resourceNames))
	klog.V(4).Infof("resourceNames = %s\n", resourceNames)
	klog.V(4).Infoln()

	errNodes := tc.AddNodeSpecs(jQuotaTree.Spec.Nodes)
	if errNodes != nil {
		return fmt.Errorf("invalid specs for some nodes")
	}
	return nil
}

// TreeCacheCreateResponse : response information of creating a tree from the cache
type TreeCacheCreateResponse struct {
	// the unique name of the tree
	TreeName string
	// name of root node, empty if missing root
	RootNodeName string
	// names of dangling nodes (nodes in the cache but not connected in the tree)
	DanglingNodeNames []string
}

// IsClean : tree created from cache has a root and no dangling nodes
func (response *TreeCacheCreateResponse) IsClean() bool {
	return len(response.RootNodeName) > 0 && len(response.DanglingNodeNames) == 0
}

// String : printout of TreeCacheCreateResponse
func (response *TreeCacheCreateResponse) String() string {
	var b bytes.Buffer
	b.WriteString("Response: {")
	fmt.Fprintf(&b, "TreeName=%s; ", response.TreeName)
	fmt.Fprintf(&b, "RootNodeName=\"%s\"; ", response.RootNodeName)
	fmt.Fprintf(&b, "DanglingNodeNames=%s", response.DanglingNodeNames)
	b.WriteString("}")
	return b.String()
}

// CreateTree : create tree from cache
func (tc *TreeCache) CreateTree() (*QuotaTree, *TreeCacheCreateResponse) {

	// names of resources
	resourceNames := make([]string, 0)
	// map of all nodes in the tree
	nodeMap := make(map[string]*QuotaNode)
	// map from node ID to parent ID for non-root nodes
	parentMap := make(map[string]string)
	// the name of the root node
	var rootNodeName string
	// tree create response
	response := &TreeCacheCreateResponse{
		TreeName:          tc.treeName,
		RootNodeName:      "",
		DanglingNodeNames: make([]string, 0),
	}
	var b bytes.Buffer

	/*
	 * set default values, if missing
	 */
	if len(tc.GetTreeName()) == 0 {
		tc.SetDefaultTreeName()
	}
	if tc.GetNumResourceNames() == 0 {
		tc.SetDefaultResourceNames()
	}

	/*
	 * process all nodes
	 */
	nodeNames := tc.GetNodeNames()
	for _, nodeName := range nodeNames {

		fmt.Fprintf(&b, "node=%s; ", nodeName)
		nodeSpec := tc.nodeSpecMap[nodeName]
		parent := nodeSpec.Parent
		if len(parent) == 0 {
			parent = "nil"
		}
		fmt.Fprintf(&b, "parent="+parent+"; ")

		if parent == "nil" {
			if len(rootNodeName) > 0 {
				klog.Errorf("node " + nodeName + ": duplicate root(" + rootNodeName + "); skipping node \n")
				continue
			}
			rootNodeName = nodeName
		} else {
			parentMap[nodeName] = parent
		}

		hard, err := strconv.ParseBool(nodeSpec.Hard)
		if err != nil {
			hard = false
		}
		fmt.Fprintf(&b, "hard="+strconv.FormatBool(hard)+"; ")

		resourceNames = tc.GetResourceNames()
		numResources := tc.GetNumResourceNames()
		values := make([]int, numResources)
		for i, res := range resourceNames {
			values[i], err = strconv.Atoi(nodeSpec.Quota[res])
			if err != nil {
				klog.Errorf("node " + nodeName + ": error converting " +
					nodeSpec.Quota[res] + "; assuming 0 \n")
			}
			fmt.Fprintf(&b, res+"="+strconv.Itoa(values[i])+"; ")
		}

		/*
		 * create node and add to map
		 */
		alloc, _ := NewAllocationCopy(values)
		node, _ := NewQuotaNodeHard(nodeName, alloc, hard)
		nodeMap[nodeName] = node
		fmt.Fprintln(&b)
	}

	/*
	 * set parents and children for all nodes
	 */
	if len(rootNodeName) > 0 {
		for nodeID, qn := range nodeMap {
			parentID := parentMap[nodeID]
			if len(parentID) > 0 {
				parentNode := nodeMap[parentID]
				if parentNode != nil {
					parentNode.AddChild(&qn.Node)
				} else {
					klog.Errorf("node " + nodeID + ": parent node " + parentID + " unknown; dropping node \n")
				}
			}
		}
	} else {
		klog.Errorf("root node is missing; creating empty tree \n")
	}

	/*
	 * create tree
	 */
	klog.V(4).Infoln(b.String())
	tree := NewQuotaTree(tc.GetTreeName(), nodeMap[rootNodeName], resourceNames)
	klog.V(4).Infoln(tree.StringSimply())
	klog.V(4).Infoln()
	klog.V(4).Infoln(tree)

	/*
	 * prepare response
	 */
	response.RootNodeName = rootNodeName
	treeNodeNames := make(map[string]bool)
	for _, name := range tree.GetNodeIDs() {
		treeNodeNames[name] = true
	}
	for _, name := range nodeNames {
		if !treeNodeNames[name] {
			response.DanglingNodeNames = append(response.DanglingNodeNames, name)
		}
	}

	return tree, response
}

// Clear : clear the cache
func (tc *TreeCache) Clear() {
	tc.clearTreeName()
	tc.clearResourceNames()
	tc.clearNodeSpecs()
	tc.clearRenamedNodes()
}
