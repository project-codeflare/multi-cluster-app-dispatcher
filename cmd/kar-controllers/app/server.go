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
	"fmt"
	"net/http"
	"strings"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/cmd/kar-controllers/app/options"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/config"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/queuejob"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/health"
)

func buildConfig(master, kubeconfig string) (*rest.Config, error) {
	if master != "" || kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	return rest.InClusterConfig()
}

func Run(opt *options.ServerOption) error {
	restConfig, err := buildConfig(opt.Master, opt.Kubeconfig)
	if err != nil {
		return fmt.Errorf("[Run] unable to build server config, - error: %w", err)
	}

	neverStop := make(chan struct{})

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
		return nil
	}
	jobctrl.Run(neverStop)

	// This call is blocking (unless an error occurs) which equates to <-neverStop
	err = listenHealthProbe(opt)
	if err != nil {
		return fmt.Errorf("[Run] unable to start health probe listener, - error: %w", err)

	}

	return nil
}

// Starts the health probe listener
func listenHealthProbe(opt *options.ServerOption) error {
	handler := http.NewServeMux()
	handler.Handle("/healthz", &health.Handler{})
	err := http.ListenAndServe(opt.HealthProbeListenAddr, handler)
	if err != nil {
		return fmt.Errorf("[listenHealthProbe] unable to listen and serve, - error: %w", err)
	}

	return nil
}
