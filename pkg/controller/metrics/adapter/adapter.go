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

package adapter

import (
	"flag"
	"net/http"
	"os"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/emicklei/go-restful"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	generatedcustommetrics "sigs.k8s.io/custom-metrics-apiserver/pkg/generated/openapi/custommetrics"

	adapterprov "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/metrics/adapter/provider"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/apiserver"
	basecmd "sigs.k8s.io/custom-metrics-apiserver/pkg/cmd"
	"sigs.k8s.io/custom-metrics-apiserver/pkg/provider"

	clusterstatecache "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/cache"
)

// New returns a Cache implementation.
func New(serverOptions *options.ServerOption, config *rest.Config, clusterStateCache clusterstatecache.Cache) *MetricsAdapter {
	return newMetricsAdapter(serverOptions, config, clusterStateCache)
}

type MetricsAdapter struct {
	basecmd.AdapterBase

	// Message is printed on succesful startup
	Message string
}

func (a *MetricsAdapter) makeProviderOrDie(clusterStateCache clusterstatecache.Cache) (provider.MetricsProvider, *restful.WebService) {
	klog.Infof("[makeProviderOrDie] Entered makeProviderOrDie()")
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

func covertServerOptionsToMetricsServerOptions(serverOptions *options.ServerOption) []string {
	var portedArgs = make([]string, 0)
	if serverOptions == nil {
		return portedArgs
	}

	if len(serverOptions.Kubeconfig) > 0 {
		kubeConfigArg := "--lister-kubeconfig=" + serverOptions.Kubeconfig
		portedArgs = append(portedArgs, kubeConfigArg)
	}
	return portedArgs
}

func newMetricsAdapter(serverOptions *options.ServerOption, config *rest.Config, clusterStateCache clusterstatecache.Cache) *MetricsAdapter {
	klog.V(10).Infof("[newMetricsAdapter] Entered newMetricsAdapter()")

	cmd := &MetricsAdapter{}

	cmd.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedcustommetrics.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(apiserver.Scheme))
	cmd.OpenAPIConfig.Info.Title = "MetricsAdpater"
	cmd.OpenAPIConfig.Info.Version = "1.0.0"

	cmd.Flags().StringVar(&cmd.Message, "msg", "starting metrics adapter...", "startup message")
	cmd.Flags().AddGoFlagSet(flag.CommandLine) // make sure we get the klog flags
	klog.V(10).Infof("[newMetricsAdapter] Go flag set from commandline: %+v", flag.CommandLine)
	klog.V(10).Infof("[newMetricsAdapter] Flag arguments: %+v", cmd.Flags().Args())
	cmd.Flags().Parse(os.Args)

	// The metrics server thread requires a different flag name than the primary server, e.g. primary
	// server uses --kubeconfig but metrics server uses --lister-kubeconfig
	portedArgs := covertServerOptionsToMetricsServerOptions(serverOptions)
	cmd.Flags().Parse(portedArgs)

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
	go cmd.Run(wait.NeverStop)
	return cmd
}
