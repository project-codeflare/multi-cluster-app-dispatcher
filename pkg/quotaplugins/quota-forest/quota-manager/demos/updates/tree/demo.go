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
	fmt.Println("===> Demonstrating dynamic tree updates")
	fmt.Println(separator)
	fmt.Println()

	// create tree
	treeCache := createTreeCache("../../../samples/ExampleTree.json")
	if treeCache == nil {
		fmt.Println("error parsing file")
		return
	}

	// create tree controller
	controller := createTreeControllerFromCache(treeCache)
	rNames := controller.GetResourceNames()

	// allocate consumer(s)
	allocateConsumer(controller, "../../../samples/ExampleConsumer.json", rNames)

	// update cache
	fmt.Println(separator)
	fmt.Println("===> Update cache")
	fmt.Println(separator)
	fmt.Println()

	fmt.Println("======> deleting node D")
	fmt.Println()
	treeCache.DeleteNode("D")
	refreshTreeFromCache(controller, treeCache)

	fmt.Println("======> renaming node C to CC")
	fmt.Println()
	treeCache.RenameNode("C", "CC")
	refreshTreeFromCache(controller, treeCache)

	fmt.Println("======> updating parent of G to B")
	fmt.Println()
	gStr := "{ \"G\": {\"parent\": \"B\", \"quota\": { \"cpu\": \"3\" } } }"
	fmt.Println(gStr)
	fmt.Println()
	fmt.Println("======> updating parent of H to A")
	fmt.Println()
	hStr := "{ \"H\": {\"parent\": \"A\", \"quota\": { \"cpu\": \"3\" } } }"
	fmt.Println(hStr)
	fmt.Println()
	fmt.Println("======> updating quota of B")
	fmt.Println()
	bStr := "{ \"B\": {\"parent\": \"A\", \"quota\": { \"cpu\": \"6\" } } }"
	fmt.Println(bStr)
	fmt.Println()
	treeCache.AddNodeSpecsFromString(gStr)
	treeCache.AddNodeSpecsFromString(hStr)
	treeCache.AddNodeSpecsFromString(bStr)
	refreshTreeFromCache(controller, treeCache)

	fmt.Println("======> deleting node K")
	fmt.Println()
	treeCache.DeleteNode("K")
	refreshTreeFromCache(controller, treeCache)

	fmt.Println("======> deleting node A")
	fmt.Println()
	treeCache.DeleteNode("A")
	refreshTreeFromCache(controller, treeCache)

	// de-allocate consumer(s)
	deallocateConsumer(controller, "C-1")
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

func createTreeControllerFromCache(treeCache *core.TreeCache) *core.Controller {
	fmt.Println(separator)
	fmt.Println("===> Create tree controller")
	fmt.Println(separator)
	fmt.Println()

	tree, response := treeCache.CreateTree()
	if !response.IsClean() {
		fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
	}
	return core.NewController(tree)
}

func allocateConsumer(controller *core.Controller, consumerSpecPath string, resourceNames []string) bool {
	fmt.Println(separator)
	fmt.Println("===> Allocate consumer")
	fmt.Println(separator)
	fmt.Println()

	consumer, err := createConsumer(consumerSpecPath, controller.GetTreeName(), resourceNames)
	fmt.Println(consumer)
	fmt.Println()
	if err != nil {
		fmt.Println("error creating consumer")
		return false
	}
	controller.Allocate(consumer)
	fmt.Println(consumer)
	fmt.Println()
	return true
}

func deallocateConsumer(controller *core.Controller, consumerID string) bool {
	fmt.Println(separator)
	fmt.Println("===> De-Allocate consumer")
	fmt.Println(separator)
	fmt.Println()

	consumer := controller.GetConsumer(consumerID)
	if consumer == nil {
		fmt.Println("error retrieving consumer " + consumerID)
		return false
	}
	controller.DeAllocate(consumerID)
	return true
}

func refreshTreeFromCache(controller *core.Controller, treeCache *core.TreeCache) {
	fmt.Println(separator)
	fmt.Println("===> Refresh cache")
	fmt.Println(separator)
	fmt.Println()

	unAllocated, response := controller.UpdateTree(treeCache)
	if !response.IsClean() {
		fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
	}
	fmt.Println(separator)
	fmt.Println("===> Updated tree")
	fmt.Println(separator)
	fmt.Println()
	fmt.Println(controller)
	fmt.Printf("unAllocated = %v \n", unAllocated)
	fmt.Println()
}

func createConsumer(fName string, treeName string, resourceNames []string) (*core.Consumer, error) {
	rNames := make(map[string][]string)
	rNames[treeName] = resourceNames
	forestConsumer, err := core.NewForestConsumerFromFile(fName, rNames)
	if err != nil {
		return nil, err
	}
	consumer := forestConsumer.GetTreeConsumer(treeName)
	if consumer == nil {
		return nil, fmt.Errorf("undefined tree %v for consumer %v", treeName, forestConsumer.GetID())
	}
	return consumer, nil
}
