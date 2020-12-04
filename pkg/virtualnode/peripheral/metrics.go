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
	"fmt"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (m *Manager) CollectMetrics(peripheralName string) error {
	logger := m.Log.WithFields(
		log.String("peripheral", peripheralName),
	)
	msgCh, _, err := m.ConnectivityManager.PostCmd(
		0,
		aranyagopb.CMD_PERIPHERAL_OPERATE,
		&aranyagopb.PeripheralMetricsCollectCmd{
			PeripheralNames: []string{peripheralName},
		},
	)

	if err != nil {
		logger.I("failed to post peripheral metrics collect cmd", log.Error(err))
		return fmt.Errorf("failed to post peripheral metrics collect cmd")
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			err = msgErr
			logger.I("peripheral metrics collection failed", log.Error(msgErr))
			return true
		}

		// should get done
		return false
	})

	return err
}
