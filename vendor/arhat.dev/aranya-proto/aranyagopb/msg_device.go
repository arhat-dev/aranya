// +build !nodev

package aranyagopb

func NewDeviceStatusMsg(deviceID string, state DeviceStatusMsg_State, msg string) *DeviceStatusMsg {
	return &DeviceStatusMsg{DeviceId: deviceID, State: state, Message: msg}
}

func NewDeviceStatusListMsg(devices []*DeviceStatusMsg) *DeviceStatusListMsg {
	return &DeviceStatusListMsg{Devices: devices}
}
