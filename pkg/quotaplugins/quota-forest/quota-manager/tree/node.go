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

// Node : a basic node in a tree
type Node struct {
	// unique id for the node
	ID string
	// value associated with the node
	value int
	// the parent of this node in the tree
	parent *Node
	// set of children of this node in the tree (node ID -> node)
	children map[string]*Node
}

// NewNode : create a node
func NewNode(id string) *Node {
	return &Node{
		ID:       id,
		value:    0,
		parent:   nil,
		children: make(map[string]*Node),
	}
}

// GetID : the unique ID of this node
func (n *Node) GetID() string {
	return n.ID
}

// GetValue : the value of this node
func (n *Node) GetValue() int {
	return n.value
}

// SetValue : set the value of this node
func (n *Node) SetValue(value int) {
	n.value = value
}

// GetParent : the parent of this node
func (n *Node) GetParent() *Node {
	return n.parent
}

// setParent : set the parent of this node
func (n *Node) setParent(parent *Node) {
	n.parent = parent
}

// GetChildren : the children of this node
func (n *Node) GetChildren() []*Node {
	children := make([]*Node, 0, len(n.children))
	for _, c := range n.children {
		children = append(children, c)
	}
	return children
}

// IsRoot : is this node the root of the tree
func (n *Node) IsRoot() bool {
	return n.parent == nil
}

// IsLeaf : is this node a leaf in the tree
func (n *Node) IsLeaf() bool {
	return len(n.children) == 0
}

// HasLeaf : does the subtree from this node has a given node as a leaf
func (n *Node) HasLeaf(leafID string) bool {
	for _, leaf := range n.GetLeaves() {
		if leaf.GetID() == leafID {
			return true
		}
	}
	return false
}

// AddChild : add a child to this node;
// return false if child already exists
func (n *Node) AddChild(child *Node) bool {
	if child != nil {
		cid := child.GetID()
		if _, exists := n.children[cid]; !exists {
			n.children[cid] = child
			child.setParent(n)
			return true
		}
	}
	return false
}

// RemoveChild : remove a child from this node
func (n *Node) RemoveChild(child *Node) bool {
	if child != nil {
		cid := child.GetID()
		if _, exists := n.children[cid]; exists {
			delete(n.children, cid)
			child.setParent(nil)
			return true
		}
	}
	return false
}

// GetNumChildren : the number of children of this node
func (n *Node) GetNumChildren() int {
	return len(n.children)
}

// GetHeight : the height of this node in the tree
func (n *Node) GetHeight() int {
	h := 0
	for _, c := range n.children {
		ch := c.GetHeight() + 1
		if ch > h {
			h = ch
		}
	}
	return h
}

// GetLeaves : the leaf nodes in the subtree consisting of this node as a root
func (n *Node) GetLeaves() []*Node {
	list := make([]*Node, 0)
	if n.IsLeaf() {
		list = append(list, n)
	} else {
		for _, c := range n.children {
			list = append(list, c.GetLeaves()...)
		}
	}
	return list
}

// GetPathToRoot : the path from this node to the root of the tree;
// node (root) is first (last) in list
func (n *Node) GetPathToRoot() []*Node {
	path := make([]*Node, 0)
	node := n
	for !node.IsRoot() {
		path = append(path, node)
		node = node.GetParent()
	}
	path = append(path, node)
	return path
}

// String : a print out of the node;
// e.g. A -> ( C -> ( D ) B )
func (n *Node) String() string {
	var b bytes.Buffer
	b.WriteString(n.ID)
	if !n.IsLeaf() {
		b.WriteString(" -> ( ")
		// order children by name
		ids := make([]string, 0, len(n.children))
		for id := range n.children {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Fprintf(&b, "%s ", n.children[id])
		}
		b.WriteString(")")
	}
	return b.String()
}
