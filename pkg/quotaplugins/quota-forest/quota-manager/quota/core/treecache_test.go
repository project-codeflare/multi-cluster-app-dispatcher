/*
Copyright 2023 The Multi-Cluster App Dispatcher Authors.

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
	"reflect"
	"testing"
	"unsafe"

	utils "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
	"github.com/stretchr/testify/assert"
)

var (
	nodeSpecA string = `{
		"parent": "nil",
		"hard": "true",
		"quota": {
		  "cpu": "10",
		  "memory": "256"
		}
	}`

	nodeSpecB string = `{
		"parent": "A",
		"hard": "false",
		"quota": {
		  "cpu": "2",
		  "memory": "64"
		}
	}`

	nodeSpecC string = `{
		"parent": "A",
		"quota": {
		  "cpu": "4",
		  "memory": "128"
		}
	}`

	nodeSpecD string = `{
		"parent": "C",
		"quota": {
		  "cpu": "2",
		  "memory": "64"
		}
	}`

	treeInfo string = `{
		"name": "TestTree",
		"resourceNames": [
			"cpu",
			"memory"
		]
	}`

	nodesSpec string = `{ ` +
		`"A": ` + nodeSpecA +
		`, "B": ` + nodeSpecB +
		`, "C": ` + nodeSpecC +
		`, "D": ` + nodeSpecD +
		` }`
)

// TestTreeCacheNames : test tree cache operations on tree and resource names
func TestTreeCacheNames(t *testing.T) {
	// create a tree cache
	treeCache := NewTreeCache()
	assert.NotNil(t, treeCache, "Expecting non-nil tree cache")

	// set tree name
	treeCache.SetDefaultTreeName()
	assert.Equal(t, utils.DefaultTreeName, treeCache.GetTreeName(), "Expecting default tree name")

	treeCache.clearTreeName()
	assert.Equal(t, "", treeCache.GetTreeName(), "Expecting tree name to be cleared")

	treeName := "test-tree"
	treeCache.SetTreeName(treeName)
	assert.Equal(t, treeName, treeCache.GetTreeName(), "Expecting tree name to be set")

	// set resource names
	treeCache.AddResourceName("cpu")
	treeCache.AddResourceNames([]string{"memory", "storage"})
	treeCache.DeleteResourceName("storage")
	finalNames := []string{"cpu", "memory"}
	resourceNames := treeCache.GetResourceNames()
	assert.ElementsMatch(t, finalNames, resourceNames, "Expecting correct resource names")
	numResources := treeCache.GetNumResourceNames()
	assert.Equal(t, len(finalNames), numResources, "Expecting matching number of resource names")

	treeCache.clearResourceNames()
	resourceNames = treeCache.GetResourceNames()
	assert.ElementsMatch(t, []string{}, resourceNames, "Expecting empty resource names")
	numResources = treeCache.GetNumResourceNames()
	assert.Equal(t, 0, numResources, "Expecting empty resource names")

	treeCache.SetDefaultResourceNames()
	resourceNames = treeCache.GetResourceNames()
	finalNames = utils.DefaultResourceNames
	assert.ElementsMatch(t, finalNames, resourceNames, "Expecting default resource names")
	numResources = treeCache.GetNumResourceNames()
	assert.Equal(t, len(finalNames), numResources, "Expecting number of default resource names")
}

// TestTreeCacheNodes : test tree cache operations on nodes
func TestTreeCacheNodes(t *testing.T) {
	// create a tree cache
	treeCache := NewTreeCache()
	assert.NotNil(t, treeCache, "Expecting non-nil tree cache")

	// set tree info
	err := treeCache.AddTreeInfoFromString(treeInfo)
	assert.NoError(t, err, "Expecting no error parsing tree info string")
	wantTreeName := "TestTree"
	assert.Equal(t, wantTreeName, treeCache.GetTreeName(), "Expecting tree name to be set")
	resourceNames := treeCache.GetResourceNames()
	wantResourceNames := []string{"memory", "cpu"}
	assert.ElementsMatch(t, wantResourceNames, resourceNames, "Expecting correct resource names")

	// add node specs
	err = treeCache.AddNodeSpecsFromString(nodesSpec)
	assert.NoError(t, err, "Expecting no error parsing nodes spec string")
	quotaTree, response := treeCache.CreateTree()
	testTree := createTestTree()
	equalTrees := reflect.DeepEqual(quotaTree, testTree)
	assert.True(t, equalTrees,
		"Expecting created tree to be same as input tree, want %v, got %v", testTree, quotaTree)
	assert.True(t, response.IsClean(), "Expecting clean response from tree cache")
	assert.ElementsMatch(t, []string{}, response.DanglingNodeNames, "Expecting no dangling nodes")

}

// createTestTree : create a test quota tree
func createTestTree() *QuotaTree {
	// create quota nodes
	quotaNodeA, _ := NewQuotaNodeHard("A", &Allocation{x: []int{10, 256}}, true)
	quotaNodeB, _ := NewQuotaNodeHard("B", &Allocation{x: []int{2, 64}}, false)
	quotaNodeC, _ := NewQuotaNodeHard("C", &Allocation{x: []int{4, 128}}, false)
	quotaNodeD, _ := NewQuotaNodeHard("D", &Allocation{x: []int{2, 64}}, false)
	// connect nodes: A -> ( B C ( D ) )
	quotaNodeA.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeB)))
	quotaNodeA.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeC)))
	quotaNodeC.AddChild((*tree.Node)(unsafe.Pointer(quotaNodeD)))
	// make quota tree
	treeName := "TestTree"
	resourceNames := []string{"cpu", "memory"}
	quotaTree := NewQuotaTree(treeName, quotaNodeA, resourceNames)
	return quotaTree
}
