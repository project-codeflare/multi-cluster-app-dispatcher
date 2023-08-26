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
package options

import (
	"flag"
	klog "k8s.io/klog/v2"
	"os"
	"strconv"
	"strings"
)

// ServerOption is the main context object for the controller manager.
type ServerOption struct {
	Master          string
	Kubeconfig      string
	SchedulerName   string
	Dispatcher      bool
	AgentConfigs    string
	SecurePort      int
	DynamicPriority bool // If DynamicPriority=true then no preemption is allowed by program logic
	Preemption      bool // Preemption is not allowed under DynamicPriority
	BackoffTime     int  // Number of seconds a job will go away for, if it can not be scheduled.  Default is 20.
	// Head of line job will not be bumped away for at least HeadOfLineHoldingTime seconds by higher priority jobs.
	// Default setting to 0 disables this mechanism.
	HeadOfLineHoldingTime int
	QuotaEnabled          bool	// Controller is to evaluate quota per request
	QuotaRestURL          string
	HealthProbeListenAddr string
	DispatchResourceReservationTimeout int64
	ExternalDispatch      bool  // if true, will use external plugin to dispatch workloads
}

// NewServerOption creates a new CMServer with a default config.
func NewServerOption() *ServerOption {
	s := ServerOption{}
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet
func (s *ServerOption) AddFlags(fs *flag.FlagSet) {
	// Set defaults via environment variables
	s.loadDefaultsFromEnvVars()

	fs.StringVar(&s.Master, "scheduler", s.SchedulerName, "scheduler name for placing pods")
	fs.StringVar(&s.Master, "master", s.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.BoolVar(&s.Dispatcher, "dispatcher", s.Dispatcher, "set dispather mode(true) or agent mode(false)")
	fs.StringVar(&s.AgentConfigs, "agentconfigs", s.AgentConfigs, "Paths to agent config file:deploymentName separted by commas(,)")
	fs.BoolVar(&s.DynamicPriority, "dynamicpriority", s.DynamicPriority, "If true, set controller to use dynamic priority. If false, set controller to use static priority.  Default is false.")
	fs.BoolVar(&s.Preemption, "preemption", s.Preemption, "Set controller to allow preemption if set to true. Note: when set to true, the Kubernetes Scheduler must be configured to enable preemption.  Default is false.")
	fs.IntVar(&s.BackoffTime, "backofftime", s.BackoffTime, "Number of seconds a job will go away for, if it can not be scheduled.  Default is 20.")
	fs.IntVar(&s.HeadOfLineHoldingTime, "headoflineholdingtime", s.HeadOfLineHoldingTime, "Number of seconds a job can stay at the Head Of Line without being bumped.  Default is 0.")
	fs.BoolVar(&s.QuotaEnabled,"quotaEnabled", s.QuotaEnabled,"Enable quota policy evaluation.  Default is false.")
	fs.StringVar(&s.QuotaRestURL, "quotaURL", s.QuotaRestURL, "URL for ReST quota management.  Default is none.")
	fs.IntVar(&s.SecurePort, "secure-port", 6443, "The port on which to serve secured, authenticated access for metrics.")
	fs.StringVar(&s.HealthProbeListenAddr, "healthProbeListenAddr", ":8081", "Listen address for health probes. Defaults to ':8081'")
	fs.Int64Var(&s.DispatchResourceReservationTimeout, "dispatchResourceReservationTimeout", s.DispatchResourceReservationTimeout, "Resource reservation timeout for pods to be created once AppWrapper is dispatched, in millisecond.  Defaults to '300000', 5 minutes")
	fs.BoolVar(&s.ExternalDispatch,"externalDispatch", s.ExternalDispatch,"Use external workload dispatch plugin.  Default is false.")
	
	flag.Parse()
	klog.V(4).Infof("[AddFlags] Controller configuration: %#v", s)
}

func (s *ServerOption) loadDefaultsFromEnvVars() {
	// Set defaults via environment variables
	s.AgentConfigs = os.Getenv("DISPATCHER_AGENT_CONFIGS")
	dispatcherMode, envVarExists := os.LookupEnv("DISPATCHER_MODE")

	s.Dispatcher = false
	if envVarExists && strings.EqualFold(dispatcherMode, "true") {
		s.Dispatcher = true
	}

	dynamicpriority, envVarExists := os.LookupEnv("DYNAMICPRIORITY")
	s.DynamicPriority = false
	if envVarExists && strings.EqualFold(dynamicpriority, "true") {
		s.DynamicPriority = true
	}

	preemption, envVarExists := os.LookupEnv("PREEMPTION")
	s.Preemption = false
	if envVarExists && strings.EqualFold(preemption, "true") {
		s.Preemption = true
	}

	backoffString, envVarExists := os.LookupEnv("BACKOFFTIME")
	s.BackoffTime = 20
	if envVarExists {
		backoffInt, err := strconv.Atoi(backoffString)
		if err == nil {
			s.BackoffTime = backoffInt
		}
	}

	holString, envVarExists := os.LookupEnv("HEADOFLINEHOLDINGTIME")
	s.HeadOfLineHoldingTime = 0
	if envVarExists {
		holInt, err := strconv.Atoi(holString)
		if err == nil {
			s.HeadOfLineHoldingTime = holInt
		}
	}

	enabledQuota, envVarExists := os.LookupEnv("QUOTA_ENABLED")
	s.QuotaEnabled = false
	if envVarExists && strings.EqualFold(enabledQuota, "true") {
		s.QuotaEnabled = true
	}

	quotaRestURLString, envVarExists := os.LookupEnv("QUOTA_REST_URL")
	s.QuotaRestURL = ""
	if envVarExists {
		s.QuotaRestURL = quotaRestURLString
	}

	dispatchResourceReservationTimeoutString, envVarExists := os.LookupEnv("DISPATCH_RESOURCE_RESERVATION_TIMEOUT")
	s.DispatchResourceReservationTimeout = 300000
	if envVarExists {
		to, err := strconv.ParseInt(dispatchResourceReservationTimeoutString, 10, 64)
		if err == nil {
			s.DispatchResourceReservationTimeout = to
		}
	}
	externalDispatch, envVarExists := os.LookupEnv("EXTERNAL_DISPATCH")
	s.ExternalDispatch = false
	if envVarExists && strings.EqualFold(externalDispatch, "true") {
		s.ExternalDispatch = true
	}

}

func (s *ServerOption) CheckOptionOrDie() {

}
