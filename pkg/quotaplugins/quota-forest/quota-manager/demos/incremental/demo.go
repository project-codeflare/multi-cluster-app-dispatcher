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

var (
	treeInfo string = "{ \"name\": \"ExampleTree\", \"resourceNames\": [ \"cpu\" ] }"

	nodeInfo1 string = "{ \"Context-1\": {\"parent\": \"Org-A\", \"hard\": \"true\", \"quota\": { \"cpu\": \"1\" } } }"

	nodeInfo2 string = "{ \"Root\": { \"parent\": \"nil\", \"quota\": { \"cpu\": \"10\" } }, \"Org-A\": {\"parent\": \"Root\", \"quota\": { \"cpu\": \"4\" } } }"

	nodeInfo3 string = "{ \"Context-2\": {\"parent\": \"Org-B\", \"quota\": { \"cpu\": \"2\" } } }"

	nodeInfo4 string = "{ \"Org-B\": {\"parent\": \"Root\", \"quota\": { \"cpu\": \"3\" } } }"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "4")
	flag.Set("skip_headers", "true")
	klog.SetOutput(os.Stdout)
	flag.Parse()
	defer klog.Flush()

	fmt.Println("Demo of building a quota tree incrementally.")
	fmt.Println()

	treeCache := core.NewTreeCache()

	/* process tree information */
	fmt.Printf("treeInfo = %s\n\n", treeInfo)
	err := treeCache.AddTreeInfoFromString(treeInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating tree: invalid tree info")
		return
	}

	/* process node information */
	fmt.Printf("nodeInfo1 = %s\n\n", nodeInfo1)
	err1 := treeCache.AddNodeSpecsFromString(nodeInfo1)
	fmt.Println()

	fmt.Printf("nodeInfo2 = %s\n\n", nodeInfo2)
	err2 := treeCache.AddNodeSpecsFromString(nodeInfo2)
	fmt.Println()

	fmt.Printf("nodeInfo3 = %s\n\n", nodeInfo3)
	err3 := treeCache.AddNodeSpecsFromString(nodeInfo3)
	fmt.Println()

	fmt.Printf("nodeInfo4 = %s\n\n", nodeInfo4)
	err4 := treeCache.AddNodeSpecsFromString(nodeInfo4)
	fmt.Println()

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		fmt.Fprintf(os.Stderr, "error creating tree: invalid nodes info")
		return
	}

	/* create tree */
	tree, response := treeCache.CreateTree()
	if !response.IsClean() {
		fmt.Printf("Warning: cache not clean after tree created: %v \n", response.String())
	}
	fmt.Println(tree)
}
