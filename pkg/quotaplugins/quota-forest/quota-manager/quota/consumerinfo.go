/*
Copyright 2022, 2023 The Multi-Cluster App Dispatcher Authors.

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

package quota

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/core"
	"github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
	"k8s.io/klog/v2"
)

// ConsumerInfo : A consumer model including specifications
type ConsumerInfo struct {
	// consumer specifications
	spec utils.JConsumerSpec
}

// NewConsumerInfo : create a new ConsumerInfo from Json struct
func NewConsumerInfo(consumerStruct utils.JConsumer) (*ConsumerInfo, error) {
	if consumerStruct.Kind != utils.DefaultConsumerKind {
		err := fmt.Errorf("consumer invalid kind: %s", consumerStruct.Kind)
		return nil, err
	}
	if len(consumerStruct.Spec.ID) == 0 {
		err := fmt.Errorf("missing consumer ID")
		return nil, err
	}
	consumerInfo := &ConsumerInfo{
		spec: consumerStruct.Spec,
	}
	return consumerInfo, nil
}

// NewConsumerInfoFromFile : create a new ConsumerInfo from Json file
func NewConsumerInfoFromFile(consumerFileName string) (*ConsumerInfo, error) {
	byteValue, err := os.ReadFile(consumerFileName)
	if err != nil {
		klog.Errorf("[NewConsumerInfoFromFile] unable to read file, - error: %#v", err)
		return nil, err
	}
	var consumerStruct utils.JConsumer
	if err := json.Unmarshal(byteValue, &consumerStruct); err != nil {
		klog.Errorf("[NewConsumerInfoFromFile] unable to unmarshal json, - error: %#v", err)
		return nil, err
	}
	return NewConsumerInfo(consumerStruct)
}

// NewConsumerInfoFromString : create a new ConsumerInfo from Json string
func NewConsumerInfoFromString(consumerString string) (*ConsumerInfo, error) {
	byteValue := []byte(consumerString)
	var consumerStruct utils.JConsumer
	if err := json.Unmarshal(byteValue, &consumerStruct); err != nil {
		klog.Errorf("[NewConsumerInfoFromString] unable to unmarshal json, - error: %#v", err)
		return nil, err
	}
	return NewConsumerInfo(consumerStruct)
}

// GetID : get the ID of the consumer
func (ci *ConsumerInfo) GetID() string {
	return ci.spec.ID
}

// CreateTreeConsumer : create a tree consumer for a given tree
func (ci *ConsumerInfo) CreateTreeConsumer(treeName string, resourceNames []string) (*core.Consumer, error) {
	consumerID := ci.spec.ID
	for _, spec := range ci.spec.Trees {
		if spec.TreeName == treeName {
			req := make([]int, len(resourceNames))
			for i, r := range resourceNames {
				req[i] = spec.Request[r]
			}
			alloc, _ := core.NewAllocationCopy(req)
			consumer := core.NewConsumer(consumerID, treeName, spec.GroupID, alloc, spec.Priority,
				spec.CType, spec.UnPreemptable)
			return consumer, nil
		}
	}
	return nil, fmt.Errorf("tree %s not specified in ConsumerInfo for consumer id %s", treeName, consumerID)
}

// CreateForestConsumer : create a forest consumer for a given forest
func (ci *ConsumerInfo) CreateForestConsumer(forestName string, resourceNames map[string][]string) (*core.ForestConsumer, error) {
	consumerID := ci.spec.ID
	consumers := make(map[string]*core.Consumer)
	for _, spec := range ci.spec.Trees {
		treeName := spec.TreeName
		if len(treeName) == 0 {
			continue
		}
		req := make([]int, len(resourceNames[treeName]))
		for i, r := range resourceNames[treeName] {
			req[i] = spec.Request[r]
		}
		alloc, _ := core.NewAllocationCopy(req)
		consumer := core.NewConsumer(consumerID, treeName, spec.GroupID, alloc, spec.Priority,
			spec.CType, spec.UnPreemptable)
		consumers[treeName] = consumer
	}
	fc := core.NewForestConsumer(consumerID, consumers)
	return fc, nil
}

// String : a print out of the consumer info
func (ci *ConsumerInfo) String() string {
	return fmt.Sprintf("Consumer: ID=%s; Spec={%v}", ci.spec.ID, ci.spec)
}
