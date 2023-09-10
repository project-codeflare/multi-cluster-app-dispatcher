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
package app

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejob"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/health"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/metrics"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func buildConfig(master, kubeconfig string) (*rest.Config, error) {
	if master != "" || kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	return rest.InClusterConfig()
}

func Run(ctx context.Context, opt *options.ServerOption) error {
	config, err := buildConfig(opt.Master, opt.Kubeconfig)
	if err != nil {
		return err
	}

	config.QPS = 100.0
	config.Burst = 200.0

	jobctrl := queuejob.NewJobController(config, opt)
	if jobctrl == nil {
		return fmt.Errorf("failed to create a job controller")
	}

	go jobctrl.Run(ctx.Done())

	err = startHealthAndMetricsServers(ctx, opt)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func healthHandler() http.Handler {
	healthHandler := http.NewServeMux()
	healthHandler.Handle("/healthz", &health.Handler{})
	return healthHandler
}

// Starts the health probe listener
func startHealthAndMetricsServers(ctx context.Context, opt *options.ServerOption) error {
	g, ctx := errgroup.WithContext(ctx)

	// metrics server
	metricsServer, err := NewServer(opt.MetricsListenPort, "/metrics", metrics.Handler())
	if err != nil {
		return err
	}

	healthServer, err := NewServer(opt.HealthProbeListenPort, "/healthz", healthHandler())
	if err != nil {
		return err
	}

	g.Go(metricsServer.Start)
	g.Go(healthServer.Start)

	go func() {
		<-ctx.Done()
		metricsServer.Shutdown()
		healthServer.Shutdown()
	}()

	return nil
}
