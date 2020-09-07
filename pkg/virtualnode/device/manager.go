package device

import (
	"context"
	"fmt"
	"time"

	"arhat.dev/pkg/log"

	"arhat.dev/aranya-proto/gopb"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Options struct {
	Devices map[string]*gopb.Device
}

func NewManager(parentCtx context.Context, name string, connectivityManager connectivity.Manager, options *Options) *Manager {
	return &Manager{
		BaseManager: manager.NewBaseManager(parentCtx, fmt.Sprintf("device.%s", name), connectivityManager),

		requestedDevices: options.Devices,
	}
}

type Manager struct {
	*manager.BaseManager

	requestedDevices map[string]*gopb.Device
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
				failedDevices  = make(map[string]*gopb.Device)
				ensuredDevices = make(map[string]*gopb.Device)
				deviceToRemove = make(map[string]struct{})
				err            error
			)

			msgCh, _, err := m.ConnectivityManager.PostCmd(0, gopb.NewDeviceListCmd())
			if err != nil {
				logger.I("failed to post device list cmd", log.Error(err))

				goto waitUntilDisconnected
			}

			gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
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
					if d, ok := m.requestedDevices[ds.Id]; ok {
						if ds.Id == d.Id && ds.State == gopb.DEVICE_STATE_CONNECTED {
							ensuredDevices[d.Id] = d
						} else {
							failedDevices[d.Id] = d
						}
					} else {
						deviceToRemove[ds.Id] = struct{}{}
					}
				}

				return false
			}, nil, gopb.HandleUnknownMessage(logger))

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

func (m *Manager) removeDevices(deviceToRemove map[string]struct{}) (remains map[string]struct{}) {
	var removedDevice []string
	for deviceID := range deviceToRemove {
		logger := m.Log.WithFields(log.String("device", deviceID))

		logger.D("removing unwanted devices")
		msgCh, _, err := m.ConnectivityManager.PostCmd(0, gopb.NewDeviceRemoveCmd(deviceID))
		if err != nil {
			logger.I("failed to post device remove cmd", log.Error(err))
			continue
		}

		gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("failed to remove device", log.Error(msgErr))
				return true
			}

			status := msg.GetDeviceStatus()
			if status == nil {
				return true
			}

			// TODO: update pod status
			removedDevice = append(removedDevice, status.Id)
			return false
		}, nil, gopb.HandleUnknownMessage(logger))
	}

	for _, id := range removedDevice {
		delete(deviceToRemove, id)
	}

	return deviceToRemove
}

func (m *Manager) ensureDevices(devices map[string]*gopb.Device) (ensuredDevices, failedDevices map[string]*gopb.Device) {
	failedDevices = make(map[string]*gopb.Device)
	ensuredDevices = make(map[string]*gopb.Device)

	for _, d := range devices {
		if _, ok := ensuredDevices[d.Id]; ok {
			continue
		}

		logger := m.Log.WithFields(log.String("device", d.Id))

		msgCh, _, err := m.ConnectivityManager.PostCmd(0, gopb.NewDeviceEnsureCmd(d))
		if err != nil {
			logger.I("failed to post device ensure cmd", log.Error(err))
			failedDevices[d.Id] = devices[d.Id]
		}

		gopb.HandleMessages(msgCh, func(msg *gopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("failed to ensure device", log.Error(msgErr))
				failedDevices[d.Id] = devices[d.Id]
				return true
			}

			status := msg.GetDeviceStatus()
			if status == nil {
				failedDevices[d.Id] = devices[d.Id]
				logger.I("unexpected non device status msg", log.Any("msg", msg))
				return true
			}

			logger.D("ensured device")
			switch status.State {
			case gopb.DEVICE_STATE_UNKNOWN:
				fallthrough
			case gopb.DEVICE_STATE_ERRORED:
				failedDevices[d.Id] = devices[d.Id]
			default:
				// TODO: update pod status
				ensuredDevices[d.Id] = devices[d.Id]
			}

			return false
		}, nil, gopb.HandleUnknownMessage(logger))
	}

	return
}
