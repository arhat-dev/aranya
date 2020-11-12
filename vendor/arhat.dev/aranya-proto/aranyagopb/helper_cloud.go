// +build !arhat

/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aranyagopb

import (
	"arhat.dev/aranya-proto/aranyagopb/runtimepb"
)

func (m *Msg) GetError() *ErrorMsg {
	if m.Kind != MSG_ERROR {
		return nil
	}

	err := new(ErrorMsg)
	if e := err.Unmarshal(m.Payload); e != nil {
		return nil
	}

	switch err.Kind {
	case ERR_COMMON:
		if err.Description != "" {
			return err
		}

		return nil
	default:
		return err
	}
}

func (m *Msg) GetStorageStatus() *StorageStatusMsg {
	if m.Kind != MSG_STORAGE_STATUS {
		return nil
	}

	ss := new(StorageStatusMsg)
	if err := ss.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return ss
}

func (m *Msg) GetStorageStatusList() *StorageStatusListMsg {
	if m.Kind != MSG_STORAGE_STATUS_LIST {
		return nil
	}

	ssl := new(StorageStatusListMsg)
	if err := ssl.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return ssl
}

func (m *Msg) GetData() []byte {
	switch m.Kind {
	case MSG_DATA_DEFAULT, MSG_DATA_STDERR:
	default:
		return nil
	}

	return m.Payload
}

func (m *Msg) GetState() *StateMsg {
	if m.Kind != MSG_STATE {
		return nil
	}

	s := new(StateMsg)
	if err := s.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return s
}

func (m *Msg) GetNodeStatus() *NodeStatusMsg {
	if m.Kind != MSG_NODE_STATUS {
		return nil
	}

	ns := new(NodeStatusMsg)
	if err := ns.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return ns
}

func (m *Msg) GetCredentialStatus() *CredentialStatusMsg {
	if m.Kind != MSG_CRED_STATUS {
		return nil
	}

	cs := new(CredentialStatusMsg)
	if err := cs.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return cs
}

func (m *Msg) GetPeripheralStatus() *PeripheralStatusMsg {
	if m.Kind != MSG_PERIPHERAL_STATUS {
		return nil
	}

	ds := new(PeripheralStatusMsg)
	if err := ds.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return ds
}

func (m *Msg) GetPeripheralStatusList() *PeripheralStatusListMsg {
	if m.Kind != MSG_PERIPHERAL_STATUS_LIST {
		return nil
	}

	dsl := new(PeripheralStatusListMsg)
	if err := dsl.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return dsl
}

func (m *Msg) GetPeripheralOperationResult() *PeripheralOperationResultMsg {
	if m.Kind != MSG_PERIPHERAL_OPERATION_RESULT {
		return nil
	}
	dor := new(PeripheralOperationResultMsg)
	if err := dor.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return dor
}

func (m *ErrorMsg) Error() string {
	if m == nil {
		return "<nil>"
	}
	return m.GetKind().String() + "/" + m.GetDescription()
}

func (m *ErrorMsg) IsCommon() bool {
	return m.isKind(ERR_COMMON)
}

func (m *ErrorMsg) IsNotFound() bool {
	return m.isKind(ERR_NOT_FOUND)
}

func (m *ErrorMsg) IsAlreadyExists() bool {
	return m.isKind(ERR_ALREADY_EXISTS)
}

func (m *ErrorMsg) IsNotSupported() bool {
	return m.isKind(ERR_NOT_SUPPORTED)
}

func (m *ErrorMsg) isKind(kind ErrorMsg_Kind) bool {
	if m == nil {
		return false
	}

	return m.Kind == kind
}

func (m *Msg) GetPodStatusList() *runtimepb.PodStatusListMsg {
	pkt := m.decodeAsRuntimePacket()
	if pkt == nil {
		return nil
	}

	if pkt.Kind != runtimepb.MSG_POD_STATUS_LIST {
		return nil
	}

	psl := new(runtimepb.PodStatusListMsg)
	if err := psl.Unmarshal(pkt.Payload); err != nil {
		return nil
	}

	return psl
}

func (m *Msg) GetPodStatus() *runtimepb.PodStatusMsg {
	pkt := m.decodeAsRuntimePacket()
	if pkt == nil {
		return nil
	}

	if pkt.Kind != runtimepb.MSG_POD_STATUS {
		return nil
	}

	ps := new(runtimepb.PodStatusMsg)
	if err := ps.Unmarshal(pkt.Payload); err != nil {
		return nil
	}

	return ps
}

func (m *Msg) GetImageList() *runtimepb.ImageStatusListMsg {
	pkt := m.decodeAsRuntimePacket()
	if pkt == nil {
		return nil
	}

	if pkt.Kind != runtimepb.MSG_IMAGE_STATUS_LIST {
		return nil
	}

	il := new(runtimepb.ImageStatusListMsg)
	if err := il.Unmarshal(pkt.Payload); err != nil {
		return nil
	}

	return il
}

func (m *Msg) decodeAsRuntimePacket() *runtimepb.Packet {
	if m.Kind != MSG_RUNTIME {
		return nil
	}

	pkt := new(runtimepb.Packet)
	if err := pkt.Unmarshal(m.Payload); err != nil {
		return nil
	}

	return pkt
}
