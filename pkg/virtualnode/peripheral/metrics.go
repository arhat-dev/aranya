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
	}, nil, connectivity.LogUnknownMessage(m.Log))

	return err
}
