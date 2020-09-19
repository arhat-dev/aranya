// +build !arhat,!abbot

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

func (m *Msg) GetError() *ErrorMsg {
	if m.Header.Kind != MSG_ERROR {
		return nil
	}

	err := new(ErrorMsg)
	if e := err.Unmarshal(m.Body); e != nil {
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
	if m.Header.Kind != MSG_STORAGE_STATUS {
		return nil
	}

	ss := new(StorageStatusMsg)
	if err := ss.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ss
}

func (m *Msg) GetStorageStatusList() *StorageStatusListMsg {
	if m.Header.Kind != MSG_STORAGE_STATUS_LIST {
		return nil
	}

	ssl := new(StorageStatusListMsg)
	if err := ssl.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ssl
}

func (m *Msg) GetData() []byte {
	switch m.Header.Kind {
	case MSG_DATA_STDOUT, MSG_DATA_STDERR, MSG_DATA_METRICS:
	default:
		return nil
	}

	return m.Body
}

func (m *Msg) GetState() *StateMsg {
	if m.Header.Kind != MSG_STATE {
		return nil
	}

	s := new(StateMsg)
	if err := s.Unmarshal(m.Body); err != nil {
		return nil
	}

	return s
}

func (m *Msg) GetNodeStatus() *NodeStatusMsg {
	if m.Header.Kind != MSG_NODE_STATUS {
		return nil
	}

	ns := new(NodeStatusMsg)
	if err := ns.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ns
}

func (m *Msg) GetPodStatusList() *PodStatusListMsg {
	if m.Header.Kind != MSG_POD_STATUS_LIST {
		return nil
	}

	psl := new(PodStatusListMsg)
	if err := psl.Unmarshal(m.Body); err != nil {
		return nil
	}

	return psl
}

func (m *Msg) GetPodStatus() *PodStatusMsg {
	if m.Header.Kind != MSG_POD_STATUS {
		return nil
	}

	ps := new(PodStatusMsg)
	if err := ps.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ps
}

func (m *Msg) GetImageList() *ImageStatusListMsg {
	if m.Header.Kind != MSG_IMAGE_STATUS_LIST {
		return nil
	}

	il := new(ImageStatusListMsg)
	if err := il.Unmarshal(m.Body); err != nil {
		return nil
	}

	return il
}

func (m *Msg) GetCredentialStatus() *CredentialStatusMsg {
	if m.Header.Kind != MSG_CRED_STATUS {
		return nil
	}

	cs := new(CredentialStatusMsg)
	if err := cs.Unmarshal(m.Body); err != nil {
		return nil
	}

	return cs
}

func (m *Msg) GetDeviceStatus() *DeviceStatusMsg {
	if m.Header.Kind != MSG_DEVICE_STATUS {
		return nil
	}

	ds := new(DeviceStatusMsg)
	if err := ds.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ds
}

func (m *Msg) GetDeviceStatusList() *DeviceStatusListMsg {
	if m.Header.Kind != MSG_DEVICE_STATUS_LIST {
		return nil
	}

	dsl := new(DeviceStatusListMsg)
	if err := dsl.Unmarshal(m.Body); err != nil {
		return nil
	}

	return dsl
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

func (s *ContainerStatus) GetState() PodStatusMsg_State {
	if s == nil {
		return POD_STATE_UNKNOWN
	}

	createdAt, startedAt, finishedAt := s.GetTimeCreatedAt(), s.GetTimeStartedAt(), s.GetTimeFinishedAt()

	if !startedAt.After(finishedAt) {
		if s.ExitCode != 0 {
			return POD_STATE_FAILED
		}

		return POD_STATE_SUCCEEDED
	}

	if !startedAt.IsZero() {
		return POD_STATE_RUNNING
	}

	if !createdAt.IsZero() {
		return POD_STATE_PENDING
	}

	if s.ContainerId == "" {
		return POD_STATE_UNKNOWN
	}

	return POD_STATE_PENDING
}
