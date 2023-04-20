package queuejob

import (
	"strings"
	"sync"

	arbv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/controller/v1beta1"
)

// ActiveAppWrapper is current scheduling AppWrapper in the XController struct.
// Its sole purpose is provide a thread safe way to for use the XController logic
type ActiveAppWrapper struct {
	activeAW      *arbv1.AppWrapper
	activeAWMutex *sync.RWMutex
}

// NewActiveAppWrapper
func NewActiveAppWrapper() *ActiveAppWrapper {
	return &ActiveAppWrapper{
		activeAW:      nil,
		activeAWMutex: &sync.RWMutex{},
	}
}

// AtomicSet as is name implies, atomically sets the activeAW to the new value
func (aw *ActiveAppWrapper) AtomicSet(newValue *arbv1.AppWrapper) {
	aw.activeAWMutex.Lock()
	defer aw.activeAWMutex.Unlock()
	aw.activeAW = newValue
}

// IsActiveAppWrapper safely performs the comparison that was done inside the if block
// at line 1977 in the queuejob_controller_ex.go
// The code looked like this:
//
//	if !qj.Status.CanRun && qj.Status.State == arbv1.AppWrapperStateEnqueued &&
//		!cc.qjqueue.IfExistUnschedulableQ(qj) && !cc.qjqueue.IfExistActiveQ(qj) {
//		// One more check to ensure AW is not the current active schedule object
//		if cc.schedulingAW == nil ||
//			(strings.Compare(cc.schedulingAW.Namespace, qj.Namespace) != 0 &&
//				strings.Compare(cc.schedulingAW.Name, qj.Name) != 0) {
//			cc.qjqueue.AddIfNotPresent(qj)
//			klog.V(3).Infof("[manageQueueJob] Recovered AppWrapper %s%s - added to active queue, Status=%+v",
//				qj.Namespace, qj.Name, qj.Status)
//			return nil
//		}
//	}
func (aw *ActiveAppWrapper) IsActiveAppWrapper(name, namespace string) bool {
	aw.activeAWMutex.RLock()
	defer aw.activeAWMutex.RUnlock()
	return aw.activeAW == nil ||
		(strings.Compare(aw.activeAW.Namespace, namespace) != 0 &&
			strings.Compare(aw.activeAW.Name, name) != 0)
}
