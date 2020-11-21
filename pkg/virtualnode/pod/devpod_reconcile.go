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
	"fmt"
	"os"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (m *Manager) onRemotePodCreationRequested(obj interface{}) *reconcile.Result {
	var (
		podUID = obj.(types.UID)
		logger = m.Log.WithFields(log.String("podUID", string(podUID)))
	)

	logger.D("creating pod in device")
	pod, found := m.podCache.GetByID(podUID)
	if !found {
		logger.I("pod cache not found for pod creation")
		return nil
	}

	logger.D("creating pod in device")
	if err := m.CreateDevicePod(pod); err != nil {
		logger.I("failed to create pod in device", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	return nil
}

func (m *Manager) onRemotePodUpdateRequested(oldObj, newObj interface{}) *reconcile.Result {
	var (
		podUID = newObj.(types.UID)
		logger = m.Log.WithFields(log.String("podUID", string(podUID)))
	)

	logger.D("updating the pod in device")
	pod, found := m.podCache.GetByID(podUID)
	if !found {
		logger.I("pod cache not found for device pod update")
		return nil
	}

	if err := m.DeleteDevicePod(pod.UID); err != nil {
		logger.I("failed to delete pod in device for update", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	if err := m.CreateDevicePod(pod); err != nil {
		// pod has been deleted, only need to create pod next time
		logger.I("failed to create pod in device for update", log.Error(err))
		return &reconcile.Result{NextAction: queue.ActionAdd}
	}

	return nil
}

func (m *Manager) onRemotePodDeletionRequested(obj interface{}) *reconcile.Result {
	var (
		podUID = obj.(types.UID)
		logger = m.Log.WithFields(log.String("podUID", string(podUID)))
	)

	logger.D("deleting pod in device")
	err := m.DeleteDevicePod(podUID)
	if err != nil {
		logger.I("failed to delete pod in device", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	m.podCache.MarkDeletedInRemote(podUID, true)

	return &reconcile.Result{NextAction: queue.ActionCleanup}
}

func (m *Manager) onRemotePodDeleted(obj interface{}) *reconcile.Result {
	var (
		podUID = obj.(types.UID)
		logger = m.Log.WithFields(log.String("podUID", string(podUID)))
	)

	pod, ok := m.podCache.GetByID(podUID)
	if !ok {
		logger.I("device pod already deleted")
		return nil
	}

	// remove logs (if any)
	err := os.RemoveAll(podLogDir(m.options.Config.LogDir, pod.Namespace, pod.Name, string(pod.UID)))
	if err != nil && !os.IsNotExist(err) {
		logger.I("failed to delete log dir", log.Error(err))
	}

	volNames, err := m.getUniqueVolumeNamesAndPaths(pod)
	if err != nil {
		logger.I("failed to get unique volume names", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	if len(volNames) > 0 {
		node := m.options.GetNode()
		if node == nil {
			logger.I("failed to get node")
			return &reconcile.Result{Err: fmt.Errorf("failed to get node")}
		}

		node.Status.VolumesInUse, node.Status.VolumesAttached = m.removeVolumeInUse(
			node.Status.VolumesInUse, node.Status.VolumesAttached, volNames,
		)

		_, err = m.nodeClient.UpdateStatus(m.Context(), node, metav1.UpdateOptions{})
		if err != nil {
			logger.I("failed to update node volume in use", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	if m.StorageEnabled() {
		err = m.CleanupStorage(keepPVCAndInlineCSIOnly(pod))
		if err != nil {
			logger.I("failed to cleanup pod storage", log.Error(err))
			return &reconcile.Result{Err: err}
		}
	}

	logger.D("deleting pod object in cloud")
	err = m.podClient.Delete(m.Context(), pod.Name, *deleteAtOnce)
	if err != nil && !errors.IsNotFound(err) {
		logger.I("failed to delete pod object", log.Error(err))
		return &reconcile.Result{Err: err}
	}

	// delete pod cache once pod object has been deleted
	m.podCache.Delete(podUID)
	return nil
}
