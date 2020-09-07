// +build !arhat,!abbot

/*
Copyright 2019 The arhat.dev Authors.

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

package gopb

import "arhat.dev/pkg/log"

type (
	MsgHandleFunc        func(msg *Msg) (exit bool)
	DataHandleFunc       func(data *Data) (exit bool)
	UnknownMsgHandleFunc func(u interface{}) (exit bool)
)

func HandleUnknownMessage(logger log.Interface) UnknownMsgHandleFunc {
	return func(u interface{}) bool {
		logger.I("unknown message type", log.Any("msg", u))
		return true
	}
}

func HandleMessages(
	msgCh <-chan interface{},
	onMsg MsgHandleFunc,
	onData DataHandleFunc,
	onUnknown UnknownMsgHandleFunc,
) {
	if onMsg == nil {
		onMsg = func(msg *Msg) bool { return false }
	}

	if onData == nil {
		onData = func(data *Data) bool { return false }
	}

	if onUnknown == nil {
		onUnknown = func(u interface{}) bool { return false }
	}

	exit := false
	for msg := range msgCh {
		if exit {
			continue
		}

		switch m := msg.(type) {
		case *Msg:
			exit = onMsg(m)
		case *Data:
			exit = onData(m)
		default:
			exit = onUnknown(msg)
		}
	}
}

func (m *Msg) GetError() *Error {
	if m.Kind != MSG_ERROR {
		return nil
	}

	err := new(Error)
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

func (m *Msg) GetStorageStatus() *StorageStatus {
	if m.Kind != MSG_STORAGE {
		return nil
	}

	ss := new(StorageStatus)
	if err := ss.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ss
}

func (m *Msg) GetStorageStatusList() *StorageStatusList {
	if m.Kind != MSG_STORAGE {
		return nil
	}

	ssl := new(StorageStatusList)
	if err := ssl.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ssl
}

func (m *Msg) GetData() *Data {
	if m.Kind != MSG_DATA {
		return nil
	}

	d := new(Data)
	if err := d.Unmarshal(m.Body); err != nil {
		return nil
	}

	return d
}

func (m *Msg) GetState() *State {
	if m.Kind != MSG_STATE {
		return nil
	}

	s := new(State)
	if err := s.Unmarshal(m.Body); err != nil {
		return nil
	}

	return s
}

func (m *Msg) GetNodeStatus() *NodeStatus {
	if m.Kind != MSG_NODE {
		return nil
	}

	ns := new(NodeStatus)
	if err := ns.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ns
}

func (m *Msg) GetPodStatusList() *PodStatusList {
	if m.Kind != MSG_POD {
		return nil
	}

	psl := new(PodStatusList)
	if err := psl.Unmarshal(m.Body); err != nil {
		return nil
	}

	return psl
}

func (m *Msg) GetPodStatus() *PodStatus {
	if m.Kind != MSG_POD {
		return nil
	}

	ps := new(PodStatus)
	if err := ps.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ps
}

func (m *Msg) GetImageList() *ImageList {
	if m.Kind != MSG_POD {
		return nil
	}

	il := new(ImageList)
	if err := il.Unmarshal(m.Body); err != nil {
		return nil
	}

	return il
}

func (m *Msg) GetMetrics() *Metrics {
	if m.Kind != MSG_METRICS {
		return nil
	}

	mc := new(Metrics)
	if err := mc.Unmarshal(m.Body); err != nil {
		return nil
	}

	return mc
}

func (m *Msg) GetCredentialStatus() *CredentialStatus {
	if m.Kind != MSG_CRED {
		return nil
	}

	cs := new(CredentialStatus)
	if err := cs.Unmarshal(m.Body); err != nil {
		return nil
	}

	return cs
}

func (m *Msg) GetDeviceStatus() *DeviceStatus {
	if m.Kind != MSG_DEVICE {
		return nil
	}

	ds := new(DeviceStatus)
	if err := ds.Unmarshal(m.Body); err != nil {
		return nil
	}

	return ds
}

func (m *Msg) GetDeviceStatusList() *DeviceStatusList {
	if m.Kind != MSG_DEVICE {
		return nil
	}

	dsl := new(DeviceStatusList)
	if err := dsl.Unmarshal(m.Body); err != nil {
		return nil
	}

	return dsl
}

func (m *Error) Error() string {
	if m == nil {
		return "<nil>"
	}
	return m.GetKind().String() + "/" + m.GetDescription()
}

func (m *Error) IsCommon() bool {
	return m.isKind(ERR_COMMON)
}

func (m *Error) IsNotFound() bool {
	return m.isKind(ERR_NOT_FOUND)
}

func (m *Error) IsAlreadyExists() bool {
	return m.isKind(ERR_ALREADY_EXISTS)
}

func (m *Error) IsNotSupported() bool {
	return m.isKind(ERR_NOT_SUPPORTED)
}

func (m *Error) isKind(kind Error_Kind) bool {
	if m == nil {
		return false
	}

	return m.Kind == kind
}

func (s *PodStatus_ContainerStatus) GetState() PodStatus_State {
	if s == nil {
		return STATE_UNKNOWN
	}

	createdAt, startedAt, finishedAt := s.GetTimeCreatedAt(), s.GetTimeStartedAt(), s.GetTimeFinishedAt()

	if !startedAt.After(finishedAt) {
		if s.ExitCode != 0 {
			return STATE_FAILED
		}

		return STATE_SUCCEEDED
	}

	if !startedAt.IsZero() {
		return STATE_RUNNING
	}

	if !createdAt.IsZero() {
		return STATE_PENDING
	}

	if s.ContainerId == "" {
		return STATE_UNKNOWN
	}

	return STATE_PENDING
}
