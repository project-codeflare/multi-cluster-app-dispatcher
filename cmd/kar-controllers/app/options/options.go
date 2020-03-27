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

package options

import (
	"github.com/spf13/pflag"
	"os"
	"strings"
)

// ServerOption is the main context object for the controller manager.
type ServerOption struct {
	Master		string
	Kubeconfig	string
	SchedulerName 	string
	Dispatcher	bool
	AgentConfigs 	string
	SecurePort	int
	DynamicPriority	bool  // If DynamicPriority=true then no preemption is allowed by program logic
	Preemption 	bool  // Preemption is not allowed under DynamicPriority
}

// NewServerOption creates a new CMServer with a default config.
func NewServerOption() *ServerOption {
	s := ServerOption{}
	return &s
}

// AddFlags adds flags for a specific CMServer to the specified FlagSet
func (s *ServerOption) AddFlags(fs *pflag.FlagSet) {
	// Set defaults via environment variables
	s.loadDefaultsFromEnvVars()

	fs.StringVar(&s.Master, "scheduler", s.SchedulerName, "scheduler name for placing pods")
	fs.StringVar(&s.Master, "master", s.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.BoolVar(&s.Dispatcher,"dispatcher",s.Dispatcher,"set dispather mode(true) or agent mode(false)")
	fs.StringVar(&s.AgentConfigs, "agentconfigs", s.AgentConfigs, "Paths to agent config file:deploymentName separted by commas(,)")
	fs.BoolVar(&s.DynamicPriority,"dynamicpriority", s.DynamicPriority,"If true set controller to use dynamic priority. If false, set controller to use static priority.  Default is false.")
	fs.BoolVar(&s.Preemption,"preemption", s.Preemption,"Set controller to allow preemption if set to true. Note: when set to true, the Kubernetes Scheduler must be configured to enable preemption.  Default is false.")
//	fs.IntVar(&s.SecurePort, "secure-port", 6443, "The port on which to serve secured, uthenticated access for metrics.")
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
}

func (s *ServerOption) CheckOptionOrDie() {

}
