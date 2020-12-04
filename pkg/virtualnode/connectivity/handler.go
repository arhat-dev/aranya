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

import (
	"arhat.dev/aranya-proto/aranyagopb"
)

type (
	MsgHandleFunc func(msg *aranyagopb.Msg) (exit bool)
)

func HandleMessages(
	msgCh <-chan *aranyagopb.Msg,
	onMsg MsgHandleFunc,
) {
	exit := false

	// always drain message channel
	for msg := range msgCh {
		if exit {
			continue
		}

		exit = onMsg(msg)
	}
}
