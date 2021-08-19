
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

