package device

import (
	"fmt"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (m *Manager) CollectMetrics(deviceName string) error {
	logger := m.Log.WithFields(
		log.String("device", deviceName),
	)
	msgCh, _, err := m.ConnectivityManager.PostCmd(
		0,
		aranyagopb.CMD_DEVICE_OPERATE,
		aranyagopb.NewDeviceMetricsCollectCmd(deviceName),
	)

	if err != nil {
		logger.I("failed to post device metrics collect cmd", log.Error(err))
		return fmt.Errorf("failed to post device metrics collect cmd")
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			err = msgErr
			logger.I("device metrics collection failed", log.Error(msgErr))
			return true
		}

		// should get done
		return false
	}, nil, connectivity.HandleUnknownMessage(m.Log))

	return err
}
