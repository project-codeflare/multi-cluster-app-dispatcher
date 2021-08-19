// +build private

package resplanmgr

import (
	"k8s.io/klog/v2"
	rpv1 "sigs.k8s.io/scheduler-plugins/pkg/apis/resourceplan/v1"
	//rpv1 "github.com/IBM/multi-cluster-app-dispatcher/pkg/apis/resourceplan/v1"
)

func (rpm *ResourcePlanManager) addRP(obj interface{}) {
	rp, ok := obj.(*rpv1.ResourcePlan)
	if !ok {
		return
	}

	rpm.rpMutex.Lock()
	rpm.rpMap[string(rp.UID)] = rp
	rpm.rpMap[rp.Namespace+"/"+rp.Name] = rp
	rpm.setResplanChanged()
	rpm.rpMutex.Unlock()
	klog.V(10).Infof("[addRP] Add complete for: %s/%s", rp.Name, rp.Namespace)
}

func (rpm *ResourcePlanManager) updateRP(oldObj, newObj interface{}) {
	oldRP, ok := oldObj.(*rpv1.ResourcePlan)
	if !ok {
		return
	}

	newRP, ok := newObj.(*rpv1.ResourcePlan)
	if !ok {
		return
	}

	rpm.rpMutex.Lock()
	delete(rpm.rpMap, string(oldRP.UID))
	delete(rpm.rpMap, oldRP.Namespace+"/"+oldRP.Name)
	rpm.rpMap[string(newRP.UID)] = newRP
	rpm.rpMap[newRP.Namespace+"/"+newRP.Name] = newRP
	notify := false
	// status change (updating running/pending pods) will not update the Generation,
	// with this logic, we only need to handle necessary update.
	if oldRP.ObjectMeta.Generation != newRP.ObjectMeta.Generation {
		notify = true
	}
	rpm.rpMutex.Unlock()

	if notify {
		rpm.mutex.Lock()
		rpm.setResplanChanged()
		rpm.mutex.Unlock()
	}
	klog.V(10).Infof("[updateRP] Update complete for: %s/%s", newRP.Name, newRP.Namespace)
}

func (rpm *ResourcePlanManager) deleteRP(obj interface{}) {
	rp, ok := obj.(*rpv1.ResourcePlan)
	if !ok {
		return
	}

	rpm.rpMutex.Lock()
	defer rpm.rpMutex.Unlock()

	delete(rpm.rpMap, string(rp.UID))
	delete(rpm.rpMap, rp.Namespace+"/"+rp.Name)
	klog.V(10).Infof("[deleteRP] Delete complete for: %s/%s", rp.Name, rp.Namespace)
}
