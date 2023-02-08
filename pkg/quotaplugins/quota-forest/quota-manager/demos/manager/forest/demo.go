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
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	fmt.Println("Demo of allocation and de-allocation of consumers on a forest using the quota manager.")
	fmt.Println()
	prefix := "../../../samples/forest/"
	indent := "===> "
	forestName := "Context-Service"
	treeNames := []string{"ContextTree", "ServiceTree"}

	// create a quota manager
	fmt.Println(indent + "Creating quota manager ... " + "\n")
	quotaManager := quota.NewManager()
	quotaManager.SetMode(quota.Normal)
	fmt.Println(quotaManager.GetModeString())
	fmt.Println()

	//	create multiple trees
	fmt.Println(indent + "Creating multiple trees ..." + "\n")
	for _, treeName := range treeNames {
		fName := prefix + treeName + ".json"
		fmt.Printf("Tree file name: %s\n", fName)
		jsonTree, err := ioutil.ReadFile(fName)
		if err != nil {
			fmt.Printf("error reading quota tree file: %s", fName)
			return
		}
		_, err = quotaManager.AddTreeFromString(string(jsonTree))
		if err != nil {
			fmt.Printf("error adding tree %s: %v", treeName, err)
			return
		}
	}

	// create forest
	fmt.Println(indent + "Creating forest " + forestName + " ..." + "\n")
	quotaManager.AddForest(forestName)
	for _, treeName := range treeNames {
		quotaManager.AddTreeToForest(forestName, treeName)
	}
	fmt.Println(quotaManager)

	//	create consumer jobs
	fmt.Println(indent + "Allocating consumers on forest ..." + "\n")
	jobs := []string{"job1", "job2", "job3", "job4", "job5"}
	for _, job := range jobs {

		// create consumer info
		fName := prefix + job + ".json"
		fmt.Printf("Consumer file name: %s\n", fName)
		consumerInfo, err := quota.NewConsumerInfoFromFile(fName)
		if err != nil {
			fmt.Printf("error reading consumer file: %s \n", fName)
			continue
		}
		consumerID := consumerInfo.GetID()

		// add consumer info to quota manager
		quotaManager.AddConsumer(consumerInfo)

		// allocate forest consumer instance of the consumer info
		allocResponse, err := quotaManager.AllocateForest(forestName, consumerID)
		if err != nil {
			fmt.Printf("error allocating consumer: %v \n", err)
			quotaManager.RemoveConsumer((consumerID))
			continue
		}

		// remove preemted consumers if any from quota manager (unless will be resubmitted)
		preemted := allocResponse.GetPreemptedIds()
		for _, pid := range preemted {
			quotaManager.RemoveConsumer(pid)
		}
	}

	// de-allocate consumers from forest
	fmt.Println(indent + "De-allocating consumers from forest ..." + "\n")
	for _, id := range quotaManager.GetAllConsumerIDs() {
		quotaManager.DeAllocateForest(forestName, id)
		quotaManager.RemoveConsumer(id)
	}
	fmt.Println()
	fmt.Println(quotaManager)
}
