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

	fmt.Println("Demo of allocation and de-allocation of a sequence of consumers on a single quota tree.")
	fmt.Println()
	prefix := "../../samples/tree/"

	treeCache := core.NewTreeCache()
	err := treeCache.FromFile(prefix + "tree.json")
	if err != nil {
		fmt.Println("error parsing file")
		return
	}
	tree, response := treeCache.CreateTree()
	if !response.IsClean() {
		fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
	}
	treeName := tree.GetName()
	rNames := treeCache.GetResourceNames()
	controller := core.NewController(tree)

	/*
	 *	run allocation/deallocation scenario
	 */

	ca, _ := createConsumer(prefix+"ca.json", treeName, rNames)
	controller.Allocate(ca)

	cb, _ := createConsumer(prefix+"cb.json", treeName, rNames)
	controller.Allocate(cb)

	cc, _ := createConsumer(prefix+"cc.json", treeName, rNames)
	controller.Allocate(cc)

	controller.DeAllocate(ca.GetID())

	cd, _ := createConsumer(prefix+"cd.json", treeName, rNames)
	controller.Allocate(cd)

	ce, _ := createConsumer(prefix+"ce.json", treeName, rNames)
	controller.Allocate(ce)

	cf, _ := createConsumer(prefix+"cf.json", treeName, rNames)
	controller.Allocate(cf)

	cg, _ := createConsumer(prefix+"cg.json", treeName, rNames)
	controller.Allocate(cg)

	ch, _ := createConsumer(prefix+"ch.json", treeName, rNames)
	controller.Allocate(ch)

	ci, _ := createConsumer(prefix+"ci.json", treeName, rNames)
	controller.Allocate(ci)

	cj, _ := createConsumer(prefix+"cj.json", treeName, rNames)
	controller.Allocate(cj)

	fmt.Println(controller)
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
