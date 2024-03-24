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
	"reflect"
	"testing"
)

var (
	treeA *Tree = &Tree{
		root: nodeA,
		leafNodeMap: map[string]*Node{
			"B": nodeB,
			"D": nodeD,
		},
	}
	treeNil *Tree = &Tree{
		root:        nil,
		leafNodeMap: make(map[string]*Node),
	}
)

func TestNewTree(t *testing.T) {
	type args struct {
		root *Node
	}
	tests := []struct {
		name string
		args args
		want *Tree
	}{
		{
			name: "good tree",
			args: args{
				root: nodeA,
			},
			want: treeA,
		},
		{
			name: "empty tree",
			args: args{
				root: nil,
			},
			want: treeNil,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewTree(tt.args.root); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTree() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTree_GetHeight(t *testing.T) {
	type fields struct {
		tree *Tree
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "good tree",
			fields: fields{
				tree: treeA,
			},
			want: 2,
		},
		{
			name: "empty tree",
			fields: fields{
				tree: treeNil,
			},
			want: 0,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.tree
			if got := tr.GetHeight(); got != tt.want {
				t.Errorf("Tree.GetHeight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTree_GetRoot(t *testing.T) {
	type fields struct {
		tree *Tree
	}
	tests := []struct {
		name   string
		fields fields
		want   *Node
	}{
		{
			name: "success",
			fields: fields{
				tree: treeA,
			},
			want: nodeA,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.tree
			if got := tr.GetRoot(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tree.GetRoot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTree_GetLeafIDs(t *testing.T) {
	type fields struct {
		tree *Tree
	}
	tests := []struct {
		name   string
		fields fields
		want   [][]string
	}{
		{
			name: "good tree",
			fields: fields{
				tree: treeA,
			},
			want: [][]string{{"B", "D"}, {"D", "B"}},
		},
		{
			name: "empty tree",
			fields: fields{
				tree: treeNil,
			},
			want: [][]string{{}},
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.tree
			got := tr.GetLeafIDs()
			ok := false
			for _, w := range tt.want {
				if reflect.DeepEqual(got, w) {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("Tree.GetLeafIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTree_GetLeafNode(t *testing.T) {
	type fields struct {
		tree *Tree
	}
	type args struct {
		nodeID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Node
	}{
		{
			name: "node exists",
			fields: fields{
				tree: treeA,
			},
			args: args{
				nodeID: "B",
			},
			want: nodeB,
		},
		{
			name: "node missing",
			fields: fields{
				tree: treeA,
			},
			args: args{
				nodeID: "C",
			},
			want: nil,
		},
		{
			name: "empty tree",
			fields: fields{
				tree: treeNil,
			},
			args: args{
				nodeID: "X",
			},
			want: nil,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.tree
			if got := tr.GetLeafNode(tt.args.nodeID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tree.GetLeafNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTree_GetNode(t *testing.T) {
	type fields struct {
		tree *Tree
	}
	type args struct {
		nodeID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Node
	}{
		{
			name: "leaf node",
			fields: fields{
				tree: treeA,
			},
			args: args{
				nodeID: "B",
			},
			want: nodeB,
		},
		{
			name: "internal node",
			fields: fields{
				tree: treeA,
			},
			args: args{
				nodeID: "C",
			},
			want: nodeC,
		},
		{
			name: "root node",
			fields: fields{
				tree: treeA,
			},
			args: args{
				nodeID: "A",
			},
			want: nodeA,
		},
		{
			name: "missing node",
			fields: fields{
				tree: treeA,
			},
			args: args{
				nodeID: "X",
			},
			want: nil,
		},
		{
			name: "empty tree",
			fields: fields{
				tree: treeNil,
			},
			args: args{
				nodeID: "X",
			},
			want: nil,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.tree
			if got := tr.GetNode(tt.args.nodeID); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tree.GetNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTree_GetNodeIDs(t *testing.T) {
	type fields struct {
		tree *Tree
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "good tree",
			fields: fields{
				tree: treeA,
			},
			want: []string{"A", "B", "C", "D"},
		},
		{
			name: "empty tree",
			fields: fields{
				tree: treeNil,
			},
			want: []string{},
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.fields.tree
			got := tr.GetNodeIDs()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tree.GetNodeIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}
