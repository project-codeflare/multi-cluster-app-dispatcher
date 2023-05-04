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
	"strings"

	core "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"k8s.io/klog/v2"
)

var separator string = strings.Repeat("*", 32)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	fmt.Println(separator)
	fmt.Println("===> Demonstrating dynamic multi-tree updates")
	fmt.Println(separator)
	fmt.Println()

	// create trees
	prefix := "../../../samples/forest/"
	treeNames := []string{"ContextTree", "ServiceTree"}
	numTrees := len(treeNames)
	treeCaches := make([]*core.TreeCache, numTrees)

	// create forest manager
	for i, treeName := range treeNames {
		fName := prefix + treeName + ".json"
		treeCache := createTreeCache(fName)
		if treeCache == nil {
			fmt.Println("error parsing file")
			return
		}
		treeCaches[i] = treeCache
	}
	forestController := createForestControllerFromCaches(treeCaches)
	rNamesMap := forestController.GetResourceNames()

	// 	// allocate consumer(s)
	job := "job1"
	fName := prefix + job + ".json"
	allocateConsumer(forestController, fName, rNamesMap)

	// update cache
	fmt.Println(separator)
	fmt.Println("===> Update cache")
	fmt.Println(separator)
	fmt.Println()

	fmt.Println("======> deleting node Srvc-Z")
	fmt.Println()
	treeCaches[1].DeleteNode("Srvc-Z")
	refreshTreeFromCache(forestController, treeCaches)

	fmt.Println("======> renaming node Srvc-X to Srvc-XX")
	fmt.Println()
	treeCaches[1].RenameNode("Srvc-X", "Srvc-XX")
	refreshTreeFromCache(forestController, treeCaches)

	fmt.Println("======> updating parent of Org-B to Org-A")
	fmt.Println()
	bStr := "{ \"Org-B\": {\"parent\": \"Org-A\", \"quota\": { \"cpu\": \"6\" } } }"
	fmt.Println(bStr)
	fmt.Println()
	fmt.Println("======> updating quota of Org-A")
	fmt.Println()
	aStr := "{ \"Org-A\": {\"parent\": \"Root\", \"quota\": { \"cpu\": \"8\" } } }"
	fmt.Println(aStr)
	fmt.Println()
	treeCaches[0].AddNodeSpecsFromString(bStr)
	treeCaches[0].AddNodeSpecsFromString(aStr)
	refreshTreeFromCache(forestController, treeCaches)

	fmt.Println("======> deleting node Context-4")
	fmt.Println()
	treeCaches[0].DeleteNode("Context-4")
	refreshTreeFromCache(forestController, treeCaches)

	fmt.Println("======> deleting node Root")
	fmt.Println()
	treeCaches[1].DeleteNode("Root")
	refreshTreeFromCache(forestController, treeCaches)

	// de-allocate consumer(s)
	deallocateConsumer(forestController, "C-1")
}

func createTreeCache(TreeSpecPath string) *core.TreeCache {
	fmt.Println(separator)
	fmt.Println("===> Create tree in cache")
	fmt.Println(separator)
	fmt.Println()

	treeCache := core.NewTreeCache()
	err := treeCache.FromFile(TreeSpecPath)
	if err != nil {
		return nil
	}
	return treeCache
}

func createForestControllerFromCaches(treeCaches []*core.TreeCache) *core.ForestController {
	fmt.Println(separator)
	fmt.Println("===> Create forest controller")
	fmt.Println(separator)
	fmt.Println()

	forestController := core.NewForestController()
	for _, tc := range treeCaches {
		tree, response := tc.CreateTree()
		if !response.IsClean() {
			fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
		}
		forestController.AddTree(tree)
	}
	fmt.Println(forestController)
	return forestController
}

func allocateConsumer(cm *core.ForestController, consumerSpecPath string, resourceNamesMap map[string][]string) bool {
	fmt.Println(separator)
	fmt.Println("===> Allocate consumer")
	fmt.Println(separator)
	fmt.Println()

	forestConsumer, err := core.NewForestConsumerFromFile(consumerSpecPath, resourceNamesMap)
	if err != nil {
		fmt.Println("Error creating consumer")
		return false
	}
	fmt.Println(forestConsumer)
	fmt.Println()
	cm.Allocate(forestConsumer)
	fmt.Println(forestConsumer)
	fmt.Println()
	return true
}

func deallocateConsumer(cm *core.ForestController, consumerID string) bool {
	fmt.Println(separator)
	fmt.Println("===> De-Allocate consumer")
	fmt.Println(separator)
	fmt.Println()

	if !cm.IsConsumerAllocated(consumerID) {
		fmt.Println("error retrieving consumer " + consumerID)
		return false
	}
	cm.DeAllocate(consumerID)
	return true
}

func refreshTreeFromCache(cm *core.ForestController, treeCaches []*core.TreeCache) {
	fmt.Println(separator)
	fmt.Println("===> Refresh cache")
	fmt.Println(separator)
	fmt.Println()

	unAllocated, responseMap := cm.UpdateTrees(treeCaches)
	for _, response := range responseMap {
		if !response.IsClean() {
			fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
		}
	}

	fmt.Println(separator)
	fmt.Println("===> Updated tree")
	fmt.Println(separator)
	fmt.Println()
	fmt.Println(cm)
	fmt.Printf("unAllocated = %v \n", unAllocated)
	fmt.Println()
}
