/*
Copyright 2023 The Multi-Cluster App Dispatcher Authors.

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
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	klog "k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	prefix := "../../../samples/tree/"
	treeFileName := prefix + "tree.json"
	caFileName := prefix + "ca.json"
	cbFileName := prefix + "cb.json"
	ccFileName := prefix + "cc.json"
	cdFileName := prefix + "cd.json"
	ceFileName := prefix + "ce.json"

	// create a quota manager
	fmt.Println("==> Creating Quota Manager")
	fmt.Println("**************************")
	quotaManager := quota.NewManager()
	treeJsonString, err := os.ReadFile(treeFileName)
	if err != nil {
		fmt.Printf("error reading quota tree file: %s", treeFileName)
		return
	}
	quotaManager.SetMode(quota.Normal)

	// add a quota tree from file
	treeName, err := quotaManager.AddTreeFromString(string(treeJsonString))
	if err != nil {
		fmt.Printf("error adding tree %s: %v", treeName, err)
		return
	}

	// allocate consumers
	allocate(quotaManager, treeName, caFileName, false)
	allocate(quotaManager, treeName, cbFileName, false)
	allocate(quotaManager, treeName, ccFileName, false)

	// try and undo allocation
	allocate(quotaManager, treeName, cdFileName, true)
	undoAllocate(quotaManager, treeName, cdFileName)

	// allocate consumers
	allocate(quotaManager, treeName, ceFileName, false)
}

// allocate consumer from file
func allocate(quotaManager *quota.Manager, treeName string, consumerFileName string, try bool) {
	consumerInfo := getConsumerInfo(consumerFileName)
	if consumerInfo == nil {
		fmt.Printf("error reading consumer file: %s", consumerFileName)
		return
	}
	consumerID := consumerInfo.GetID()
	fmt.Println("==> Allocating consumer " + consumerID)
	fmt.Println("**************************")
	quotaManager.AddConsumer(consumerInfo)

	var allocResponse *core.AllocationResponse
	var err error
	if try {
		allocResponse, err = quotaManager.TryAllocate(treeName, consumerID)
	} else {
		allocResponse, err = quotaManager.Allocate(treeName, consumerID)
	}
	if err != nil {
		fmt.Printf("error allocating consumer: %v", err)
		return
	}
	fmt.Println(allocResponse)
	fmt.Println(quotaManager)
}

// undo most recent consumer allocation
func undoAllocate(quotaManager *quota.Manager, treeName string, consumerFileName string) {
	consumerInfo := getConsumerInfo(consumerFileName)
	if consumerInfo == nil {
		fmt.Printf("error reading consumer file: %s", consumerFileName)
		return
	}
	consumerID := consumerInfo.GetID()
	fmt.Println("==> Undo allocating consumer " + consumerID)
	fmt.Println("**************************")
	quotaManager.UndoAllocate(treeName, consumerID)
	fmt.Println(quotaManager)
}

// get consumer info from yaml file
func getConsumerInfo(consumerFileName string) *quota.ConsumerInfo {
	consumerInfo, err := quota.NewConsumerInfoFromFile(consumerFileName)
	if err != nil {
		fmt.Printf("error reading consumer file: %s", consumerFileName)
		return nil
	}
	return consumerInfo
}
