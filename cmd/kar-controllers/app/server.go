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
	"strings"
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejob"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/health"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/utils/pointer"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/config"
)

func buildConfig(master, kubeconfig string) (*rest.Config, error) {
	if master != "" || kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	return rest.InClusterConfig()
}

func Run(ctx context.Context, opt *options.ServerOption) error {
	restConfig, err := buildConfig(opt.Master, opt.Kubeconfig)
	if err != nil {
		return err
	}

	restConfig.QPS = 100.0
	restConfig.Burst = 200.0

	mcadConfig := &config.MCADConfiguration{
		DynamicPriority:       pointer.Bool(opt.DynamicPriority),
		Preemption:            pointer.Bool(opt.Preemption),
		BackoffTime:           pointer.Int32(int32(opt.BackoffTime)),
		HeadOfLineHoldingTime: pointer.Int32(int32(opt.HeadOfLineHoldingTime)),
		QuotaEnabled:          &opt.QuotaEnabled,
	}
	extConfig := &config.MCADConfigurationExtended{
		Dispatcher:   pointer.Bool(opt.Dispatcher),
		AgentConfigs: strings.Split(opt.AgentConfigs, ","),
	}

	jobctrl := queuejob.NewJobController(restConfig, mcadConfig, extConfig)
	if jobctrl == nil {
		return fmt.Errorf("failed to create a job controller")
	}

	stopCh := make(chan struct{})
    // this channel is used to signal that the job controller is done
    jobctrlDoneCh := make(chan struct{})

	go func() {
        defer close(stopCh)
		<-ctx.Done()
	}()

	go func() {
     jobctrl.Run(stopCh)
    // close the jobctrlDoneCh channel when the job controller is done
     close(jobctrlDoneCh)
    }()

    // wait for the job controller to be done before shutting down the server
    <-jobctrlDoneCh

	err = startHealthAndMetricsServers(ctx, opt)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

// Starts the health probe listener
func startHealthAndMetricsServers(ctx context.Context, opt *options.ServerOption) error {
    
    // Create a new registry.
	reg := prometheus.NewRegistry()

	// Add Go module build info.
	reg.MustRegister(collectors.NewBuildInfoCollector())
	reg.MustRegister(collectors.NewGoCollector())
    reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

    metricsHandler := http.NewServeMux()

    // Use the HTTPErrorOnError option for the Prometheus handler
	handlerOpts := promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	}

	metricsHandler.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, handlerOpts))

    healthHandler := http.NewServeMux()
    healthHandler.Handle("/healthz", &health.Handler{})

    metricsServer := &http.Server{
        Addr:    opt.MetricsListenAddr,
        Handler: metricsHandler,
    }

    healthServer := &http.Server{
        Addr:    opt.HealthProbeListenAddr,
        Handler: healthHandler,
    }

    // make a channel for errors for each server
    metricsServerErrChan := make(chan error)
    healthServerErrChan := make(chan error)

    // start servers in their own goroutines
    go func() {
        defer close(metricsServerErrChan)
        err := metricsServer.ListenAndServe()
        if err != nil && err != http.ErrServerClosed {
            metricsServerErrChan <- err
        }
    }()

    go func() {
        defer close(healthServerErrChan)
        err := healthServer.ListenAndServe()
        if err != nil && err != http.ErrServerClosed {
            healthServerErrChan <- err
        }
    }()

    // use select to wait for either a shutdown signal or an error
    select {
    case <-ctx.Done():
        // received an OS shutdown signal, shut down servers gracefully
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

  		errM := metricsServer.Shutdown(ctx)
		if errM != nil {
			return fmt.Errorf("metrics server shutdown error: %v", errM)
		}
   		errH := healthServer.Shutdown(ctx)
			if errH != nil {
			return fmt.Errorf("health server shutdown error: %v", errH)
		}
    case err := <-metricsServerErrChan:
        return fmt.Errorf("metrics server error: %v", err)
    case err := <-healthServerErrChan:
        return fmt.Errorf("health server error: %v", err)
    }

    return nil
}
