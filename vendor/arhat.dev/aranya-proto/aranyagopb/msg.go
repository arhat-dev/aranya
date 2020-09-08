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
	"fmt"
	"math"

	"github.com/gogo/protobuf/proto"
)

var (
	EmptyDataMsgSize int
)

func init() {
	EmptyDataMsgSize = NewDataMsg(math.MaxUint64, true, DATA_STDOUT, math.MaxUint64, make([]byte, 0)).Size()
}

func newMsg(kind MsgType, onlineID string, sid uint64, completed bool, msg proto.Marshaler) *Msg {
	body, _ := msg.Marshal()
	return &Msg{
		Kind:      kind,
		SessionId: sid,
		Completed: completed,
		OnlineId:  onlineID,
		Body:      body,
	}
}

func NewOnlineMsg(id string) *Msg {
	return newMsg(MSG_STATE, id, 0, true, &State{DeviceId: id})
}

func NewOfflineMsg(id string) *Msg {
	return newMsg(MSG_STATE, id, 0, true, &State{DeviceId: id})
}

func NewNodeMsg(
	sid uint64,
	systemInfo *NodeSystemInfo,
	capacity *NodeResources,
	conditions *NodeConditions,
	extInfo []*NodeExtInfo,
) *Msg {
	return newMsg(MSG_NODE, "", sid, true, &NodeStatus{
		SystemInfo: systemInfo,
		Capacity:   capacity,
		Conditions: conditions,
		ExtInfo:    extInfo,
	})
}

func NewDataErrorMsg(sid uint64, completed bool, seq uint64, err *Error) *Msg {
	data, _ := err.Marshal()
	return NewDataMsg(sid, completed, DATA_ERROR, seq, data)
}

func NewDataMsg(sid uint64, completed bool, kind Data_Kind, seq uint64, data []byte) *Msg {
	return newMsg(MSG_DATA, "", sid, completed, &Data{Kind: kind, Data: data, Seq: seq})
}

func NewCredentialStatus(sid uint64, sshPrivateKeySha256Hex string) *Msg {
	return newMsg(MSG_CRED, "", sid, true, &CredentialStatus{
		SshPrivateKeySha256Hex: sshPrivateKeySha256Hex,
	})
}

func newError(kind Error_Kind, description string, code int64) *Error {
	return &Error{
		Kind:        kind,
		Description: description,
		Code:        code,
	}
}

func NewTimeoutErrorMsg(sid uint64) *Msg {
	return newMsg(MSG_ERROR, "", sid, true, &Error{Kind: ERR_TIMEOUT, Description: "timeout"})
}

func NewError(kind Error_Kind, format string, args ...interface{}) *Error {
	return newError(kind, fmt.Sprintf(format, args...), 0)
}

func NewCommonError(format string, args ...interface{}) *Error {
	return newError(ERR_COMMON, fmt.Sprintf(format, args...), 0)
}

func NewCommonErrorWithCode(code int64, format string, args ...interface{}) *Error {
	return newError(ERR_COMMON, fmt.Sprintf(format, args...), code)
}

func NewErrorMsg(sid uint64, err *Error) *Msg {
	return newMsg(MSG_ERROR, "", sid, true, err)
}
