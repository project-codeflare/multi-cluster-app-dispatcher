/*
Copyright 2019 The Kubernetes Authors.

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
package queuejob

import (
	"fmt"
        corev1 "k8s.io/api/core/v1"
        apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
        apierrors "k8s.io/apimachinery/pkg/api/errors"
        "k8s.io/apimachinery/pkg/api/meta"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"

        arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
        "github.com/IBM/multi-cluster-app-dispatcher/pkg/client/clientset/controller-versioned/clients"
)

var queueJobKind = arbv1.SchemeGroupVersion.WithKind("QueueJob")

var appwrapperJobKind = arbv1.SchemeGroupVersion.WithKind("AppWrapper")

// GetPodFullName returns a name that uniquely identifies a qj.
func GetQJFullName(qj *arbv1.QueueJob) string {
	// Use underscore as the delimiter because it is not allowed in qj name
	// (DNS subdomain format).
	return qj.Name + "_" + qj.Namespace
}

func GetXQJFullName(qj *arbv1.AppWrapper) string {
        // Use underscore as the delimiter because it is not allowed in qj name
        // (DNS subdomain format).
        return qj.Name + "_" + qj.Namespace
}

func HigherPriorityQJ(qj1, qj2 interface{} ) bool {
	return (qj1.(*arbv1.AppWrapper).Spec.Priority > qj2.(*arbv1.AppWrapper).Spec.Priority)
}

func HigherSystemPriorityQJ(qj1, qj2 interface{} ) bool {
	return (qj1.(*arbv1.AppWrapper).Status.SystemPriority > qj2.(*arbv1.AppWrapper).Status.SystemPriority)
}

func generateUUID() string {
	id := uuid.NewUUID()

	return fmt.Sprintf("%s", id)
}

// getStatus returns no of succeeded and failed pods running a job
func getStatus(pods []*corev1.Pod) (succeeded, failed int32) {
	succeeded = int32(filterPods(pods, corev1.PodSucceeded))
	failed = int32(filterPods(pods, corev1.PodFailed))
	return
}

// filterPods returns pods based on their phase.
func filterPods(pods []*corev1.Pod, phase corev1.PodPhase) int {
	result := 0
	for i := range pods {
		if phase == pods[i].Status.Phase {
			result++
		}
	}
	return result
}

func eventKey(obj interface{}) (string, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}

	return string(accessor.GetUID()), nil
}

func createQueueJobSchedulingSpec(qj *arbv1.QueueJob) *arbv1.SchedulingSpec {
	return &arbv1.SchedulingSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      qj.Name,
			Namespace: qj.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(qj, queueJobKind),
			},
		},
		Spec: qj.Spec.SchedSpec,
	}
}

func createXQueueJobSchedulingSpec(qj *arbv1.AppWrapper) *arbv1.SchedulingSpec {
        return &arbv1.SchedulingSpec{
                ObjectMeta: metav1.ObjectMeta{
                        Name:      qj.Name,
                        Namespace: qj.Namespace,
                        OwnerReferences: []metav1.OwnerReference{
                                *metav1.NewControllerRef(qj, queueJobKind),
                        },
                },
                Spec: qj.Spec.SchedSpec,
        }
}


func createQueueJobPod(qj *arbv1.QueueJob, template *corev1.PodTemplateSpec, ix int32) *corev1.Pod {
	templateCopy := template.DeepCopy()

	prefix := fmt.Sprintf("%s-", qj.Name)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: prefix,
			Namespace:    qj.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(qj, queueJobKind),
			},
			Labels: templateCopy.Labels,
		},
		Spec: templateCopy.Spec,
	}
	// we fill the schedulerName in the pod definition with the one specified in the QJ template
	if qj.Spec.SchedulerName != "" && pod.Spec.SchedulerName == "" {
		pod.Spec.SchedulerName = qj.Spec.SchedulerName
	}
	return pod
}

func createQueueJobKind(config *rest.Config) error {
	extensionscs, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = clients.CreateQueueJobKind(extensionscs)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func createAppWrapperKind(config *rest.Config) error {
	extensionscs, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = clients.CreateAppWrapperKind(extensionscs)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// AppWrapperCondition returns condition of a AppWrapper condition.
func GenerateAppWrapperCondition(condType arbv1.AppWrapperConditionType, condStatus corev1.ConditionStatus, condReason string, condMsg string) arbv1.AppWrapperCondition {
	return arbv1.AppWrapperCondition{
		Type:    condType,
		Status:  condStatus,
		LastUpdateMicroTime:  metav1.NowMicro(),
		LastTransitionMicroTime: metav1.NowMicro(),
		Reason:  condReason,
		Message: condMsg,
	}
}

// AppWrapperCondition returns condition of a AppWrapper condition.
func isLastConditionDuplicate(aw *arbv1.AppWrapper, condType arbv1.AppWrapperConditionType, condStatus corev1.ConditionStatus, condReason string, condMsg string) bool {
	if (aw.Status.Conditions == nil) {
		return false
	}

	lastIndex := len(aw.Status.Conditions) - 1

	if (lastIndex < 0) {
		return false
	}

	lastCond := aw.Status.Conditions[lastIndex]
	if (lastCond.Type == condType) &&
		(lastCond.Status == condStatus) &&
		(lastCond.Reason == condReason) &&
		(lastCond.Message == condMsg) {
		return true
	} else {
		return false
	}
}