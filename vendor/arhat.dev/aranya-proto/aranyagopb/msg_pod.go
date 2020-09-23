// +build !rt_none

package aranyagopb

func NewPodStatusMsg(podUID, podIPv4, podIPv6 string, containerStatus map[string]*ContainerStatus) *PodStatusMsg {
	return &PodStatusMsg{
		Uid:               podUID,
		ContainerStatuses: containerStatus,
		PodIpv4:           podIPv4,
		PodIpv6:           podIPv6,
	}
}

func NewPodStatusListMsg(pods []*PodStatusMsg) *PodStatusListMsg {
	return &PodStatusListMsg{Pods: pods}
}
