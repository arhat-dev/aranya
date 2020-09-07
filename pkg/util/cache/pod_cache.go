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

package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewPodCache() *PodCache {
	return &PodCache{
		uidIndex:      make(map[types.UID]*corev1.Pod),
		nameIndex:     make(map[types.NamespacedName]*corev1.Pod),
		remoteDeleted: make(map[types.UID]struct{}),
		mu:            new(sync.RWMutex),
	}
}

// PodCache
type PodCache struct {
	uidIndex  map[types.UID]*corev1.Pod
	nameIndex map[types.NamespacedName]*corev1.Pod

	remoteDeleted map[types.UID]struct{}

	mu *sync.RWMutex
}

func (c *PodCache) MarkDeletedInRemote(podUID types.UID, deleted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if deleted {
		c.remoteDeleted[podUID] = struct{}{}
	} else {
		delete(c.remoteDeleted, podUID)
	}
}

func (c *PodCache) IsDeletedInRemote(podUID types.UID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, deleted := c.remoteDeleted[podUID]
	return deleted
}

func (c *PodCache) Update(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newPod := pod.DeepCopy()
	c.uidIndex[newPod.UID] = newPod
	c.nameIndex[types.NamespacedName{Namespace: newPod.Namespace, Name: newPod.Name}] = newPod
}

func (c *PodCache) GetAll() []*corev1.Pod {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*corev1.Pod
	for uid := range c.uidIndex {
		result = append(result, c.uidIndex[uid])
	}

	return result
}

func (c *PodCache) GetByID(podUID types.UID) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.uidIndex[podUID]
	if !ok {
		return nil, false
	}

	// deep copy before return, so the caller can make any changes to that pod object
	return pod.DeepCopy(), true
}

func (c *PodCache) GetByName(namespace, name string) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.nameIndex[types.NamespacedName{Namespace: namespace, Name: name}]
	if !ok {
		return nil, false
	}

	// deep copy before return, so the caller can make any changes to that pod object
	return pod.DeepCopy(), true
}

func (c *PodCache) Delete(podUID types.UID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pod, ok := c.uidIndex[podUID]; ok {
		delete(c.nameIndex, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})
	}

	delete(c.uidIndex, podUID)
	delete(c.remoteDeleted, podUID)
}

func (c *PodCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var uidAll []types.UID
	for uid, pod := range c.uidIndex {
		uidAll = append(uidAll, uid)
		delete(c.nameIndex, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})
	}

	for _, uid := range uidAll {
		delete(c.uidIndex, uid)
	}
}
