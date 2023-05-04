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

func TestAllocationResponse_Merge(t *testing.T) {
	type fields struct {
		consumerID   string
		allocated    bool
		message      string
		preemptedIDs map[string]bool
	}
	type args struct {
		other *AllocationResponse
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *AllocationResponse
	}{
		{
			name: "test1",
			fields: fields{
				consumerID:   "C10",
				allocated:    true,
				message:      "success",
				preemptedIDs: map[string]bool{"C2": true, "C5": true},
			},
			args: args{
				other: &AllocationResponse{
					consumerID:   "C10",
					allocated:    true,
					message:      "again",
					preemptedIDs: map[string]bool{"C5": true, "C3": true},
				},
			},
			want: &AllocationResponse{
				consumerID:   "C10",
				allocated:    true,
				message:      "success again",
				preemptedIDs: map[string]bool{"C2": true, "C3": true, "C5": true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ar := &AllocationResponse{
				consumerID:   tt.fields.consumerID,
				allocated:    tt.fields.allocated,
				message:      tt.fields.message,
				preemptedIDs: tt.fields.preemptedIDs,
			}
			ar.Merge(tt.args.other)
			got := ar
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AllocationResponse_Merge = %v, want %v", got, tt.want)
			}
		})
	}
}
