// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// IBM Confidential
// OCO Source Materials
// IBM Watson Machine Learning Core
//
// Copyright IBM Corp. 2021
//
// The source code for this program is not published or otherwise
// divested of its trade secrets, irrespective of what has been
// deposited with the U.S. Copyright Office.
// ------------------------------------------------------ {COPYRIGHT-END} ---
package quota

import (
	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clusterstateapi "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
)

type QuotaManagerInterface interface {
	Fits(aw *arbv1.AppWrapper, resources *clusterstateapi.Resource, proposedPremptions []*arbv1.AppWrapper) (bool, []*arbv1.AppWrapper, string)
	Release(aw *arbv1.AppWrapper) bool
}
