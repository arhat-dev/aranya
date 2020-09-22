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
	"sync"

	corev1 "k8s.io/api/core/v1"
)

func newNodeCache() *NodeCache {
	return &NodeCache{
		status: new(corev1.NodeStatus),
	}
}

// NodeCache thread-safe cache store for node status
type NodeCache struct {
	status *corev1.NodeStatus

	extLabels, extAnnotations map[string]string

	mu sync.RWMutex
}

func (n *NodeCache) UpdatePhase(p corev1.NodePhase) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if p != "" {
		n.status.Phase = p
	}
}

func (n *NodeCache) UpdateCapacity(c corev1.ResourceList) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(c) > 0 {
		n.status.Capacity = c
	}
}

func (n *NodeCache) UpdateConditions(c []corev1.NodeCondition) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(c) > 0 {
		n.status.Conditions = c
	}
}

func (n *NodeCache) UpdateSystemInfo(i *corev1.NodeSystemInfo) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if i != nil {
		n.status.NodeInfo = *i
	}
}

func (n *NodeCache) UpdateExtInfo(labels, annotations map[string]string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.extLabels = labels
	n.extAnnotations = annotations
}

func (n *NodeCache) RetrieveExtInfo() (labels, annotations map[string]string) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	labels, annotations = make(map[string]string), make(map[string]string)

	for k, v := range n.extLabels {
		labels[k] = v
	}

	for k, v := range n.extAnnotations {
		annotations[k] = v
	}

	return
}

// Retrieve latest node status cache
func (n *NodeCache) RetrieveStatus(s corev1.NodeStatus) corev1.NodeStatus {
	n.mu.RLock()
	defer n.mu.RUnlock()

	status := n.status.DeepCopy()
	result := s.DeepCopy()

	result.Phase = status.Phase
	result.Capacity = status.Capacity
	result.Conditions = status.Conditions
	result.NodeInfo = status.NodeInfo

	return *result
}
