// +build !rt_none

package aranyagopb

func NewPodStatusMsg(podUID string, network []byte, containerStatus map[string]*ContainerStatus) *PodStatusMsg {
	return &PodStatusMsg{
		Uid:        podUID,
		Network:    network,
		Containers: containerStatus,
	}
}

func NewPodStatusListMsg(pods []*PodStatusMsg) *PodStatusListMsg {
	return &PodStatusListMsg{Pods: pods}
}
