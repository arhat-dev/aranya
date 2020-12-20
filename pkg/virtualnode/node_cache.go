/*
Copyright 2020 The arhat.dev Authors.

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

package virtualnode

import (
	"runtime"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
)

func newNodeCache() *NodeCache {
	return &NodeCache{
		status: new(corev1.NodeStatus),
	}
}

// NodeCache is a thread-safe cache store for node status and ext info (extra labels and annotations)
type NodeCache struct {
	status *corev1.NodeStatus

	extLabels, extAnnotations map[string]string

	_working uint32
}

func (n *NodeCache) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&n._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&n._working, 0)
}

func (n *NodeCache) UpdatePhase(p corev1.NodePhase) {
	if p == "" {
		return
	}

	n.doExclusive(func() { n.status.Phase = p })
}

func (n *NodeCache) UpdateCapacity(c corev1.ResourceList) {
	if len(c) == 0 {
		return
	}

	n.doExclusive(func() { n.status.Capacity = c })
}

func (n *NodeCache) UpdateConditions(c []corev1.NodeCondition) {
	if len(c) == 0 {
		return
	}

	n.doExclusive(func() { n.status.Conditions = c })
}

func (n *NodeCache) UpdateSystemInfo(i *corev1.NodeSystemInfo) {
	if i == nil {
		return
	}

	n.doExclusive(func() { n.status.NodeInfo = *i })
}

func (n *NodeCache) UpdateExtInfo(labels, annotations map[string]string) {
	n.doExclusive(func() {
		n.extLabels = labels
		n.extAnnotations = annotations
	})
}

func (n *NodeCache) RetrieveExtInfo() (labels, annotations map[string]string) {
	labels, annotations = make(map[string]string), make(map[string]string)

	n.doExclusive(func() {
		for k, v := range n.extLabels {
			labels[k] = v
		}

		for k, v := range n.extAnnotations {
			annotations[k] = v
		}
	})

	return
}

// Retrieve latest node status cache
func (n *NodeCache) RetrieveStatus(s corev1.NodeStatus) corev1.NodeStatus {
	var status *corev1.NodeStatus
	n.doExclusive(func() { status = n.status.DeepCopy() })

	result := s.DeepCopy()
	result.Phase = status.Phase
	result.Capacity = status.Capacity
	result.Conditions = status.Conditions
	result.NodeInfo = status.NodeInfo

	return *result
}
