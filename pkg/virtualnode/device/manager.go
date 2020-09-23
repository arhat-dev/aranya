package device

import (
	"context"
	"fmt"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"
	"k8s.io/apimachinery/pkg/util/sets"

	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Options struct {
	Devices map[string]*aranyagopb.DeviceEnsureCmd
}

func NewManager(
	parentCtx context.Context,
	name string,
	connectivityManager connectivity.Manager,
	options *Options,
) *Manager {
	return &Manager{
		BaseManager: manager.NewBaseManager(parentCtx, fmt.Sprintf("device.%s", name), connectivityManager),

		requestedDevices: options.Devices,
	}
}

type Manager struct {
	*manager.BaseManager

	requestedDevices map[string]*aranyagopb.DeviceEnsureCmd
}

func (m *Manager) Start() error {
	return m.OnStart(func() error {
		logger := m.Log.WithFields(log.String("routine", "main"))

		for !m.Closing() {
			// wait until device connected
			select {
			case <-m.Context().Done():
				return m.Context().Err()
			case <-m.ConnectivityManager.Connected():
			}

			var (
				failedDevices  = make(map[string]*aranyagopb.DeviceEnsureCmd)
				ensuredDevices = make(map[string]*aranyagopb.DeviceEnsureCmd)
				deviceToRemove = sets.NewString()
				err            error
			)

			msgCh, _, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_DEVICE_LIST, aranyagopb.NewDeviceListCmd())
			if err != nil {
				logger.I("failed to post device list cmd", log.Error(err))

				goto waitUntilDisconnected
			}

			connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
				if msgErr := msg.GetError(); msgErr != nil {
					logger.I("failed to list devices", log.Error(msgErr))
					err = msgErr
					return true
				}

				sl := msg.GetDeviceStatusList()
				if sl == nil {
					return true
				}

				for _, ds := range sl.Devices {
					if d, ok := m.requestedDevices[ds.DeviceId]; ok {
						if ds.DeviceId == d.DeviceId && ds.State == aranyagopb.DEVICE_STATE_CONNECTED {
							ensuredDevices[d.DeviceId] = d
						} else {
							failedDevices[d.DeviceId] = d
						}
					} else {
						deviceToRemove.Insert(ds.DeviceId)
					}
				}

				return false
			}, nil, connectivity.HandleUnknownMessage(logger))

			if err != nil {
				goto waitUntilDisconnected
			}

			deviceToRemove = m.removeDevices(deviceToRemove)
			ensuredDevices, failedDevices = m.ensureDevices(m.requestedDevices)

			if len(deviceToRemove) > 0 {
				go func() {
					for len(deviceToRemove) > 0 {
						time.Sleep(5 * time.Second)
						select {
						case <-m.Context().Done():
							return
						case <-m.ConnectivityManager.Disconnected():
							return
						default:
							deviceToRemove = m.removeDevices(deviceToRemove)
						}
					}
				}()
			}

			if len(failedDevices) > 0 {
				go func() {
					for len(failedDevices) > 0 {
						// ensure failed device with timeout
						time.Sleep(5 * time.Second)
						select {
						case <-m.Context().Done():
							return
						case <-m.ConnectivityManager.Disconnected():
							return
						default:
							ensuredDevices, failedDevices = m.ensureDevices(failedDevices)
						}
					}
				}()
			}
		waitUntilDisconnected:
			select {
			case <-m.Context().Done():
				return m.Context().Err()
			case <-m.ConnectivityManager.Disconnected():
			}
		}
		return nil
	})
}

// Close the metrics manager
func (m *Manager) Close() {
	m.OnClose(nil)
}

func (m *Manager) removeDevices(deviceToRemove sets.String) sets.String {
	if deviceToRemove.Len() == 0 {
		return deviceToRemove
	}

	var (
		devices = deviceToRemove.UnsortedList()
	)

	logger := m.Log.WithFields(log.Strings("devices", devices))

	logger.D("removing unwanted devices")
	msgCh, _, err := m.ConnectivityManager.PostCmd(
		0, aranyagopb.CMD_DEVICE_DELETE, aranyagopb.NewDeviceDeleteCmd(devices...),
	)
	if err != nil {
		logger.I("failed to post device remove cmd", log.Error(err))
	} else {
		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("failed to remove device", log.Error(msgErr))
				return true
			}

			dsl := msg.GetDeviceStatusList()
			if dsl == nil {
				return true
			}

			// TODO: update pod status
			for _, ds := range dsl.Devices {
				deviceToRemove = deviceToRemove.Delete(ds.DeviceId)
			}

			return false
		}, nil, connectivity.HandleUnknownMessage(logger))
	}

	return deviceToRemove
}

func (m *Manager) ensureDevices(
	devices map[string]*aranyagopb.DeviceEnsureCmd,
) (ensuredDevices, failedDevices map[string]*aranyagopb.DeviceEnsureCmd) {
	failedDevices = make(map[string]*aranyagopb.DeviceEnsureCmd)
	ensuredDevices = make(map[string]*aranyagopb.DeviceEnsureCmd)

	for _, dev := range devices {
		d := dev
		if _, ok := ensuredDevices[d.DeviceId]; ok {
			continue
		}

		logger := m.Log.WithFields(log.String("device", d.DeviceId))

		msgCh, _, err := m.ConnectivityManager.PostCmd(
			0, aranyagopb.CMD_DEVICE_ENSURE, d,
		)
		if err != nil {
			logger.I("failed to post device ensure cmd", log.Error(err))
			failedDevices[d.DeviceId] = devices[d.DeviceId]
		}

		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("failed to ensure device", log.Error(msgErr))
				failedDevices[d.DeviceId] = devices[d.DeviceId]
				return true
			}

			status := msg.GetDeviceStatus()
			if status == nil {
				failedDevices[d.DeviceId] = devices[d.DeviceId]
				logger.I("unexpected non device status msg", log.Any("msg", msg))
				return true
			}

			logger.D("ensured device")
			switch status.State {
			case aranyagopb.DEVICE_STATE_UNKNOWN:
				fallthrough
			case aranyagopb.DEVICE_STATE_ERRORED:
				failedDevices[d.DeviceId] = devices[d.DeviceId]
			default:
				// TODO: update pod status
				ensuredDevices[d.DeviceId] = devices[d.DeviceId]
			}

			return false
		}, nil, connectivity.HandleUnknownMessage(logger))
	}

	return
}
