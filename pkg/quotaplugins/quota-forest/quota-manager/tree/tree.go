/*
Copyright 2022 The Multi-Cluster App Dispatcher Authors.

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

package tree

import (
	"bytes"
	"fmt"
	"sort"
)

// Tree : a basic tree
type Tree struct {
	// the root of the tree (could be null)
	root *Node
	// a map to help find leaf nodes (leaf ID -> leaf node)
	leafNodeMap map[string]*Node
}

// NewTree : create a tree
func NewTree(root *Node) *Tree {
	t := &Tree{
		root:        root,
		leafNodeMap: make(map[string]*Node),
	}
	for _, n := range t.GetLeaves() {
		t.leafNodeMap[n.GetID()] = n
	}
	return t
}

// GetHeight : the height of the tree
func (t *Tree) GetHeight() int {
	if t.root == nil {
		return 0
	}
	return t.root.GetHeight()
}

// GetRoot : the root of the tree
func (t *Tree) GetRoot() *Node {
	return t.root
}

// GetLeaves : the leaves of the tree
func (t *Tree) GetLeaves() []*Node {
	if t.root == nil {
		return make([]*Node, 0)
	}
	return t.root.GetLeaves()
}

// GetLeafIDs : the IDs of the leaves of the tree
func (t *Tree) GetLeafIDs() []string {
	leafIDs := make([]string, 0)
	if t.root != nil {
		for _, n := range t.root.GetLeaves() {
			leafIDs = append(leafIDs, n.GetID())
		}
	}
	return leafIDs
}

// GetLeafNode : get a leaf node by ID; null if not found
func (t *Tree) GetLeafNode(nodeID string) *Node {
	return t.leafNodeMap[nodeID]
}

// GetNode : find node in tree with a given ID; null if not found
func (t *Tree) GetNode(nodeID string) *Node {
	allNodes := t.GetNodeListBFS()
	for _, n := range allNodes {
		if n.GetID() == nodeID {
			return n
		}
	}
	return nil
}

// GetNodeListBFS : list of nodes in BFS order
func (t *Tree) GetNodeListBFS() []*Node {
	allNodes := make([]*Node, 0)

	nodeList := make([]*Node, 0)
	if t.root != nil {
		nodeList = append(nodeList, t.root)
	}

	for len(nodeList) > 0 {
		n := nodeList[0]
		nodeList = nodeList[1:]
		allNodes = append(allNodes, n)
		nodeList = append(nodeList, n.GetChildren()...)
	}
	return allNodes
}

// GetNodeIDs : get (sorted) IDs of all nodes in the tree
func (t *Tree) GetNodeIDs() []string {
	nodes := t.GetNodeListBFS()
	nodeIDs := make([]string, len(nodes))
	for i, node := range nodes {
		nodeIDs[i] = node.GetID()
	}
	sort.Strings(nodeIDs)
	return nodeIDs
}

// String : a print out of the tree
func (t *Tree) String() string {
	var b bytes.Buffer
	if t.root != nil {
		fmt.Fprintf(&b, "%s", t.root)
	} else {
		b.WriteString("null")
	}
	return b.String()
}
