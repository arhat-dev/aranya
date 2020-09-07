package pod

import (
	"arhat.dev/pkg/patchhelper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (m *Manager) patch(oldPod, newPod *corev1.Pod) (*corev1.Pod, error) {
	err := patchhelper.TwoWayMergePatch(oldPod, newPod, &corev1.Pod{}, func(patchData []byte) error {
		var err error
		newPod, err = m.podClient.Patch(m.Context(), oldPod.Name,
			types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
		return err
	})

	if err != nil {
		return oldPod, err
	}

	return newPod, nil
}
