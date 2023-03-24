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

	core "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	fmt.Println("Demo of multiple quota trees.")
	fmt.Println()
	prefix := "../../samples/forest/"

	treeNames := []string{"ContextTree", "ServiceTree"}
	forestController := core.NewForestController()
	resourceNames := make(map[string][]string)
	treeCache := core.NewTreeCache()

	/*
	 *	create multiple trees
	 */
	for _, treeName := range treeNames {
		fName := prefix + treeName + ".json"
		fmt.Printf("Tree file name: %s\n", fName)

		err := treeCache.FromFile(fName)
		if err != nil {
			fmt.Println("Error creating tree " + treeName)
			return
		}
		tree, response := treeCache.CreateTree()
		if !response.IsClean() {
			fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
		}
		resourceNames[treeName] = treeCache.GetResourceNames()
		forestController.AddTree(tree)
		treeCache.Clear()
	}
	fmt.Println(forestController)

	/*
	 *	create consumer jobs
	 */
	jobs := []string{"job1", "job2", "job3", "job4", "job5"}
	ids := make([]string, len(jobs))
	for i, job := range jobs {
		fName := prefix + job + ".json"
		fmt.Printf("Consumer file name: %s\n", fName)
		forestConsumer, err := core.NewForestConsumerFromFile(fName, resourceNames)
		if err != nil {
			fmt.Println("Error creating consumers from " + fName)
		}
		forestController.Allocate(forestConsumer)
		ids[i] = forestConsumer.GetID()
	}

	fmt.Println(forestController)

	for _, id := range ids {
		if forestController.IsConsumerAllocated(id) {
			forestController.DeAllocate(id)
		}
	}

	fmt.Println(forestController)
}
