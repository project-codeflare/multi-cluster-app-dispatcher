/*
Copyright 2022 The Multi-Cluster App Dispatcher Authors.

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

package utils

// JQuotaTree : JSON quota tree
type JQuotaTree struct {
	Kind     string    `json:"kind"`
	MetaData JMetaData `json:"metadata"`
	Spec     JTreeSpec `json:"spec"`
}

// JMetaData : common metada
type JMetaData struct {
	Name string `json:"name"`
}

// JTreeSpec : spec of quota tree
type JTreeSpec struct {
	ResourceNames []string             `json:"resourceNames"`
	Nodes         map[string]JNodeSpec `json:"nodes"`
}

// JNodeSpec : spec for a node in the quota tree
type JNodeSpec struct {
	Parent string            `json:"parent"`
	Quota  map[string]string `json:"quota"`
	Hard   string            `json:"hard"`
}

// JTreeInfo : data about tree name and resource names
type JTreeInfo struct {
	Name          string   `json:"name"`
	ResourceNames []string `json:"resourceNames"`
}

// JConsumer : JSON consumer
type JConsumer struct {
	Kind     string        `json:"kind"`
	MetaData JMetaData     `json:"metadata"`
	Spec     JConsumerSpec `json:"spec"`
}

// JConsumerSpec : spec of consumer of multiple trees
type JConsumerSpec struct {
	ID    string              `json:"id"`
	Trees []JConsumerTreeSpec `json:"trees"`
}

// JConsumerTreeSpec : consumer spec for a tree
type JConsumerTreeSpec struct {
	ID            string         `json:"id"`
	TreeName      string         `json:"treeName"`
	GroupID       string         `json:"groupID"`
	Request       map[string]int `json:"request"`
	Priority      int            `json:"priority"`
	CType         int            `json:"type"`
	UnPreemptable bool           `json:"unPreemptable"`
}
