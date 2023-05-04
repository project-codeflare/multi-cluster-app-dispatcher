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

	tree "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/tree"
)

var (
	nodeA  *tree.Node = tree.NewNode("A")
	alloc1 Allocation = Allocation{
		x: []int{5, 10, 20},
	}
	quotaNodeA QuotaNode = QuotaNode{
		Node:   *nodeA,
		quota:  &alloc1,
		isHard: false,
		allocated: &Allocation{
			x: make([]int, len(alloc1.x)),
		},
		consumers: make([]*Consumer, 0),
	}
)

func TestNewQuotaNode(t *testing.T) {
	type args struct {
		id    string
		quota *Allocation
	}
	tests := []struct {
		name    string
		args    args
		want    *QuotaNode
		wantErr bool
	}{
		{name: "testA",
			args: args{
				id:    "A",
				quota: &alloc1,
			},
			want:    &quotaNodeA,
			wantErr: false,
		},
		{name: "test empty ID",
			args: args{
				id:    "",
				quota: &alloc1,
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

func TestQuotaNode_String(t *testing.T) {
	type fields struct {
		quotaNode QuotaNode
	}
	type args struct {
		level int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{name: "testA",
			fields: fields{
				quotaNode: quotaNodeA,
			},
			args: args{
				level: 1,
			},
			want: "--|A: isHard=false; quota=[5 10 20]; allocated=[0 0 0]; consumers={ }\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qn := &tt.fields.quotaNode
			if got := qn.String(tt.args.level); got != tt.want {
				t.Errorf("QuotaNode.String() = %s, want %s", got, tt.want)
			}
		})
	}
}
