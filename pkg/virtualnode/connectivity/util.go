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

package connectivity

import "arhat.dev/aranya-proto/aranyagopb"

func createStateMessages(id string) (onlineMsgBytes, offlineMsgBytes []byte) {
	onlineMsgBytes, _ = (&aranyagopb.StateMsg{
		Kind:     aranyagopb.STATE_ONLINE,
		DeviceId: id,
	}).Marshal()

	onlineMsgBytes, _ = (&aranyagopb.Msg{
		Kind:     aranyagopb.MSG_STATE,
		Sid:      0,
		Seq:      0,
		Complete: true,
		Payload:  onlineMsgBytes,
	}).Marshal()

	offlineMsgBytes, _ = (&aranyagopb.StateMsg{
		Kind:     aranyagopb.STATE_OFFLINE,
		DeviceId: id,
	}).Marshal()

	offlineMsgBytes, _ = (&aranyagopb.Msg{
		Kind:     aranyagopb.MSG_STATE,
		Sid:      0,
		Seq:      0,
		Complete: true,
		Payload:  offlineMsgBytes,
	}).Marshal()

	return onlineMsgBytes, offlineMsgBytes
}
