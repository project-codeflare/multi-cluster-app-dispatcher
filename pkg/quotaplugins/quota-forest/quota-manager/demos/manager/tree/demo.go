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
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"k8s.io/klog/v2"
)

const (
	prefix = "../../../samples/tree/"
)

var (
	quotaManager *quota.Manager
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	fmt.Println("Demo of allocation and de-allocation of consumers on a tree using the quota manager.")
	fmt.Println()
	treeFileName := prefix + "tree.json"

	// create a quota manager
	quotaManager = quota.NewManager()
	quotaManager.SetMode(quota.Normal)
	fmt.Println(quotaManager.GetModeString())

	// add a quota tree from file
	jsonTree, err := ioutil.ReadFile(treeFileName)
	if err != nil {
		fmt.Printf("error reading quota tree file: %s", treeFileName)
		return
	}
	treeName, err := quotaManager.AddTreeFromString(string(jsonTree))
	if err != nil {
		fmt.Printf("error adding tree %s: %v", treeName, err)
		return
	}

	// allocate consumers from files
	ar := createAndAllocateConsumer("ca", treeName)
	caID := ar.GetConsumerID()

	createAndAllocateConsumer("cb", treeName)
	createAndAllocateConsumer("cc", treeName)

	quotaManager.DeAllocate(treeName, caID)
	quotaManager.RemoveConsumer(caID)

	createAndAllocateConsumer("cd", treeName)
	createAndAllocateConsumer("ce", treeName)
	createAndAllocateConsumer("cf", treeName)
	createAndAllocateConsumer("cg", treeName)
	createAndAllocateConsumer("ch", treeName)
	createAndAllocateConsumer("ci", treeName)
	createAndAllocateConsumer("cj", treeName)

	fmt.Println(quotaManager)
}

func createAndAllocateConsumer(consumerFile string, treeName string) (allocResponse *core.AllocationResponse) {
	// create consumer info
	consumerFileName := prefix + consumerFile + ".json"
	consumerInfo, err := quota.NewConsumerInfoFromFile(consumerFileName)
	if err != nil {
		fmt.Printf("error reading consumer file: %s \n", consumerFileName)
		os.Exit(1)
	}
	consumerID := consumerInfo.GetID()

	// add consumer info to quota manager
	quotaManager.AddConsumer(consumerInfo)

	// allocate a tree consumer instance of the consumer info
	allocResponse, err = quotaManager.Allocate(treeName, consumerID)
	if err != nil {
		fmt.Printf("error allocating consumer: %v \n", err)
		os.Exit(1)
	}

	// remove preemted consumers if any from quota manager (unless will be resubmitted)
	preemted := allocResponse.GetPreemptedIds()
	for _, pid := range preemted {
		quotaManager.RemoveConsumer(pid)
	}
	return allocResponse
}
