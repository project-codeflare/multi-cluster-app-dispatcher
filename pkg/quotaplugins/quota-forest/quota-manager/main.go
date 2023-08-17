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
	"os"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota"
	klog "k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	treeFileName := "samples/TestTree.json"
	consumerFileName := "samples/TestConsumer.json"

	// create a quota manager
	fmt.Println("==> Creating Quota Manager")
	fmt.Println("**************************")
	quotaManager := quota.NewManager()
	treeJsonString, err := os.ReadFile(treeFileName)
	if err != nil {
		fmt.Printf("error reading quota tree file: %s", treeFileName)
		return
	}

	// set mode of quota manager
	quotaManager.SetMode(quota.Normal)

	// add a quota tree from file
	treeName, err := quotaManager.AddTreeFromString(string(treeJsonString))
	if err != nil {
		fmt.Printf("error adding tree %s: %v", treeName, err)
		return
	}

	// allocate a consumer from file
	fmt.Println("==> Allocating a consumer")
	fmt.Println("**************************")
	consumerInfo, err := quota.NewConsumerInfoFromFile(consumerFileName)
	if err != nil {
		fmt.Printf("error reading consumer file: %s", consumerFileName)
		return
	}
	consumerID := consumerInfo.GetID()
	quotaManager.AddConsumer(consumerInfo)
	allocResponse, err := quotaManager.Allocate(treeName, consumerID)
	if err != nil {
		fmt.Printf("error allocating consumer: %v", err)
		return
	}
	fmt.Println(allocResponse)
	fmt.Println(quotaManager)

	// deallocate consumer
	fmt.Println("==> DeAllocating consumer")
	fmt.Println("**************************")
	quotaManager.DeAllocate(treeName, consumerID)
	quotaManager.RemoveConsumer(consumerID)

	fmt.Println(quotaManager)
}
