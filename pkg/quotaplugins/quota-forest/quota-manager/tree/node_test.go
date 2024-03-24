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
	nodeA *Node = &Node{
		ID:       "A",
		value:    0,
		parent:   nil,
		children: make(map[string]*Node),
	}

	nodeB *Node = &Node{
		ID:       "B",
		value:    0,
		parent:   nil,
		children: make(map[string]*Node),
	}

	nodeC *Node = &Node{
		ID:       "C",
		value:    0,
		parent:   nil,
		children: make(map[string]*Node),
	}

	nodeD *Node = &Node{
		ID:       "D",
		value:    0,
		parent:   nil,
		children: make(map[string]*Node),
	}
)

func resetNodes() {
	nodes := []*Node{nodeA, nodeB, nodeC, nodeD}
	for _, n := range nodes {
		n.parent = nil
		n.children = make(map[string]*Node)
	}
}

func connectNodes() {
	//A -> ( B C -> ( D ) )
	nodeD.parent = nodeC
	nodeC.children["D"] = nodeD
	nodeC.parent = nodeA
	nodeA.children["C"] = nodeC
	nodeB.parent = nodeA
	nodeA.children["B"] = nodeB
}

func TestNewNode(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		name string
		args args
		want *Node
	}{
		{
			name: "node-A",
			args: args{
				id: "A",
			},
			want: nodeA,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewNode(tt.args.id); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_SetValue(t *testing.T) {
	type fields struct {
		ID    string
		value int
	}
	type args struct {
		value int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "value-X",
			fields: fields{
				ID:    "X",
				value: 5,
			},
			args: args{
				value: 10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNode(tt.fields.ID)
			n.SetValue(tt.fields.value)
			n.SetValue(tt.args.value)
			if got := n.GetValue(); got != tt.args.value {
				t.Errorf("SetValue() = %v, want %v", got, tt.args.value)
			}
		})
	}
}

func TestNode_GetChildren(t *testing.T) {
	type fields struct {
		node *Node
	}
	tests := []struct {
		name   string
		fields fields
		want   [][]*Node
	}{
		{
			name: "children-A",
			fields: fields{
				node: nodeA,
			},
			want: [][]*Node{{nodeC, nodeB}, {nodeB, nodeC}},
		},
		{
			name: "children-B",
			fields: fields{
				node: nodeB,
			},
			want: [][]*Node{{}},
		},
		{
			name: "children-C",
			fields: fields{
				node: nodeC,
			},
			want: [][]*Node{{nodeD}},
		},
		{
			name: "children-D",
			fields: fields{
				node: nodeD,
			},
			want: [][]*Node{{}},
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			got := n.GetChildren()
			ok := false
			for _, w := range tt.want {
				if reflect.DeepEqual(got, w) {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("Node.GetChildren() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_HasLeaf(t *testing.T) {
	type args struct {
		leafID string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "leaf-child-B",
			args: args{leafID: "B"},
			want: true,
		},
		{
			name: "leaf-D",
			args: args{leafID: "D"},
			want: true,
		},
		{
			name: "leaf-X",
			args: args{leafID: "X"},
			want: false,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := nodeA
			if got := n.HasLeaf(tt.args.leafID); got != tt.want {
				t.Errorf("Node.HasLeaf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_AddChild(t *testing.T) {
	type fields struct {
		node *Node
	}
	type args struct {
		child *Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "add first",
			fields: fields{
				node: nodeA,
			},
			args: args{
				child: nodeB,
			},
			want: true,
		},
		{
			name: "add again",
			fields: fields{
				node: nodeA,
			},
			args: args{
				child: nodeB,
			},
			want: false,
		},
	}
	resetNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			if got := n.AddChild(tt.args.child); got != tt.want {
				t.Errorf("Node.AddChild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_RemoveChild(t *testing.T) {
	type fields struct {
		node *Node
	}
	type args struct {
		child *Node
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "remove first",
			fields: fields{
				node: nodeA,
			},
			args: args{
				child: nodeB,
			},
			want: true,
		},
		{
			name: "remove again",
			fields: fields{
				node: nodeA,
			},
			args: args{
				child: nodeB,
			},
			want: false,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			if got := n.RemoveChild(tt.args.child); got != tt.want {
				t.Errorf("Node.RemoveChild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_GetNumChildren(t *testing.T) {
	type fields struct {
		node *Node
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "children-A",
			fields: fields{
				node: nodeA,
			},
			want: 2,
		},
		{
			name: "children-B",
			fields: fields{
				node: nodeB,
			},
			want: 0,
		},
		{
			name: "children-C",
			fields: fields{
				node: nodeC,
			},
			want: 1,
		},
		{
			name: "children-D",
			fields: fields{
				node: nodeD,
			},
			want: 0,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			if got := n.GetNumChildren(); got != tt.want {
				t.Errorf("Node.GetNumChildren() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_GetHeight(t *testing.T) {
	type fields struct {
		node *Node
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "height-A",
			fields: fields{
				node: nodeA,
			},
			want: 2,
		},
		{
			name: "height-B",
			fields: fields{
				node: nodeB,
			},
			want: 0,
		},
		{
			name: "height-C",
			fields: fields{
				node: nodeC,
			},
			want: 1,
		},
		{
			name: "height-D",
			fields: fields{
				node: nodeD,
			},
			want: 0,
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			if got := n.GetHeight(); got != tt.want {
				t.Errorf("Node.GetHeight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_GetLeaves(t *testing.T) {
	type fields struct {
		node *Node
	}
	tests := []struct {
		name   string
		fields fields
		want   [][]*Node
	}{
		{
			name: "leaves-A",
			fields: fields{
				node: nodeA,
			},
			want: [][]*Node{{nodeD, nodeB}, {nodeB, nodeD}},
		},
		{
			name: "leaves-B",
			fields: fields{
				node: nodeB,
			},
			want: [][]*Node{{nodeB}},
		},
		{
			name: "leaves-C",
			fields: fields{
				node: nodeC,
			},
			want: [][]*Node{{nodeD}},
		},
		{
			name: "leaves-D",
			fields: fields{
				node: nodeD,
			},
			want: [][]*Node{{nodeD}},
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			got := n.GetLeaves()
			ok := false
			for _, w := range tt.want {
				if reflect.DeepEqual(got, w) {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("Node.GetLeaves() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_GetPathToRoot(t *testing.T) {
	type fields struct {
		node *Node
	}
	tests := []struct {
		name   string
		fields fields
		want   []*Node
	}{
		{
			name: "path-A",
			fields: fields{
				node: nodeA,
			},
			want: []*Node{nodeA},
		},
		{
			name: "path-B",
			fields: fields{
				node: nodeB,
			},
			want: []*Node{nodeB, nodeA},
		},
		{
			name: "path-C",
			fields: fields{
				node: nodeC,
			},
			want: []*Node{nodeC, nodeA},
		},
		{
			name: "path-D",
			fields: fields{
				node: nodeD,
			},
			want: []*Node{nodeD, nodeC, nodeA},
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			if got := n.GetPathToRoot(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Node.GetPathToRoot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNode_String(t *testing.T) {
	type fields struct {
		node *Node
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		{
			name: "print-A",
			fields: fields{
				node: nodeA,
			},
			want: []string{"A -> ( C -> ( D ) B )", "A -> ( B C -> ( D ) )"},
		},
		{
			name: "print-B",
			fields: fields{
				node: nodeB,
			},
			want: []string{"B"},
		},
		{
			name: "print-C",
			fields: fields{
				node: nodeC,
			},
			want: []string{"C -> ( D )"},
		},
		{
			name: "print-D",
			fields: fields{
				node: nodeD,
			},
			want: []string{"D"},
		},
	}
	resetNodes()
	connectNodes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.fields.node
			got := n.String()
			ok := false
			for _, w := range tt.want {
				if reflect.DeepEqual(got, w) {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("Node.ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
