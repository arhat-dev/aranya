// +build !rt_none

package aranyagopb

func NewPodStatusMsg(podUID, podIP string, containerStatus map[string]*ContainerStatus) *PodStatusMsg {
	return &PodStatusMsg{
		Uid:               podUID,
		ContainerStatuses: containerStatus,
		PodIp:             podIP,
	}
}

func NewPodStatusListMsg(pods []*PodStatusMsg) *PodStatusListMsg {
	return &PodStatusListMsg{Pods: pods}
}
