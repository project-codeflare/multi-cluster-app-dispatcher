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

var (
	// DefaultTreeName : the default name of the tree (if unspecified)
	DefaultTreeName string = "default"

	// DefaultResourceNames : the default resource names
	DefaultResourceNames []string = []string{"cpu", "memory", "nvidia.com/gpu"}

	// DefaultTreeKind : the default kind attribute of the tree
	DefaultTreeKind string = "QuotaTree"

	// DefaultConsumerKind : the kind attribute for a consumer
	DefaultConsumerKind string = "Consumer"
)
