/*
Copyright 2017 The Kubernetes Authors.

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
/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

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
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SchedulingSpecPlural is the plural of SchedulingSpec
const SchedulingSpecPlural = "schedulingspecs"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SchedulingSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec SchedulingSpecTemplate `json:"spec,omitempty" protobuf:"bytes,1,rep,name=spec"`
}

type SchedulingSpecTemplate struct {
	NodeSelector      map[string]string     `json:"nodeSelector,omitempty" protobuf:"bytes,1,rep,name=nodeSelector"`
	MinAvailable      int                   `json:"minAvailable,omitempty" protobuf:"bytes,2,rep,name=minAvailable"`
	Requeuing         RequeuingTemplate     `json:"requeuing,omitempty" protobuf:"bytes,1,rep,name=requeuing"`
	ClusterScheduling ClusterSchedulingSpec `json:"clusterScheduling,omitempty"`
	DispatchingWindow DispatchingWindowSpec `json:"dispatchingWindow,omitempty"`
	DispatchDuration  DispatchDurationSpec  `json:"dispatchDuration,omitempty"`
}

type RequeuingTemplate struct {
	InitialTimeInSeconds int    `json:"initialTimeInSeconds,omitempty" protobuf:"bytes,1,rep,name=initialTimeInSeconds"`
	TimeInSeconds        int    `json:"timeInSeconds,omitempty" protobuf:"bytes,2,rep,name=timeInSeconds"`
	MaxTimeInSeconds     int    `json:"maxTimeInSeconds,omitempty" protobuf:"bytes,3,rep,name=maxTimeInSeconds"`
	GrowthType           string `json:"growthType,omitempty" protobuf:"bytes,4,rep,name=growthType"`
	NumRequeuings        int    `json:"numRequeuings,omitempty" protobuf:"bytes,5,rep,name=numRequeuings"`
	MaxNumRequeuings     int    `json:"maxNumRequeuings,omitempty" protobuf:"bytes,6,rep,name=maxNumRequeuings"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SchedulingSpecList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SchedulingSpec `json:"items"`
}

type ResourceName string

type ClusterReference struct {
	Name string `json:"name"`
}

type ClusterSchedulingSpec struct {
	Clusters        []ClusterReference    `json:"clusters,omitempty"`
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`
	PolicyResult    ClusterDecision       `json:"policyResult,omitempty"`
}

type ClusterDecision struct {
	TargetCluster ClusterReference        `json:"targetCluster,omitempty"`
	PolicySource  []PolicySourceReference `json:"policySource,omitempty"`
}

type PolicySourceReference struct {
	// ID/Name of the policy decision maker.  Most often this will be MCAD but design can support alternatives
	Name                string           `json:"name,omitempty"`
	// The latest time this condition was updated.
	LastUpdateMicroTime metav1.MicroTime `json:"lastUpdateMicroTime,omitempty"`
	// A human readable message indicating details about the cluster decision.
	Message             string           `json:"message,omitempty"`
}

type ScheduleTimeSpec struct {
	Min     metav1.Time `json:"minTimestamp,omitempty"`
	Desired metav1.Time `json:"desiredTimestamp,omitempty"`
	Max     metav1.Time `json:"maxTimestamp,omitempty"`
}

type DispatchDurationSpec struct {
	Expected int  `json:"expected,omitempty"`
	Limit    int  `json:"limit,omitempty"`
	Overrun  bool `json:"overrun,omitempty"`
}

type DispatchingWindowSpec struct {
	Start ScheduleTimeSpec `json:"start,omitempty"`
	End   ScheduleTimeSpec `json:"end,omitempty"`
}
