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
package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	utils "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/quotaplugins/quota-forest/quota-manager/quota/utils"
)

// ForestConsumer : A forest consumer (multiple tree consumers)
type ForestConsumer struct {
	//  a unique id for the consumer
	id string
	// map of tree consumers: treeName -> consumer
	consumers map[string]*Consumer
}

// NewForestConsumer: create a forest consumer
func NewForestConsumer(id string, consumers map[string]*Consumer) *ForestConsumer {
	return &ForestConsumer{
		id:        id,
		consumers: consumers,
	}
}

// NewForestConsumerFromFile : create a forest consumer from JSON file
func NewForestConsumerFromFile(consumerFileName string, resourceNames map[string][]string) (*ForestConsumer, error) {
	byteValue, err := os.ReadFile(consumerFileName)
	if err != nil {
		return nil, fmt.Errorf("[NewForestConsumerFromFile] unable to read file, - error: %#v", err)
	}
	return NewForestConsumerFromString(string(byteValue), resourceNames)
}

// NewForestConsumerFromString : create a forest consumer from JSON string spec
func NewForestConsumerFromString(consumerString string, resourceNames map[string][]string) (*ForestConsumer, error) {
	byteValue := []byte(consumerString)
	var jConsumerMulti utils.JConsumer
	if err := json.Unmarshal(byteValue, &jConsumerMulti); err != nil {
		return nil, fmt.Errorf("[NewForestConsumerFromString] unable to unmarshal json, - error: %#v", err)
	}
	return NewForestConsumerFromStruct(jConsumerMulti, resourceNames)
}

// NewForestConsumerFromStruct : create a forest consumer from JSON struct
func NewForestConsumerFromStruct(consumerJson utils.JConsumer, resourceNames map[string][]string) (*ForestConsumer, error) {
	if consumerJson.Kind != utils.DefaultConsumerKind {
		err := fmt.Errorf("consumer multi invalid kind: %s", consumerJson.Kind)
		return nil, err
	}

	consumerID := consumerJson.Spec.ID
	if len(consumerID) == 0 {
		err := fmt.Errorf("missing consumer multi ID")
		return nil, err
	}

	consumers := make(map[string]*Consumer)
	for _, spec := range consumerJson.Spec.Trees {
		treeName := spec.TreeName
		if len(treeName) == 0 {
			continue
		}
		req := make([]int, len(resourceNames[treeName]))
		for i, r := range resourceNames[treeName] {
			req[i] = spec.Request[r]
		}
		alloc, _ := NewAllocationCopy(req)
		consumer := NewConsumer(consumerID, treeName, spec.GroupID, alloc, spec.Priority,
			spec.CType, spec.UnPreemptable)
		consumers[treeName] = consumer
	}
	return &ForestConsumer{
		id:        consumerID,
		consumers: consumers,
	}, nil
}

// GetID : get the id of the consumer
func (fc *ForestConsumer) GetID() string {
	return fc.id
}

// GetConsumers : get the tree consumers
func (fc *ForestConsumer) GetConsumers() map[string]*Consumer {
	return fc.consumers
}

// GetTreeConsumer : get the consumer of a given tree
func (fc *ForestConsumer) GetTreeConsumer(treeName string) *Consumer {
	return fc.consumers[treeName]
}

// IsAllocated : is consumer allocated on all trees in the forest
func (fc *ForestConsumer) IsAllocated() bool {
	for _, consumer := range fc.consumers {
		if !consumer.IsAllocated() {
			return false
		}
	}
	return true
}

// String : a print out of the forest consumer
func (fc *ForestConsumer) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "ForestConsumer: ID=%s {\n", fc.id)
	for treeName, consumer := range fc.consumers {
		fmt.Fprintf(&b, "treeName=%s; ", treeName)
		fmt.Fprintf(&b, "%s\n", consumer.String())
	}
	fmt.Fprint(&b, "}")
	return b.String()
}
