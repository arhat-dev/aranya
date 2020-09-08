// +build !nodev

package aranyagopb

func NewDeviceStatus(deviceID string, state DeviceStatus_State, msg string) *DeviceStatus {
	return &DeviceStatus{Id: deviceID, State: state, Message: msg}
}

func NewDeviceStatusMsg(sid uint64, device *DeviceStatus) *Msg {
	return newMsg(MSG_DEVICE, "", sid, true, device)
}

func NewDeviceStatusListMsg(sid uint64, devices []*DeviceStatus) *Msg {
	return newMsg(MSG_DEVICE, "", sid, true, &DeviceStatusList{Devices: devices})
}
