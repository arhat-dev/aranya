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

package pod

import (
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (m *Manager) onResourcePodUpdated(oldObj, newObj interface{}) *reconcile.Result {
	var (
		oldPod = oldObj.(*corev1.Pod)
		newPod = newObj.(*corev1.Pod)
		name   = newPod.Name
		logger = m.Log.WithFields(log.String("pod", name))
	)

	if name == m.nodeName {
		// virtual pod
		return nil
	}

	// admit pod, if pod was admitted, it will be cached
	handled, result := m.admitPodCreation(newPod)
	if handled {
		return result
	}

	_, ok := m.podCache.GetByID(newPod.UID)
	if !ok {
		logger.V("pod not admitted")
		return nil
	}

	if m.hostIP != "" && newPod.Status.HostIP != m.hostIP {
		pod := newPod.DeepCopy()
		pod.Status.HostIP = m.hostIP

		var err error
		newPod, err = m.patch(newPod, pod)
		if err != nil {
			logger.I("failed to set pod hostIP", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	// always cache latest pod
	m.podCache.Update(newPod)

	// check if pod already created on device pod
	if _, latest := m.devPodRec.Get(newPod.UID); latest == nil {
		// need to create it
		m.devPodRec.Update(newPod.UID, nil, newPod.UID)

		logger.D("scheduling pod creation job")
		err := m.devPodRec.Schedule(queue.Job{Action: queue.ActionAdd, Key: newPod.UID}, 0)
		if err != nil {
			logger.I("failed to schedule device pod creation job", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	// pod need to be updated on device only when some field of its spec has been changed
	if !podRequiresRecreation(oldPod, newPod) {
		logger.V("pod spec not updated, skip")
		return nil
	}

	logger.D("scheduling pod update job")
	err := m.devPodRec.Schedule(queue.Job{Action: queue.ActionUpdate, Key: newPod.UID}, 0)
	if err != nil {
		logger.I("failed to schedule device pod update job", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	logger.D("scheduled device pod update job")

	return nil
}

func (m *Manager) onResourcePodDeleting(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		logger = m.Log.WithFields(log.String("pod", name), log.String("uid", string(pod.UID)))
	)

	_, ok := m.podCache.GetByID(pod.UID)
	if !ok {
		// pod not admitted, delete it at once
		logger.I("deleting pod not admitted at once")

		err := m.podClient.Delete(m.Context(), name, *deleteAtOnce)
		if err != nil && !errors.IsNotFound(err) {
			logger.I("failed to delete pod not admitted", log.Error(err))
			return &reconcile.Result{Err: err}
		}

		return nil
	}

	// pod admitted, admit pod deletion
	handled, result := m.admitPodDeletion(pod)
	if handled {
		return result
	}

	// pod object being deleted, and we do have cache for it, which means we have that
	// pod running on the edge device, delete it
	m.devPodRec.Update(pod.UID, nil, pod.UID)
	err := m.devPodRec.Schedule(queue.Job{Action: queue.ActionDelete, Key: pod.UID}, 0)
	if err != nil {
		logger.I("failed to schedule device pod deletion", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	logger.D("scheduled device pod deletion")

	return nil
}

func (m *Manager) onResourcePodDeleted(obj interface{}) *reconcile.Result {
	var (
		pod    = obj.(*corev1.Pod)
		name   = pod.Name
		logger = m.Log.WithFields(log.String("pod", name), log.String("uid", string(pod.UID)))
	)

	if name == m.nodeName {
		return nil
	}

	_ = logger

	// normal pod, delegate to onResourcePodDeleting
	return &reconcile.Result{NextAction: queue.ActionDelete}
}
