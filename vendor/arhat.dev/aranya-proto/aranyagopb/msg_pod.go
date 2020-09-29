// +build !rt_none

package aranyagopb

func NewPodStatusMsg(podUID, podIPv4, podIPv6 string, containerStatus map[string]*ContainerStatus) *PodStatusMsg {
	return &PodStatusMsg{
		Uid:        podUID,
		Containers: containerStatus,
		Ipv4:       podIPv4,
		Ipv6:       podIPv6,
	}
}

func NewPodStatusListMsg(pods []*PodStatusMsg) *PodStatusListMsg {
	return &PodStatusListMsg{Pods: pods}
}
