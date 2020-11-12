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
)

var (
	EmptyMsgSize int
)

func init() {
	emptyBody, _ := (&Empty{}).Marshal()
	emptyMsg := &Msg{
		Kind:      math.MaxInt32,
		Sid:       math.MaxUint64,
		Seq:       math.MaxUint64,
		Completed: true,
		Payload:   emptyBody,
	}

	EmptyMsgSize = emptyMsg.Size()
	_ = EmptyMsgSize
}

func NewMsg(kind MsgType, sid, seq uint64, completed bool, payload []byte) *Msg {
	return &Msg{
		Kind:      kind,
		Sid:       sid,
		Seq:       seq,
		Completed: completed,
		Payload:   payload,
	}
}

func NewOnlineStateMsg(id string) *StateMsg {
	return &StateMsg{Kind: STATE_ONLINE, DeviceId: id}
}

func NewOfflineStateMsg(id string) *StateMsg {
	return &StateMsg{Kind: STATE_OFFLINE, DeviceId: id}
}

func NewNodeStatusMsg(
	systemInfo *NodeSystemInfo,
	capacity *NodeResources,
	conditions *NodeConditions,
	extInfo []*NodeExtInfo,
) *NodeStatusMsg {
	return &NodeStatusMsg{
		SystemInfo: systemInfo,
		Capacity:   capacity,
		Conditions: conditions,
		ExtInfo:    extInfo,
	}
}

func NewCredentialStatusMsg(sshPrivateKeySha256Hex string) *CredentialStatusMsg {
	return &CredentialStatusMsg{
		SshPrivateKeySha256Hex: sshPrivateKeySha256Hex,
	}
}

func newError(k ErrorMsg_Kind, description string, code int64) *ErrorMsg {
	return &ErrorMsg{
		Kind:        k,
		Description: description,
		Code:        code,
	}
}

func NewTimeoutErrorMsg(sid uint64) *ErrorMsg {
	return newError(ERR_TIMEOUT, "timeout", 0)
}

func NewErrorMsg(kind ErrorMsg_Kind, format string, args ...interface{}) *ErrorMsg {
	return newError(kind, fmt.Sprintf(format, args...), 0)
}

func NewCommonErrorMsg(format string, args ...interface{}) *ErrorMsg {
	return newError(ERR_COMMON, fmt.Sprintf(format, args...), 0)
}

func NewCommonErrorMsgWithCode(code int64, format string, args ...interface{}) *ErrorMsg {
	return newError(ERR_COMMON, fmt.Sprintf(format, args...), code)
}
