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

package peripheral

import (
	"encoding/base64"
	"fmt"
	"io"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (m *Manager) Operate(peripheralName, operationName string, data []byte, out io.Writer) error {
	logger := m.Log.WithFields(
		log.String("device", peripheralName),
		log.String("operation", operationName),
	)
	msgCh, _, err := m.ConnectivityManager.PostCmd(
		0,
		aranyagopb.CMD_PERIPHERAL_OPERATE,
		&aranyagopb.PeripheralOperateCmd{
			PeripheralName: peripheralName,
			OperationId:    operationName,
			Data:           data,
		},
	)

	if err != nil {
		logger.I("failed to post device ensure cmd", log.Error(err))
		return fmt.Errorf("failed to post device operate cmd")
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			err = msgErr
			logger.I("device operation failed", log.Error(msgErr))
			return true
		}

		dor := msg.GetPeripheralOperationResult()
		if dor == nil {
			logger.I("unexpected non device operation result msg")
			return true
		}

		for _, data := range dor.Data {
			_, err = fmt.Fprintln(out, base64.StdEncoding.EncodeToString(data))
			if err != nil {
				logger.I("failed to write operation result", log.Error(err))
			}
		}

		return false
	})
	return nil
}
