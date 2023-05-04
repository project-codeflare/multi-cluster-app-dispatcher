// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// Copyright 2022 The Multi-Cluster App Dispatcher Authors.
// 
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// ------------------------------------------------------ {COPYRIGHT-END} ---
package util

import (
	"fmt"
	"strings"
)

const (
	// AW Namespace used for building unique name for AW job
	NamespacePrefix string = "NAMESPACE_"

	// AW Name used for building unique name for AW job
	AppWrapperNamePrefix string = "_AWNAME_"
)

func ParseId(id string) (string, string) {
	ns := ""
	n := ""

	// Extract the namespace seperator
	nspSplit := strings.Split(id, NamespacePrefix)
	if len(nspSplit) == 2 {
		// Extract the appwrapper seperator
		awnpSplit := strings.Split(nspSplit[1], AppWrapperNamePrefix)
		if len(awnpSplit) == 2 {
			// What is left if the namespace value in the first slice
			if len(awnpSplit[0]) > 0 {
				ns = awnpSplit[0]
			}
			// And the names value in the second slice
			if len(awnpSplit[1]) > 0 {
				n = awnpSplit[1]
			}
		}
	}
	return ns, n
}

func CreateId(ns string, n string) string {
	id := ""
	if len(ns) > 0 && len(n) > 0 {
		id = fmt.Sprintf("%s%s%s%s", NamespacePrefix, ns, AppWrapperNamePrefix, n)
	}
	return id
}

