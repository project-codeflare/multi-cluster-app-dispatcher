
package quota

import (
	arbv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
	clusterstateapi "github.com/IBM/multi-cluster-app-dispatcher/pkg/controller/clusterstate/api"
)

type QuotaManagerInterface interface {
	Fits(aw *arbv1.AppWrapper, resources *clusterstateapi.Resource, proposedPremptions []*arbv1.AppWrapper) (bool, []*arbv1.AppWrapper)
	Release(aw *arbv1.AppWrapper) bool
}
