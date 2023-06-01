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
package api

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func jobInfoEqual(l, r *JobInfo) bool {
	if !reflect.DeepEqual(l, r) {
		return false
	}

	return true
}

func TestAddTaskInfo(t *testing.T) {
	// case1
	case01_uid := JobID("uid")
	case01_owner := buildOwnerReference("uid")

	case01_pod1 := buildPod("c1", "p1", "", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_task1 := NewTaskInfo(case01_pod1)
	case01_pod2 := buildPod("c1", "p2", "n1", v1.PodRunning, buildResourceList("2000m", "2G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_task2 := NewTaskInfo(case01_pod2)
	case01_pod3 := buildPod("c1", "p3", "n1", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_task3 := NewTaskInfo(case01_pod3)
	case01_pod4 := buildPod("c1", "p4", "n1", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_task4 := NewTaskInfo(case01_pod4)

	tests := []struct {
		name     string
		uid      JobID
		pods     []*v1.Pod
		expected *JobInfo
	}{
		{
			name: "add 1 pending owner pod, 1 running owner pod",
			uid:  case01_uid,
			pods: []*v1.Pod{case01_pod1, case01_pod2, case01_pod3, case01_pod4},
			expected: &JobInfo{
				UID:          case01_uid,
				MinAvailable: 0,
				Allocated:    buildResource("4000m", "4G"),
				TotalRequest: buildResource("5000m", "5G"),
				Tasks: tasksMap{
					case01_task1.UID: case01_task1,
					case01_task2.UID: case01_task2,
					case01_task3.UID: case01_task3,
					case01_task4.UID: case01_task4,
				},
				TaskStatusIndex: map[TaskStatus]tasksMap{
					Running: {
						case01_task2.UID: case01_task2,
					},
					Pending: {
						case01_task1.UID: case01_task1,
					},
					Bound: {
						case01_task3.UID: case01_task3,
						case01_task4.UID: case01_task4,
					},
				},
				NodeSelector: make(map[string]string),
			},
		},
	}

	for i, test := range tests {
		ps := NewJobInfo(test.uid)

		for _, pod := range test.pods {
			pi := NewTaskInfo(pod)
			ps.AddTaskInfo(pi)
		}

		if !jobInfoEqual(ps, test.expected) {
			t.Errorf("podset info %d: \n expected: %v, \n got: %v \n",
				i, test.expected, ps)
		}
	}
}

func TestDeleteTaskInfo(t *testing.T) {
	// case1
	case01_uid := JobID("owner1")
	case01_owner := buildOwnerReference(string(case01_uid))
	case01_pod1 := buildPod("c1", "p1", "", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_task1 := NewTaskInfo(case01_pod1)
	case01_pod2 := buildPod("c1", "p2", "n1", v1.PodRunning, buildResourceList("2000m", "2G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_pod3 := buildPod("c1", "p3", "n1", v1.PodRunning, buildResourceList("3000m", "3G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01_task3 := NewTaskInfo(case01_pod3)

	// case2
	case02_uid := JobID("owner2")
	case02_owner := buildOwnerReference(string(case02_uid))
	case02_pod1 := buildPod("c1", "p1", "", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case02_owner}, make(map[string]string))
	case02_task1 := NewTaskInfo(case02_pod1)
	case02_pod2 := buildPod("c1", "p2", "n1", v1.PodPending, buildResourceList("2000m", "2G"), []metav1.OwnerReference{case02_owner}, make(map[string]string))
	case02_pod3 := buildPod("c1", "p3", "n1", v1.PodRunning, buildResourceList("3000m", "3G"), []metav1.OwnerReference{case02_owner}, make(map[string]string))
	case02_task3 := NewTaskInfo(case02_pod3)

	tests := []struct {
		name     string
		uid      JobID
		pods     []*v1.Pod
		rmPods   []*v1.Pod
		expected *JobInfo
	}{
		{
			name:   "add 1 pending owner pod, 2 running owner pod, remove 1 running owner pod",
			uid:    case01_uid,
			pods:   []*v1.Pod{case01_pod1, case01_pod2, case01_pod3},
			rmPods: []*v1.Pod{case01_pod2},
			expected: &JobInfo{
				UID:          case01_uid,
				MinAvailable: 0,
				Allocated:    buildResource("3000m", "3G"),
				TotalRequest: buildResource("4000m", "4G"),
				Tasks: tasksMap{
					case01_task1.UID: case01_task1,
					case01_task3.UID: case01_task3,
				},
				TaskStatusIndex: map[TaskStatus]tasksMap{
					Pending: {case01_task1.UID: case01_task1},
					Running: {case01_task3.UID: case01_task3},
				},
				NodeSelector: make(map[string]string),
			},
		},
		{
			name:   "add 2 pending owner pod, 1 running owner pod, remove 1 pending owner pod",
			uid:    case02_uid,
			pods:   []*v1.Pod{case02_pod1, case02_pod2, case02_pod3},
			rmPods: []*v1.Pod{case02_pod2},
			expected: &JobInfo{
				UID:          case02_uid,
				MinAvailable: 0,
				Allocated:    buildResource("3000m", "3G"),
				TotalRequest: buildResource("4000m", "4G"),
				Tasks: tasksMap{
					case02_task1.UID: case02_task1,
					case02_task3.UID: case02_task3,
				},
				TaskStatusIndex: map[TaskStatus]tasksMap{
					Pending: {
						case02_task1.UID: case02_task1,
					},
					Running: {
						case02_task3.UID: case02_task3,
					},
				},
				NodeSelector: make(map[string]string),
			},
		},
	}

	for i, test := range tests {
		ps := NewJobInfo(test.uid)

		for _, pod := range test.pods {
			pi := NewTaskInfo(pod)
			ps.AddTaskInfo(pi)
		}

		for _, pod := range test.rmPods {
			pi := NewTaskInfo(pod)
			ps.DeleteTaskInfo(pi)
		}

		if !jobInfoEqual(ps, test.expected) {
			t.Errorf("podset info %d: \n expected: %v, \n got: %v \n",
				i, test.expected, ps)
		}
	}
}

func TestCloneJob(t *testing.T) {
	case01UID := JobID("job_1")
	case01Ns := "c1"
	case01_owner := buildOwnerReference("uid")

	case01Pod1 := buildPod("c1", "p1", "", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01Pod2 := buildPod("c1", "p2", "n1", v1.PodRunning, buildResourceList("2000m", "2G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01Pod3 := buildPod("c1", "p3", "n1", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01Pod4 := buildPod("c1", "p4", "n1", v1.PodPending, buildResourceList("1000m", "1G"), []metav1.OwnerReference{case01_owner}, make(map[string]string))
	case01Task1 := NewTaskInfo(case01Pod1)
	case01Task2 := NewTaskInfo(case01Pod2)
	case01Task3 := NewTaskInfo(case01Pod3)
	case01Task4 := NewTaskInfo(case01Pod4)

	tests := []struct {
		name     string
		expected *JobInfo
	}{
		{
			name: "add 1 pending owner pod, 1 running owner pod",
			expected: &JobInfo{
				UID:          case01UID,
				Name:         "job_1",
				Namespace:    case01Ns,
				MinAvailable: 1,

				Allocated:    buildResource("4000m", "4G"),
				TotalRequest: buildResource("5000m", "5G"),
				Tasks: tasksMap{
					case01Task1.UID: case01Task1,
					case01Task2.UID: case01Task2,
					case01Task3.UID: case01Task3,
					case01Task4.UID: case01Task4,
				},
				TaskStatusIndex: map[TaskStatus]tasksMap{
					Running: {
						case01Task2.UID: case01Task2,
					},
					Pending: {
						case01Task1.UID: case01Task1,
					},
					Bound: {
						case01Task3.UID: case01Task3,
						case01Task4.UID: case01Task4,
					},
				},
			},
		},
	}

	for i, test := range tests {
		clone := test.expected.Clone()
		//assert.Equal(t, test.expected, clone)
		if !jobInfoEqual(clone, test.expected) {
			t.Errorf("clone info %d: \n expected: %v, \n got: %v \n", i, test.expected, clone)
		}
	}
}
