// +build !noperipheral

package aranyagopb

func NewPeripheralStatusMsg(
	kind PeripheralType,
	name string,
	state PeripheralState,
	msg string,
) *PeripheralStatusMsg {
	return &PeripheralStatusMsg{
		Kind:    kind,
		Name:    name,
		State:   state,
		Message: msg,
	}
}

func NewPeripheralStatusListMsg(peripherals []*PeripheralStatusMsg) *PeripheralStatusListMsg {
	return &PeripheralStatusListMsg{Peripherals: peripherals}
}

func NewPeripheralOperationResultMsg(result [][]byte) *PeripheralOperationResultMsg {
	return &PeripheralOperationResultMsg{
		Data: result,
	}
}
