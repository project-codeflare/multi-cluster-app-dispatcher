/*
Copyright 2017 The Kubernetes Authors.

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
/*
Copyright 2019, 2021 The Multi-Cluster App Dispatcher Authors.

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

	"k8s.io/klog/v2"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"

	"k8s.io/apiserver/pkg/server"
)

func main() {
	s := options.NewServerOption()

	flagSet := flag.CommandLine
	klog.InitFlags(flagSet)
	s.AddFlags(flagSet)
	flag.Parse()

	ctx := server.SetupSignalContext()

	// Run the server
	if err := app.Run(ctx, s); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	<-ctx.Done()
	fmt.Println("Shutting down gracefully")
	
}
