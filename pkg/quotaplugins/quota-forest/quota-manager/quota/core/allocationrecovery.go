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

package core

import (
	"unsafe"

	"k8s.io/klog/v2"
)

// AllocationRecovery : Used to recover from failure after partial allocation
type AllocationRecovery struct {
	// consumer : subject to partial allocation
	consumer *Consumer
	// alteredNodes : nodes with altered allocations during partial allocation
	alteredNodes []*QuotaNode
	// alteredConsumers : consumers altered during partial allocation
	alteredConsumers map[string]*Consumer
	// originalConsumersNode : original allocation nodes of consumers altered during partial allocation
	originalConsumersNode map[string]*QuotaNode
}

// NewAllocationRecovery : create an allocation recovery for a consumer
func NewAllocationRecovery(consumer *Consumer) *AllocationRecovery {
	ar := &AllocationRecovery{
		consumer: consumer,
	}
	ar.Reset()
	return ar
}

// AlteredNode : mark a node as altered
func (ar *AllocationRecovery) AlteredNode(qn *QuotaNode) {
	ar.alteredNodes = append(ar.alteredNodes, qn)
}

// AlteredConsumer : mark a consumer as altered
func (ar *AllocationRecovery) AlteredConsumer(consumer *Consumer) {
	cid := consumer.GetID()
	if ar.alteredConsumers[cid] == nil {
		ar.alteredConsumers[cid] = consumer
		ar.originalConsumersNode[cid] = consumer.GetNode()
	}
}

// Recover : perform recovery actions
func (ar *AllocationRecovery) Recover() {

	for _, qn := range ar.alteredNodes {
		qn.SubtractRequest(ar.consumer)
	}
	consumerNode := ar.consumer.GetNode()
	if consumerNode != nil {
		consumerNode.RemoveConsumer(ar.consumer)
		ar.consumer.SetNode(nil)
	}

	if len(ar.alteredConsumers) > 0 {
		klog.V(4).Infoln("*** Recovering ...")
	}
	for cid, ci := range ar.alteredConsumers {
		ni := ar.originalConsumersNode[cid]
		if ci != nil && ni != nil {

			curNode := ci.GetNode()
			if curNode == ni {
				// nothing to do
				continue
			}

			var curNodeID string
			if curNode != nil {
				curNodeID = curNode.GetID()
			} else {
				curNodeID = "null"
			}
			klog.V(4).Infof("*** Restating consumer %s from node %s to %s \n", cid, curNodeID, ni.GetID())

			if curNode != nil {
				curNode.RemoveConsumer(ci)
			}
			ni.AddConsumer(ci)
			ci.SetNode(ni)
			pi := ni.GetPathToRoot()
			for _, p := range pi {
				pqn := (*QuotaNode)(unsafe.Pointer(p))
				if pqn == curNode {
					break
				}
				pqn.AddRequest(ci)
			}
		}
	}
}

// Reset : reset the allocation recovery
func (ar *AllocationRecovery) Reset() {
	ar.alteredNodes = make([]*QuotaNode, 0)
	ar.alteredConsumers = make(map[string]*Consumer)
	ar.originalConsumersNode = make(map[string]*QuotaNode)
}
