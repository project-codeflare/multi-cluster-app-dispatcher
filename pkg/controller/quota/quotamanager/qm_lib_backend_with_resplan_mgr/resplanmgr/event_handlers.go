// +build private
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

package resplanmgr

import (
	"k8s.io/klog/v2"
	rpv1 "sigs.k8s.io/scheduler-plugins/pkg/apis/resourceplan/v1"
	//rpv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/resourceplan/v1"
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
