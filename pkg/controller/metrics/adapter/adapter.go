/*
Copyright 2018 The Kubernetes Authors.

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

package adapter

import (
	"flag"
	"net/http"
	"os"

	"github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	adapterprov "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/metrics/adapter/provider"
	basecmd "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/metrics/cmd"
	"github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/metrics/provider"

	clusterstatecache "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/cache"
)

// New returns a Cache implementation.
func New(config *rest.Config, clusterStateCache clusterstatecache.Cache) *MetricsAdpater {
	return newMetricsAdpater(config, clusterStateCache)
}

type MetricsAdpater struct {
	basecmd.AdapterBase

	// Message is printed on succesful startup
	Message string
}

func (a *MetricsAdpater) makeProviderOrDie(clusterStateCache clusterstatecache.Cache) (provider.MetricsProvider, *restful.WebService) {
	klog.Infof("Entered makeProviderOrDie()")
	client, err := a.DynamicClient()
	if err != nil {
		klog.Fatalf("unable to construct dynamic client: %v", err)
	}

	mapper, err := a.RESTMapper()
	if err != nil {
		klog.Fatalf("unable to construct discovery REST mapper: %v", err)
	}

	return adapterprov.NewFakeProvider(client, mapper, clusterStateCache)
}

func newMetricsAdpater(config *rest.Config, clusterStateCache clusterstatecache.Cache) *MetricsAdpater {
	klog.V(10).Infof("Entered main()")

	cmd := &MetricsAdpater{}
	cmd.Flags().StringVar(&cmd.Message, "msg", "starting adapter...", "startup message")
	klog.Infof("")
	cmd.Flags().AddGoFlagSet(flag.CommandLine) // make sure we get the glog flags
	klog.V(9).Infof("commandline: %v", flag.CommandLine)
	cmd.Flags().Args()
	cmd.Flags().Parse(os.Args)

	testProvider, webService := cmd.makeProviderOrDie(clusterStateCache)
	cmd.WithCustomMetrics(testProvider)
	cmd.WithExternalMetrics(testProvider)

	klog.Infof(cmd.Message)
	// Set up POST endpoint for writing fake metric values
	restful.DefaultContainer.Add(webService)
	go func() {
		// Open port for POSTing fake metrics
		klog.Fatal(http.ListenAndServe(":8080", nil))
	}()
	//	if err := cmd.Run(wait.NeverStop); err != nil {
	//		klog.Fatalf("unable to run custom metrics adapter: %v", err)
	//	}
	go cmd.Run(wait.NeverStop)
	return cmd
}
