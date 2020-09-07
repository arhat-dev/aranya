package pod

import (
	"fmt"

	"arhat.dev/pkg/log"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"arhat.dev/aranya/pkg/constant"
)

const (
	podCreationDenied  = "PodCreationDenied"
	podCreationAllowed = "PodCreationAllowed"
	podDeletionDenied  = "PodDeletionDenied"
	podDeletionAllowed = "PodDeletionAllowed"
)

func (m *Manager) getAbbotPod() (*corev1.Pod, bool) {
	podUID := m.abbotPodUIDStore.Load().(types.UID)
	if podUID == "" {
		return nil, false
	}
	return m.podCache.GetByID(podUID)
}

func (m *Manager) hasAbbotPod() bool {
	return m.abbotPodUIDStore.Load().(types.UID) != ""
}

func (m *Manager) setAbbotPod(podUID string) {
	m.abbotPodUIDStore.Store(podUID)
}

func isAbbotPod(pod *corev1.Pod) bool {
	looksLikeAbbotPod := pod.Labels != nil && pod.Labels[constant.LabelRole] == constant.LabelRoleValueAbbot
	if looksLikeAbbotPod {
		for _, ctr := range pod.Spec.Containers {
			if ctr.Name == constant.ContainerNameAbbot {
				return true
			}
		}
	}

	return false
}

func (m *Manager) admitPodCreation(pod *corev1.Pod) (handled bool, result *reconcile.Result) {
	var (
		err    error
		logger = m.Log.WithFields(log.String("pod", pod.Name), log.String("uid", string(pod.UID)))
	)

	// check if pod has been terminated first to avoid unexpected recreation
	switch pod.Status.Phase {
	case corev1.PodRunning:
		// pod was running on this node before, we have to handle this anyway
		logger.V("running pod is to be synced")

		// cache to allow further operations
		m.podCache.Update(pod)

		return true, nil
	case corev1.PodSucceeded, corev1.PodFailed:
		// pod terminated on this node before, we need to handle its cleanup if needed
		logger.V("pod already terminated")

		// cache to allow further operations
		m.podCache.Update(pod)

		return true, nil
	}

	if !pod.Spec.HostNetwork {
		// cluster network pods require abbot deployment
		result = &reconcile.Result{
			Err: fmt.Errorf("cluster network required, but abbot pod not running"),
		}

		if !m.hasAbbotPod() {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationDenied,
				"Using cluster network, abbot pod required but not found")
			return true, result
		}

		abbotPod, ok := m.getAbbotPod()
		if !ok {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationDenied,
				"abbot pod not found")
			return true, result
		}

		if abbotPod.Status.Phase != corev1.PodRunning {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationDenied,
				"abbot pod not in running phase")
			return true, result
		}

		logger.V("ensuring cluster network finalizer for pod")
		pod, err = m.ensureFinalizer(pod, constant.FinalizerClusterNetwork)
		if err != nil {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationDenied,
				fmt.Sprintf("Failed to ensure cluster network finalizer: %v", err))
			return true, &reconcile.Result{
				Err: fmt.Errorf("cluster network fianlizer not applied: %v", err),
			}
		}

		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationAllowed, "Owner ref to abbot pod set")

		// cache to allow further operations
		m.podCache.Update(pod)

		return false, nil
	}

	// host network pods expects no abbot pod deployment

	if !isAbbotPod(pod) {
		// not abbot pod, normal host network pod, create as usual
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationAllowed, "Using host network")

		// cache to allow further operations
		m.podCache.Update(pod)

		return false, nil
	}

	if m.hasAbbotPod() {
		// only one abbot pod allowed per node
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationDenied,
			"Already have abbot pod in this node")
		return true, &reconcile.Result{
			Err: fmt.Errorf("only one abbot pod can be deployed per node"),
		}
	}

	// this is the abbot pod to be deployed
	pod, err = m.ensureFinalizer(pod, constant.FinalizerClusterNetworkManager)
	if err != nil {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podCreationDenied,
			"Failed to set cluster network manager finalizer to abbot pod")
		return true, &reconcile.Result{
			Err: fmt.Errorf("failed to add cluster network manager finalizer for abbot pod: %w", err),
		}
	}

	m.setAbbotPod(string(pod.UID))

	// cache to allow further operations
	m.podCache.Update(pod)

	return false, nil
}

func (m *Manager) admitPodDeletion(pod *corev1.Pod) (handled bool, _ *reconcile.Result) {
	var (
		err error
	)

	if !pod.Spec.HostNetwork {
		// cluster network pods require abbot pod
		if !m.hasAbbotPod() {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionDenied,
				"Using cluster network, but abbot pod not found")
			return true, &reconcile.Result{
				Err: fmt.Errorf("cannot delete cluster network pod without abbot pod"),
			}
		}

		// has abbot pod

		pod, err = m.removeFinalizer(pod, constant.FinalizerClusterNetwork)
		if err != nil {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionDenied,
				fmt.Sprintf("Failed to remove cluster network finalizer: %v", err))
			return true, &reconcile.Result{Err: err}
		}

		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionAllowed, "Deleting pod")
		return false, nil
	}

	// host network pods

	if !isAbbotPod(pod) {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionAllowed, "Deleting pod")
		return false, nil
	}

	// abbot pod, check if cluster network pods exists
	for _, p := range m.podCache.GetAll() {
		if !p.Spec.HostNetwork {
			m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionDenied,
				"Cluster network pod(s) exists, abbot pod is not going to be deleted")
			return true, &reconcile.Result{
				Err: fmt.Errorf("cannot delete abbot pod when cluster network pod deployed"),
			}
		}
	}

	pod, err = m.removeFinalizer(pod, constant.FinalizerClusterNetworkManager)
	if err != nil {
		m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionDenied,
			fmt.Sprintf("Failed to remove cluster network manager finalizer: %v", err))
		return true, &reconcile.Result{Err: err}
	}

	// abbot deletion allowed, set abbot pod to none
	m.setAbbotPod("")

	m.options.EventRecorder.Event(pod, corev1.EventTypeNormal, podDeletionAllowed, "Deleting pod")
	return false, nil
}
