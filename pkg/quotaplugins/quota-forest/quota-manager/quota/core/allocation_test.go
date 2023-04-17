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
)

func TestNewAllocation(t *testing.T) {
	type args struct {
		size int
	}
	tests := []struct {
		name    string
		args    args
		want    *Allocation
		wantErr bool
	}{
		{name: "test1",
			args: args{
				size: 3,
			},
			want: &Allocation{
				x: []int{0, 0, 0},
			},
			wantErr: false,
		},
		{name: "test2",
			args: args{
				size: 0,
			},
			want: &Allocation{
				x: []int{},
			},
			wantErr: false,
		},
		{name: "test3",
			args: args{
				size: -2,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAllocation(tt.args.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAllocation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAllocation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewAllocationCopy(t *testing.T) {
	type args struct {
		value []int
	}
	tests := []struct {
		name    string
		args    args
		want    *Allocation
		wantErr bool
	}{
		{name: "test1",
			args: args{
				value: []int{1, 2, 3},
			},
			want: &Allocation{
				x: []int{1, 2, 3},
			},
			wantErr: false,
		},
		{name: "test2",
			args: args{
				value: []int{},
			},
			want: &Allocation{
				x: []int{},
			},
			wantErr: false,
		},
		{name: "test3",
			args: args{
				value: nil,
			},
			want: &Allocation{
				x: []int{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAllocationCopy(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAllocationCopy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAllocationCopy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllocation_SetValue(t *testing.T) {
	type fields struct {
		x []int
	}
	type args struct {
		value []int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Allocation
	}{
		{name: "test1",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				value: []int{4, 5},
			},
			want: &Allocation{
				x: []int{4, 5},
			},
		},
		{name: "test2",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				value: []int{},
			},
			want: &Allocation{
				x: []int{},
			},
		},
		{name: "test3",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				value: nil,
			},
			want: &Allocation{
				x: []int{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocation{
				x: tt.fields.x,
			}
			a.SetValue(tt.args.value)
			got := a
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Allocation_SetValue() = %v, want %v", got, tt.want)
			}

		})
	}
}

func TestAllocation_Add(t *testing.T) {
	type fields struct {
		x []int
		y []int
	}
	type args struct {
		other *Allocation
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
				x: []int{1, 2},
				y: []int{4, 6},
			},
			args: args{
				other: &Allocation{
					x: []int{3, 4},
				},
			},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				x: []int{1, 2},
				y: []int{1, 2},
			},
			args: args{
				other: &Allocation{
					x: []int{3, 4, 5},
				},
			},
			want: false,
		},
		{
			name: "test3",
			fields: fields{
				x: []int{1, 2},
				y: []int{4, 9},
			},
			args: args{
				other: &Allocation{
					x: []int{3, 4},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocation{
				x: tt.fields.x,
			}
			if got := a.Add(tt.args.other) && reflect.DeepEqual(a.x, tt.fields.y); got != tt.want {
				t.Errorf("Allocation.Add() = %v, want %v; result = %v, want %v",
					got, tt.want, a.x, tt.fields.y)
			}
		})
	}
}

func TestAllocation_Fit(t *testing.T) {
	type fields struct {
		x []int
	}
	type args struct {
		allocated *Allocation
		capacity  *Allocation
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{name: "test1",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				allocated: &Allocation{
					x: []int{1, 1, 0},
				},
				capacity: &Allocation{
					x: []int{5, 4, 3},
				},
			},
			want: true,
		},
		{name: "test2",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				allocated: &Allocation{
					x: []int{1, 1, 0},
				},
				capacity: &Allocation{
					x: []int{2, 3, 3},
				},
			},
			want: true,
		},
		{name: "test3",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				allocated: &Allocation{
					x: []int{1, 1, 0},
				},
				capacity: &Allocation{
					x: []int{5, 2, 3},
				},
			},
			want: false,
		},
		{name: "test4",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				allocated: &Allocation{
					x: []int{0, 0},
				},
				capacity: &Allocation{
					x: []int{5, 5, 5},
				},
			},
			want: false,
		},
		{name: "test5",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				allocated: &Allocation{
					x: []int{5, 5, 0},
				},
				capacity: &Allocation{
					x: []int{5, 5, 5},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocation{
				x: tt.fields.x,
			}
			if got := a.Fit(tt.args.allocated, tt.args.capacity); got != tt.want {
				t.Errorf("Allocation.Fit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllocation_IsZero(t *testing.T) {
	type fields struct {
		x []int
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "test1",
			fields: fields{
				x: []int{0, 0, 0},
			},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				x: []int{0, 1},
			},
			want: false,
		},
		{
			name: "test3",
			fields: fields{
				x: []int{1, 2, -4},
			},
			want: false,
		},
		{
			name: "test4",
			fields: fields{
				x: []int{},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocation{
				x: tt.fields.x,
			}
			if got := a.IsZero(); got != tt.want {
				t.Errorf("Allocation.IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllocation_Equal(t *testing.T) {
	type fields struct {
		x []int
	}
	type args struct {
		other *Allocation
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
				x: []int{1, 2, 3},
			},
			args: args{
				other: &Allocation{x: []int{1, 2, 3}},
			},
			want: true,
		},
		{
			name: "test2",
			fields: fields{
				x: []int{},
			},
			args: args{
				other: &Allocation{x: []int{}},
			},
			want: true,
		},
		{
			name: "test3",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				other: &Allocation{x: []int{4, 5, 6}},
			},
			want: false,
		},
		{
			name: "test4",
			fields: fields{
				x: []int{1, 2},
			},
			args: args{
				other: &Allocation{x: []int{1, 2, 3}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocation{
				x: tt.fields.x,
			}
			if got := a.Equal(tt.args.other); got != tt.want {
				t.Errorf("Allocation.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllocation_StringPretty(t *testing.T) {
	type fields struct {
		x []int
	}
	type args struct {
		resourceNames []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "test1",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				resourceNames: []string{"cpu", "memory", "gpu"},
			},
			want: "[cpu:1, memory:2, gpu:3]",
		},
		{
			name: "test2",
			fields: fields{
				x: []int{1, 2, 3},
			},
			args: args{
				resourceNames: []string{"cpu", "memory"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Allocation{
				x: tt.fields.x,
			}
			if got := a.StringPretty(tt.args.resourceNames); got != tt.want {
				t.Errorf("Allocation.StringPretty() = %v, want %v", got, tt.want)
			}
		})
	}
}
