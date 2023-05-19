// b private
// ------------------------------------------------------ {COPYRIGHT-TOP} ---
// Copyright 2019, 2021, 2022, 2023 The Multi-Cluster App Dispatcher Authors.
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

package quotasubtmgr

import (
	qstv1 "github.com/project-codeflare/multi-cluster-app-dispatcher/pkg/apis/quotaplugins/quotasubtree/v1"
	"k8s.io/klog/v2"
)

func (qstm *QuotaSubtreeManager) addQST(obj interface{}) {
	qst, ok := obj.(*qstv1.QuotaSubtree)
	if !ok {
		return
	}

	qstm.qstMutex.Lock()
	qstm.qstMap[string(qst.UID)] = qst
	qstm.qstMap[qst.Namespace+"/"+qst.Name] = qst
	qstm.setQuotasubtreeChanged()
	qstm.qstMutex.Unlock()
	klog.V(10).Infof("[addQST] Add complete for: %s/%s", qst.Name, qst.Namespace)
}

func (qstm *QuotaSubtreeManager) updateQST(oldObj, newObj interface{}) {
	oldQST, ok := oldObj.(*qstv1.QuotaSubtree)
	if !ok {
		return
	}

	newQST, ok := newObj.(*qstv1.QuotaSubtree)
	if !ok {
		return
	}

	qstm.qstMutex.Lock()
	delete(qstm.qstMap, string(oldQST.UID))
	delete(qstm.qstMap, oldQST.Namespace+"/"+oldQST.Name)
	qstm.qstMap[string(newQST.UID)] = newQST
	qstm.qstMap[newQST.Namespace+"/"+newQST.Name] = newQST
	notify := false
	// status change (updating running/pending pods) will not update the Generation,
	// with this logic, we only need to handle necessary update.
	if oldQST.ObjectMeta.Generation != newQST.ObjectMeta.Generation {
		notify = true
	}
	qstm.qstMutex.Unlock()

	if notify {
		qstm.qstMutex.Lock()
		qstm.setQuotasubtreeChanged()
		qstm.qstMutex.Unlock()
	}
	klog.V(10).Infof("[updateQST] Update complete for: %s/%s", newQST.Name, newQST.Namespace)
}

func (qstm *QuotaSubtreeManager) deleteQST(obj interface{}) {
	qst, ok := obj.(*qstv1.QuotaSubtree)
	if !ok {
		return
	}

	qstm.qstMutex.Lock()
	defer qstm.qstMutex.Unlock()

	delete(qstm.qstMap, string(qst.UID))
	delete(qstm.qstMap, qst.Namespace+"/"+qst.Name)
	klog.V(10).Infof("[deleteQST] Delete complete for: %s/%s", qst.Name, qst.Namespace)
}
