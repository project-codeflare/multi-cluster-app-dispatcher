/*
Copyright 2018 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"os"
	"time"

	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppWrapper E2E Test", func() {

	/* 	It("Create AppWrapper - Generic 100 Deployment Only - 2 pods each", func() {
		context := initTestContext()
		defer cleanupTestContextExtendedTime(context, (300 * time.Second))

		const (
			awCount = 100
		)
		modDivisor := int(awCount / 10)
		replicas := 2
		var aws [awCount]*arbv1.AppWrapper
		for i := 0; i < awCount; i++ {
			name := fmt.Sprintf("%s%d-", "aw-generic-deployment-", replicas)
			if i < 99 {
				name = fmt.Sprintf("%s%s", name, "0")
			}
			if i < 9 {
				name = fmt.Sprintf("%s%s", name, "0")
			}
			name = fmt.Sprintf("%s%d", name, i+1)
			cpuDemand := "5m"
			if ((i+1)%modDivisor) == 0 || i == 0 {
				fmt.Fprintf(os.Stdout, "[e2e] Creating AW %s with %s cpu and %d replica(s).\n", name, cpuDemand, replicas)
			}
			aws[i] = createGenericDeploymentWithCPUAW(context, name, cpuDemand, replicas)
		}

		// Give the deployments time to create pods
		time.Sleep(2 * time.Minute)
		for i := 0; i < awCount; i++ {
			if ((i+1)%modDivisor) == 0 || i == 0 {
				fmt.Fprintf(os.Stdout, "[e2e] Checking for %d replicas running for AW %s.\n", replicas, aws[i].Name)
			}
			err := waitAWReadyQuiet(context, aws[i])
			Expect(err).NotTo(HaveOccurred())
		}
	}) */

	It("MCAD CPU Accounting Test", func() {
		context := initTestContext()

		// This should fill up the worker node and most of the master node
		aw := createDeploymentAWwith550CPU(context, "aw-deployment-2-550cpu")

		err := waitAWPodsReady(context, aw)
		Expect(err).NotTo(HaveOccurred())

		// This should fill up the master node
		aw2 := createDeploymentAWwith350CPU(context, "aw-deployment-2-350cpu")

		// Using quite mode due to creating of pods in earlier step.
		err = waitAWReadyQuiet(context, aw2)
		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		appwrappers = append(appwrappers, aw2)
		cleanupTestObjects(context, appwrappers)
		defer cleanupTestContextExtendedTime(context, (240 * time.Second))
	})

	It("Create AppWrapper - StatefulSet Only - 2 Pods", func() {
		context := initTestContext()

		aw := createStatefulSetAW(context, "aw-statefulset-2")

		err := waitAWPodsReady(context, aw)

		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
		defer cleanupTestContext(context)
	})

	It("Create AppWrapper - Generic StatefulSet Only - 2 Pods", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createGenericStatefulSetAW(context, "aw-generic-statefulset-2")

		err := waitAWPodsReady(context, aw)

		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper - Deployment Only - 3 pods", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createDeploymentAW(context, "aw-deployment-3")
		fmt.Fprintf(os.Stdout, "[e2e] Awaiting %d pods running for AW %s.\n", aw.Spec.SchedSpec.MinAvailable, aw.Name)
		err := waitAWPodsReady(context, aw)
		Expect(err).NotTo(HaveOccurred())

		// Now delete the appwrapper
		pods := getPodsOfAppWrapper(context, aw)
		fmt.Fprintf(os.Stdout, "[e2e] Deleting AW %s.\n", aw.Name)
		err = deleteAppWrapper(context, "aw-deployment-3")
		Expect(err).NotTo(HaveOccurred())

		// Wait for the pods of the deleted the appwrapper to be destroyed
		fmt.Fprintf(os.Stdout, "[e2e] Awaiting %d pods to be deleted for AW %s.\n", aw.Spec.SchedSpec.MinAvailable, aw.Name)
		err = waitAWDeleted(context, aw, pods)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Create AppWrapper - Generic Deployment Only - 3 pods", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createGenericDeploymentAW(context, "aw-generic-deployment-3")

		err := waitAWPodsReady(context, aw)
		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)

	})

	//NOTE: Recommend this test not to be the last test in the test suite it may pass
	//      the local test but may cause controller to fail which is not
	//      part of this test's validation.

	It("Create AppWrapper- Bad PodTemplate", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createBadPodTemplateAW(context, "aw-bad-podtemplate-2")

		err := waitAWPodsReady(context, aw)

		Expect(err).To(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper  - Bad Generic PodTemplate Only", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createBadGenericPodTemplateAW(context, "aw-generic-podtemplate-2")

		err := waitAWPodsReady(context, aw)

		Expect(err).To(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper  - PodTemplate Only - 2 Pods", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createPodTemplateAW(context, "aw-podtemplate-2")

		err := waitAWPodsReady(context, aw)

		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper  - Generic Pod Only - 1 Pod", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createGenericPodAW(context, "aw-generic-pod-1")

		err := waitAWPodsReady(context, aw)

		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper  - Bad Generic Pod Only", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createBadGenericPodAW(context, "aw-bad-generic-pod-1")

		err := waitAWPodsReady(context, aw)

		Expect(err).To(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper - Namespace Only - 0 Pods", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createNamespaceAW(context, "aw-namespace-0")

		err := waitAWNonComputeResourceActive(context, aw)

		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("Create AppWrapper - Generic Namespace Only - 0 Pods", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		aw := createGenericNamespaceAW(context, "aw-generic-namespace-0")

		err := waitAWNonComputeResourceActive(context, aw)

		Expect(err).NotTo(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		cleanupTestObjects(context, appwrappers)
	})

	It("MCAD CPU Accounting Fail Test", func() {
		context := initTestContext()
		defer cleanupTestContext(context)

		// This should fill up the worker node and most of the master node
		aw := createDeploymentAWwith550CPU(context, "aw-deployment-2-550cpu")

		err := waitAWPodsReady(context, aw)
		Expect(err).NotTo(HaveOccurred())

		// This should not fit on cluster
		aw2 := createDeploymentAWwith351CPU(context, "aw-deployment-2-351cpu")

		err = waitAWReadyQuiet(context, aw2)
		Expect(err).To(HaveOccurred())

		var appwrappers []*arbv1.AppWrapper
		appwrappers = append(appwrappers, aw)
		appwrappers = append(appwrappers, aw2)
		cleanupTestObjects(context, appwrappers)

	})
	/*
		It("Gang scheduling", func() {
			context := initTestContext()
			defer cleanupTestContext(context)
			rep := clusterSize(context, oneCPU)/2 + 1

			replicaset := createReplicaSet(context, "rs-1", rep, "nginx", oneCPU)
			err := waitReplicaSetReady(context, replicaset.Name)
			Expect(err).NotTo(HaveOccurred())

			job := &jobSpec{
				name:      "gang-qj",
				namespace: "test",
				tasks: []taskSpec{
					{
						img: "busybox",
						req: oneCPU,
						min: rep,
						rep: rep,
					},
				},
			}

			_, pg := createJobEx(context, job)
			err = waitPodGroupPending(context, pg)
			Expect(err).NotTo(HaveOccurred())

			waitPodGroupUnschedulable(context, pg)
			Expect(err).NotTo(HaveOccurred())

			err = deleteReplicaSet(context, replicaset.Name)
			Expect(err).NotTo(HaveOccurred())

			err = waitPodGroupReady(context, pg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Reclaim", func() {
			context := initTestContext()
			defer cleanupTestContext(context)

			slot := oneCPU
			rep := clusterSize(context, slot)

			job := &jobSpec{
				tasks: []taskSpec{
					{
						img: "nginx",
						req: slot,
						min: 1,
						rep: rep,
					},
				},
			}

			job.name = "q1-qj-1"
			job.queue = "q1"
			_, aw1 := createJobEx(context, job)
			err := waitAWPodsReady(context, aw1)
			Expect(err).NotTo(HaveOccurred())

			expected := int(rep) / 2
			// Reduce one pod to tolerate decimal fraction.
			if expected > 1 {
				expected--
			} else {
				err := fmt.Errorf("expected replica <%d> is too small", expected)
				Expect(err).NotTo(HaveOccurred())
			}

			job.name = "q2-qj-2"
			job.queue = "q2"
			_, aw2 := createJobEx(context, job)
			err = waitAWPodsReadyEx(context, aw2, expected)
			Expect(err).NotTo(HaveOccurred())

			err = waitAWPodsReadyEx(context, aw1, expected)
			Expect(err).NotTo(HaveOccurred())
		})
	*/
})
