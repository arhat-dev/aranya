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
	"arhat.dev/pkg/log"
)

type Data struct {
	Kind    aranyagopb.Kind
	Payload []byte
}

type (
	MsgHandleFunc        func(msg *aranyagopb.Msg) (exit bool)
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
		onMsg = func(*aranyagopb.Msg) bool { return false }
	}

	if onData == nil {
		onData = func(*Data) bool { return false }
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
		case *aranyagopb.Msg:
			exit = onMsg(m)
		case *Data:
			exit = onData(m)
		default:
			exit = onUnknown(msg)
		}
	}
}
