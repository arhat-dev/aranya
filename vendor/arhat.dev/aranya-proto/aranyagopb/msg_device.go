// +build !nodev

package aranyagopb

func NewDeviceStatusMsg(kind DeviceType, connectorHashHex string, state DeviceState, msg string) *DeviceStatusMsg {
	return &DeviceStatusMsg{
		Kind:             kind,
		ConnectorHashHex: connectorHashHex,
		State:            state,
		Message:          msg,
	}
}

func NewDeviceStatusListMsg(devices []*DeviceStatusMsg) *DeviceStatusListMsg {
	return &DeviceStatusListMsg{Devices: devices}
}

func NewDeviceOperationResultMsg(result [][]byte) *DeviceOperationResultMsg {
	return &DeviceOperationResultMsg{
		Data: result,
	}
}
