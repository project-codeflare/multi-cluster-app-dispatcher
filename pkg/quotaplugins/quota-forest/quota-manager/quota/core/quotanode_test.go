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
package core

import (
	"reflect"
	"testing"
	"unsafe"

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
)

var (
	quotaNodeA QuotaNode
	quotaNodeB QuotaNode
	quotaNodeC QuotaNode
	quotaNodeD QuotaNode

	consumer1 *Consumer = &Consumer{
		id:            "C1",
		treeID:        "T1",
		groupID:       "B",
		request:       &Allocation{x: []int{1, 2}},
		priority:      0,
		cType:         0,
		unPreemptable: false,
		aNode:         nil,
	}

	consumer2 *Consumer = &Consumer{
		id:            "C2",
		treeID:        "T1",
		groupID:       "D",
		request:       &Allocation{x: []int{3, 4}},
		priority:      0,
		cType:         1,
		unPreemptable: false,
		aNode:         nil,
	}

	consumer3 *Consumer = &Consumer{
		id:            "C3",
		treeID:        "T1",
		groupID:       "B",
		request:       &Allocation{x: []int{5, 6}},
		priority:      0,
		cType:         0,
		unPreemptable: false,
		aNode:         nil,
	}

	nodeT      *tree.Node = tree.NewNode("T")
	quotaNodeT QuotaNode  = QuotaNode{
		Node:      *nodeT,
		quota:     &Allocation{x: []int{}},
		isHard:    false,
		allocated: &Allocation{x: []int{}},
		consumers: []*Consumer{},
	}
)

func TestNewQuotaNode(t *testing.T) {
	type args struct {
		id    string
		quota *Allocation
	}
	type data struct {
		quota *Allocation
		alloc *Allocation
	}
	tests := []struct {
		name    string
		args    args
		data    data
		want    *QuotaNode
		wantErr bool
	}{
		{name: "testA",
			args: args{
				id:    "A",
				quota: &Allocation{x: []int{10, 20}},
			},
			data: data{
				quota: &Allocation{x: []int{10, 20}},
				alloc: &Allocation{x: []int{0, 0}},
			},
			want: &QuotaNode{
				Node:      *tree.NewNode("A"),
				quota:     &Allocation{x: []int{10, 20}},
				isHard:    false,
				allocated: &Allocation{x: []int{0, 0}},
				consumers: []*Consumer{},
			},
			wantErr: false,
		},
		{name: "test empty ID",
			args: args{
				id:    "",
				quota: &Allocation{x: []int{10, 20}},
			},
			want:    nil,
			wantErr: true,
		},
		{name: "test empty quota",
			args: args{
				id:    "A",
				quota: nil,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := NewQuotaNode(tt.args.id, tt.args.quota); !reflect.DeepEqual(got, tt.want) {
				if (err != nil) != tt.wantErr {
					t.Errorf("NewQuotaNode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				t.Errorf("NewQuotaNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewQuotaNodeHard(t *testing.T) {
	type args struct {
		id     string
		quota  *Allocation
		isHard bool
	}
	type data struct {
		quota *Allocation
		alloc *Allocation
	}
	tests := []struct {
		name    string
		args    args
		data    data
		want    *QuotaNode
		wantErr bool
	}{
		{name: "test1",
			args: args{
				id:     "A",
				quota:  &Allocation{x: []int{10, 20}},
				isHard: false,
			},
			data: data{
				quota: &Allocation{x: []int{10, 20}},
				alloc: &Allocation{x: []int{0, 0}},
			},
			want: &QuotaNode{
				Node:      *tree.NewNode("A"),
				quota:     &Allocation{x: []int{10, 20}},
				isHard:    false,
				allocated: &Allocation{x: []int{0, 0}},
				consumers: []*Consumer{},
			},
			wantErr: false,
		},
		{name: "test2",
			args: args{
				id:     "A",
				quota:  &Allocation{x: []int{10, 20}},
				isHard: true,
			},
			data: data{
				quota: &Allocation{x: []int{10, 20}},
				alloc: &Allocation{x: []int{0, 0}},
			},
			want: &QuotaNode{
				Node:      *tree.NewNode("A"),
				quota:     &Allocation{x: []int{10, 20}},
				isHard:    true,
				allocated: &Allocation{x: []int{0, 0}},
				consumers: []*Consumer{},
			},
			wantErr: false,
		},
		{name: "test3",
			args: args{
				id:     "A",
				quota:  nil,
				isHard: true,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewQuotaNodeHard(tt.args.id, tt.args.quota, tt.args.isHard)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewQuotaNodeHard() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewQuotaNodeHard() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaNode_CanFit(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	type args struct {
		c *Consumer
	}
	type results struct {
		alloc *Allocation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		results results
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				quota:     &Allocation{x: []int{10, 20}},
				allocated: &Allocation{x: []int{5, 8}},
			},
			args: args{
				c: &Consumer{
					id:      "c1",
					request: &Allocation{x: []int{3, 2}},
				},
			},
			want: true,
			results: results{
				alloc: &Allocation{x: []int{5, 8}},
			},
		},
		{
			name: "test2",
			fields: fields{
				Node:      *nodeT,
				quota:     &Allocation{x: []int{10, 20}},
				allocated: &Allocation{x: []int{5, 8}},
			},
			args: args{
				c: &Consumer{
					id:      "c1",
					request: &Allocation{x: []int{10, 10}},
				},
			},
			want: false,
			results: results{
				alloc: &Allocation{x: []int{5, 8}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.CanFit(tt.args.c); got != tt.want {
				t.Errorf("QuotaNode.CanFit() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(qn.allocated, tt.results.alloc) {
				t.Errorf("QuotaNode.CanFit(): quota = %v, want %v", qn.allocated, tt.results.alloc)
			}
		})
	}
}

func TestQuotaNode_AddRequest(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	type args struct {
		c *Consumer
	}
	type results struct {
		r *Allocation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		results results
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				allocated: &Allocation{x: []int{1, 2}},
			},
			args: args{
				c: &Consumer{id: "C1",
					request: &Allocation{x: []int{1, 0}}},
			},
			results: results{
				&Allocation{x: []int{2, 2}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			qn.AddRequest(tt.args.c)
			if !reflect.DeepEqual(qn.allocated, tt.results.r) {
				t.Errorf("QuotaNode.AddRequest(): allocated = %v, want %v", qn.allocated, tt.results.r)
			}
		})
	}
}

func TestQuotaNode_SubtractRequest(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	type args struct {
		c *Consumer
	}
	type results struct {
		r *Allocation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		results results
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				allocated: &Allocation{x: []int{1, 2}},
			},
			args: args{
				c: &Consumer{id: "C1",
					request: &Allocation{x: []int{1, 0}}},
			},
			results: results{
				&Allocation{x: []int{0, 2}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			qn.SubtractRequest(tt.args.c)
			if !reflect.DeepEqual(qn.allocated, tt.results.r) {
				t.Errorf("QuotaNode.SubtractRequest(): allocated = %v, want %v", qn.allocated, tt.results.r)
			}
		})
	}
}

func TestQuotaNode_AddConsumer(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	type args struct {
		c *Consumer
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{},
			},
			args: args{c: consumer1},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{consumer1},
			},
			args: args{c: consumer2},
			want: true,
		},
		{
			name: "test3",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{consumer1},
			},
			args: args{c: consumer1},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.AddConsumer(tt.args.c); got != tt.want {
				t.Errorf("QuotaNode.AddConsumer() = %v, want %v", got, tt.want)
			}
			if !containsConsumer(qn.consumers, tt.args.c) {
				t.Errorf("QuotaNode.AddConsumer(): consumer %v not added to consumers = %v",
					tt.args.c, qn.consumers)
			}
		})
	}
}

func TestQuotaNode_RemoveConsumer(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	type args struct {
		c *Consumer
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{},
			},
			args: args{
				c: consumer1,
			},
			want: false,
		},
		{
			name: "test2",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{consumer1, consumer2},
			},
			args: args{
				c: consumer1,
			},
			want: true,
		},
		{
			name: "test3",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{consumer1},
			},
			args: args{
				c: consumer2,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.RemoveConsumer(tt.args.c); got != tt.want {
				t.Errorf("QuotaNode.RemoveConsumer() = %v, want %v", got, tt.want)
			}
			if containsConsumer(qn.consumers, tt.args.c) {
				t.Errorf("QuotaNode.removeConsumer(): consumer %v not removed from consumers = %v",
					tt.args.c, qn.consumers)
			}
		})
	}
}

func TestQuotaNode_Allocate(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	type args struct {
		c *Consumer
	}
	type results struct {
		alloc *Allocation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		results results
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				quota:     &Allocation{x: []int{10, 20}},
				allocated: &Allocation{x: []int{5, 8}},
				consumers: []*Consumer{},
			},
			args: args{
				c: &Consumer{
					id:      "c1",
					request: &Allocation{x: []int{3, 2}},
				},
			},
			results: results{
				alloc: &Allocation{x: []int{8, 10}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			qn.Allocate(tt.args.c)
			if !reflect.DeepEqual(qn.allocated, tt.results.alloc) {
				t.Errorf("QuotaNode.Allocate(): allocated = %v, want %v", qn.allocated, tt.results.alloc)
			}
			if !containsConsumer(qn.consumers, tt.args.c) {
				t.Errorf("QuotaNode.Allocate(): consumer %v not added to consumers = %v",
					tt.args.c, qn.consumers)
			}
			if tt.args.c.aNode != qn {
				t.Errorf("QuotaNode.Allocate(): consumer aNode %v not set to QuotaNode %v",
					tt.args.c.aNode, qn)
			}
		})
	}
}

func TestQuotaNode_SlideDown(t *testing.T) {
	type fields struct {
		qn *QuotaNode
	}
	type data struct {
		quota           *Allocation
		allocated       *Allocation
		parent          *QuotaNode
		parentQuota     *Allocation
		parentAlloc     *Allocation
		parentConsumers []*Consumer
	}
	type results struct {
		parentAlloc *Allocation
		nodeAlloc   *Allocation
	}
	tests := []struct {
		name    string
		fields  fields
		data    data
		results results
	}{
		{
			name: "test1",
			fields: fields{
				qn: &quotaNodeB,
			},
			data: data{
				quota:           &Allocation{x: []int{2, 2}},
				allocated:       &Allocation{x: []int{0, 0}},
				parent:          &quotaNodeA,
				parentQuota:     &Allocation{x: []int{10, 20}},
				parentAlloc:     &Allocation{x: []int{0, 0}},
				parentConsumers: []*Consumer{consumer1, consumer2, consumer3},
			},
			results: results{
				parentAlloc: &Allocation{x: []int{9, 12}},
				nodeAlloc:   &Allocation{x: []int{1, 2}},
			},
		},
		{
			name: "test2",
			fields: fields{
				qn: &quotaNodeA,
			},
			data: data{
				quota:     &Allocation{x: []int{5, 5}},
				allocated: &Allocation{x: []int{1, 2}},
				parent:    nil,
			},
			results: results{
				nodeAlloc: &Allocation{x: []int{1, 2}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connectTree()
			if tt.data.parent != nil {
				p := tt.data.parent
				p.quota = tt.data.parentQuota
				p.allocated = tt.data.parentAlloc
				for _, c := range tt.data.parentConsumers {
					p.Allocate(c)
				}
			}
			qn := tt.fields.qn
			qn.quota = tt.data.quota
			qn.allocated = tt.data.allocated
			qn.SlideDown()
			if !reflect.DeepEqual(qn.allocated, tt.results.nodeAlloc) {
				t.Errorf("QuotaNode.SlideDown(): node allocated = %v, want %v", qn.allocated, tt.results.nodeAlloc)
			}
			if tt.data.parent != nil {
				if !reflect.DeepEqual(tt.data.parent.allocated, tt.results.parentAlloc) {
					t.Errorf("QuotaNode.SlideDown(): parent allocated = %v, want %v", tt.data.parent.allocated, tt.results.parentAlloc)
				}
			}
		})
	}
}

func TestQuotaNode_SlideUp(t *testing.T) {
	type fields struct {
		qn *QuotaNode
	}
	type args struct {
		c                  *Consumer
		applyPriority      bool
		allocationRecovery *AllocationRecovery
		preemptedConsumers *[]string
	}
	type data struct {
		nodeQuota       *Allocation
		nodeAlloc       *Allocation // before adding consumers to node
		nodeConsumers   []*Consumer // consumerd to be added to node
		parent          *QuotaNode
		parentQuota     *Allocation
		parentAlloc     *Allocation // before adding consumers to parent node
		parentConsumers []*Consumer // consumerd to be added to parent node
	}
	type results struct {
		nodeAlloc          *Allocation
		nodeConsumers      []*Consumer
		parentAlloc        *Allocation
		parentConsumers    []*Consumer
		preemptedConsumers []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		data    data
		results results
		want    bool
	}{
		{
			name: "test1",
			fields: fields{
				qn: &quotaNodeB,
			},
			args: args{
				c:             consumer3,
				applyPriority: false,
				allocationRecovery: &AllocationRecovery{
					consumer:              consumer3,
					alteredNodes:          []*QuotaNode{},
					alteredConsumers:      make(map[string]*Consumer),
					originalConsumersNode: map[string]*QuotaNode{},
				},
				preemptedConsumers: &[]string{},
			},
			data: data{
				nodeQuota:       &Allocation{x: []int{5, 6}},
				nodeAlloc:       &Allocation{x: []int{0, 0}},
				nodeConsumers:   []*Consumer{consumer1},
				parent:          &quotaNodeA,
				parentQuota:     &Allocation{x: []int{10, 10}},
				parentAlloc:     &Allocation{x: []int{0, 0}},
				parentConsumers: []*Consumer{},
			},
			results: results{
				nodeAlloc:          &Allocation{x: []int{0, 0}},
				nodeConsumers:      []*Consumer{},
				parentAlloc:        &Allocation{x: []int{1, 2}},
				parentConsumers:    []*Consumer{consumer1},
				preemptedConsumers: []string{},
			},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				qn: &quotaNodeB,
			},
			args: args{
				c:             consumer3,
				applyPriority: false,
				allocationRecovery: &AllocationRecovery{
					consumer:              consumer3,
					alteredNodes:          []*QuotaNode{},
					alteredConsumers:      make(map[string]*Consumer),
					originalConsumersNode: map[string]*QuotaNode{},
				},
				preemptedConsumers: &[]string{},
			},
			data: data{
				nodeQuota:       &Allocation{x: []int{4, 4}},
				nodeAlloc:       &Allocation{x: []int{0, 0}},
				nodeConsumers:   []*Consumer{consumer1},
				parent:          &quotaNodeA,
				parentQuota:     &Allocation{x: []int{10, 10}},
				parentAlloc:     &Allocation{x: []int{0, 0}},
				parentConsumers: []*Consumer{},
			},
			results: results{
				nodeAlloc:          &Allocation{x: []int{1, 2}},
				nodeConsumers:      []*Consumer{consumer1},
				parentAlloc:        &Allocation{x: []int{1, 2}},
				parentConsumers:    []*Consumer{},
				preemptedConsumers: []string{},
			},
			want: false,
		},
		{
			name: "test3",
			fields: fields{
				qn: &quotaNodeA,
			},
			args: args{
				c:             consumer3,
				applyPriority: false,
				allocationRecovery: &AllocationRecovery{
					consumer:              consumer3,
					alteredNodes:          []*QuotaNode{},
					alteredConsumers:      make(map[string]*Consumer),
					originalConsumersNode: map[string]*QuotaNode{},
				},
				preemptedConsumers: &[]string{},
			},
			data: data{
				nodeQuota:     &Allocation{x: []int{5, 6}},
				nodeAlloc:     &Allocation{x: []int{0, 0}},
				nodeConsumers: []*Consumer{consumer1},
				parent:        nil,
			},
			results: results{
				nodeAlloc:          &Allocation{x: []int{0, 0}},
				nodeConsumers:      []*Consumer{},
				preemptedConsumers: []string{consumer1.id},
			},
			want: true,
		},
		{
			name: "test4",
			fields: fields{
				qn: &quotaNodeA,
			},
			args: args{
				c:             consumer3,
				applyPriority: false,
				allocationRecovery: &AllocationRecovery{
					consumer:              consumer3,
					alteredNodes:          []*QuotaNode{},
					alteredConsumers:      make(map[string]*Consumer),
					originalConsumersNode: map[string]*QuotaNode{},
				},
				preemptedConsumers: &[]string{},
			},
			data: data{
				nodeQuota:     &Allocation{x: []int{4, 4}},
				nodeAlloc:     &Allocation{x: []int{0, 0}},
				nodeConsumers: []*Consumer{consumer1},
				parent:        nil,
			},
			results: results{
				nodeAlloc:          &Allocation{x: []int{1, 2}},
				nodeConsumers:      []*Consumer{consumer1},
				preemptedConsumers: []string{},
			},
			want: false,
		},
		{
			name: "test5",
			fields: fields{
				qn: &quotaNodeC,
			},
			args: args{
				c:             consumer3,
				applyPriority: false,
				allocationRecovery: &AllocationRecovery{
					consumer:              consumer3,
					alteredNodes:          []*QuotaNode{},
					alteredConsumers:      make(map[string]*Consumer),
					originalConsumersNode: map[string]*QuotaNode{},
				},
				preemptedConsumers: &[]string{},
			},
			data: data{
				nodeQuota:       &Allocation{x: []int{5, 6}},
				nodeAlloc:       &Allocation{x: []int{0, 0}},
				nodeConsumers:   []*Consumer{consumer1},
				parent:          &quotaNodeA,
				parentQuota:     &Allocation{x: []int{10, 10}},
				parentAlloc:     &Allocation{x: []int{0, 0}},
				parentConsumers: []*Consumer{},
			},
			results: results{
				nodeAlloc:          &Allocation{x: []int{1, 2}},
				nodeConsumers:      []*Consumer{consumer1},
				parentAlloc:        &Allocation{x: []int{1, 2}},
				parentConsumers:    []*Consumer{},
				preemptedConsumers: []string{},
			},
			want: false,
		},
		{
			name: "test6",
			fields: fields{
				qn: &quotaNodeA,
			},
			args: args{
				c:             consumer2,
				applyPriority: false,
				allocationRecovery: &AllocationRecovery{
					consumer:              consumer2,
					alteredNodes:          []*QuotaNode{},
					alteredConsumers:      make(map[string]*Consumer),
					originalConsumersNode: map[string]*QuotaNode{},
				},
				preemptedConsumers: &[]string{},
			},
			data: data{
				nodeQuota:     &Allocation{x: []int{3, 4}},
				nodeAlloc:     &Allocation{x: []int{0, 0}},
				nodeConsumers: []*Consumer{consumer1},
				parent:        nil,
			},
			results: results{
				nodeAlloc:          &Allocation{x: []int{1, 2}},
				nodeConsumers:      []*Consumer{consumer1},
				preemptedConsumers: []string{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connectTree()
			if tt.data.parent != nil {
				p := tt.data.parent
				p.quota = tt.data.parentQuota
				p.allocated = tt.data.parentAlloc
				for _, c := range tt.data.parentConsumers {
					p.Allocate(c)
				}
				for _, c := range tt.data.nodeConsumers {
					p.AddRequest(c)
				}
			}
			qn := tt.fields.qn
			qn.quota = tt.data.nodeQuota
			qn.allocated = tt.data.nodeAlloc
			for _, c := range tt.data.nodeConsumers {
				qn.Allocate(c)
			}
			if got := qn.SlideUp(tt.args.c, tt.args.applyPriority, tt.args.allocationRecovery, tt.args.preemptedConsumers); got != tt.want {
				t.Errorf("QuotaNode.SlideUp() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(qn.allocated, tt.results.nodeAlloc) {
				t.Errorf("QuotaNode.SlideUp(): node allocated = %v, want %v", qn.allocated, tt.results.nodeAlloc)
			}
			if !containsAllConsumers(qn.consumers, tt.results.nodeConsumers) {
				t.Errorf("QuotaNode.SlideUp(): node consumers = %v, want %v", qn.consumers, tt.results.nodeConsumers)
			}
			if tt.data.parent != nil {
				if !reflect.DeepEqual(tt.data.parent.allocated, tt.results.parentAlloc) {
					t.Errorf("QuotaNode.SlideUp(): parent allocated = %v, want %v", tt.data.parent.allocated, tt.results.parentAlloc)
				}
				if !containsAllConsumers(tt.data.parent.consumers, tt.results.parentConsumers) {
					t.Errorf("QuotaNode.SlideUp(): parent consumers = %v, want %v", tt.data.parent.consumers, tt.results.parentConsumers)
				}
			}
			if !sameAllStrings(*tt.args.preemptedConsumers, tt.results.preemptedConsumers) {
				t.Errorf("QuotaNode.SlideUp(): preemptedConsumers = %v, want %v", *tt.args.preemptedConsumers, tt.results.preemptedConsumers)
			}
		})
	}
}

func TestQuotaNode_HasLeaf(t *testing.T) {
	type fields struct {
		qn *QuotaNode
	}
	type args struct {
		c *Consumer
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				qn: &quotaNodeA,
			},
			args: args{
				c: &Consumer{
					id:      "c1",
					groupID: "B",
				},
			},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				qn: &quotaNodeA,
			},
			args: args{
				c: &Consumer{
					id:      "c1",
					groupID: "C",
				},
			},
			want: false,
		},
		{
			name: "test1",
			fields: fields{
				qn: &quotaNodeA,
			},
			args: args{
				c: &Consumer{
					id:      "c1",
					groupID: "D",
				},
			},
			want: true,
		},
	}
	connectTree()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := tt.fields.qn
			if got := qn.HasLeaf(tt.args.c); got != tt.want {
				t.Errorf("QuotaNode.HasLeaf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaNode_IsHard(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				Node:   *nodeT,
				isHard: true,
			},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				Node:   *nodeT,
				isHard: false,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.IsHard(); got != tt.want {
				t.Errorf("QuotaNode.IsHard() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaNode_GetQuota(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	tests := []struct {
		name   string
		fields fields
		want   *Allocation
	}{
		{
			name: "test1",
			fields: fields{
				Node:  *nodeT,
				quota: &Allocation{x: []int{1, 2, 3}},
			},
			want: &Allocation{x: []int{1, 2, 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.GetQuota(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QuotaNode.GetQuota() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaNode_SetQuota(t *testing.T) {
	type fields struct {
		qn *QuotaNode
	}
	type args struct {
		quota *Allocation
	}
	type results struct {
		r *Allocation
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		results results
	}{
		{
			name: "test1",
			fields: fields{
				qn: &quotaNodeT,
			},
			args: args{
				quota: &Allocation{x: []int{10, 20}},
			},
			results: results{
				r: &Allocation{x: []int{10, 20}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := tt.fields.qn
			qn.SetQuota(tt.args.quota)
			if !reflect.DeepEqual(qn.quota, tt.results.r) {
				t.Errorf("QuotaNode.SetQuota(): quota = %v, want %v", qn.quota, tt.results.r)
			}
		})
	}
}

func TestQuotaNode_GetAllocated(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	tests := []struct {
		name   string
		fields fields
		want   *Allocation
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				allocated: &Allocation{x: []int{1, 2, 3}},
			},
			want: &Allocation{x: []int{1, 2, 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.GetAllocated(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QuotaNode.GetAllocated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaNode_GetConsumers(t *testing.T) {
	type fields struct {
		Node      tree.Node
		quota     *Allocation
		isHard    bool
		allocated *Allocation
		consumers []*Consumer
	}
	tests := []struct {
		name   string
		fields fields
		want   []*Consumer
	}{
		{
			name: "test1",
			fields: fields{
				Node:      *nodeT,
				consumers: []*Consumer{consumer1, consumer2},
			},
			want: []*Consumer{consumer1, consumer2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &QuotaNode{
				Node:      tt.fields.Node,
				quota:     tt.fields.quota,
				isHard:    tt.fields.isHard,
				allocated: tt.fields.allocated,
				consumers: tt.fields.consumers,
			}
			if got := qn.GetConsumers(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QuotaNode.GetConsumers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsConsumer(elems []*Consumer, v *Consumer) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func containsAllConsumers(elems []*Consumer, vs []*Consumer) bool {
	for _, v := range vs {
		if !containsConsumer(elems, v) {
			return false
		}
	}
	return true
}

func containsString(elems []string, v string) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func sameAllStrings(elems []string, vs []string) bool {
	if len(elems) != len(vs) {
		return false
	}
	for _, v := range vs {
		if !containsString(elems, v) {
			return false
		}
	}
	return true
}

func connectTree() {
	clearTree()
	//A -> ( B C -> ( D ) )
	nodeA := (*tree.Node)(unsafe.Pointer(&quotaNodeA))
	nodeB := (*tree.Node)(unsafe.Pointer(&quotaNodeB))
	nodeC := (*tree.Node)(unsafe.Pointer(&quotaNodeC))
	nodeD := (*tree.Node)(unsafe.Pointer(&quotaNodeD))

	nodeA.AddChild(nodeB)
	nodeA.AddChild(nodeC)
	nodeC.AddChild(nodeD)
}

func clearTree() {
	quotaNodeA = QuotaNode{
		Node:      *tree.NewNode("A"),
		quota:     &Allocation{x: []int{}},
		isHard:    false,
		allocated: &Allocation{x: []int{}},
		consumers: []*Consumer{},
	}

	quotaNodeB = QuotaNode{
		Node:      *tree.NewNode("B"),
		quota:     &Allocation{x: []int{}},
		isHard:    false,
		allocated: &Allocation{x: []int{}},
		consumers: []*Consumer{},
	}

	quotaNodeC = QuotaNode{
		Node:      *tree.NewNode("C"),
		quota:     &Allocation{x: []int{}},
		isHard:    true,
		allocated: &Allocation{x: []int{}},
		consumers: []*Consumer{},
	}

	quotaNodeD = QuotaNode{
		Node:      *tree.NewNode("D"),
		quota:     &Allocation{x: []int{}},
		isHard:    false,
		allocated: &Allocation{x: []int{}},
		consumers: []*Consumer{},
	}
}
