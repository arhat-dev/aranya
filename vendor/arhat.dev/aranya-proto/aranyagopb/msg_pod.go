// +build !rt_none

package aranyagopb

func NewPodStatus(podUID, podIP string, containerStatus map[string]*PodStatus_ContainerStatus) *PodStatus {
	return &PodStatus{
		Uid:               podUID,
		ContainerStatuses: containerStatus,
		PodIp:             podIP,
	}
}

func NewPodStatusMsg(sid uint64, pod *PodStatus) *Msg {
	return newMsg(MSG_POD, "", sid, true, pod)
}

func NewPodStatusListMsg(sid uint64, pods []*PodStatus) *Msg {
	return newMsg(MSG_POD, "", sid, true, &PodStatusList{Pods: pods})
}

func NewImageListMsg(sid uint64, images []*Image) *Msg {
	return newMsg(MSG_POD, "", sid, true, &ImageList{Images: images})
}
