package peripheral

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
	MetricsReporters map[string]*aranyagopb.PeripheralEnsureCmd
	Peripherals      map[string]*aranyagopb.PeripheralEnsureCmd
}

func NewManager(
	parentCtx context.Context,
	name string,
	connectivityManager connectivity.Manager,
	options *Options,
) *Manager {
	return &Manager{
		BaseManager: manager.NewBaseManager(parentCtx, fmt.Sprintf("device.%s", name), connectivityManager),

		requestedMRs:         options.MetricsReporters,
		requestedPeripherals: options.Peripherals,
	}
}

type Manager struct {
	*manager.BaseManager

	requestedMRs map[string]*aranyagopb.PeripheralEnsureCmd

	requestedPeripherals map[string]*aranyagopb.PeripheralEnsureCmd
}

// nolint:gocyclo
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
				devicesToRemove = sets.NewString()

				failedPeripherals  = make(map[string]*aranyagopb.PeripheralEnsureCmd)
				ensuredPeripherals = make(map[string]*aranyagopb.PeripheralEnsureCmd)

				failedMRs  = make(map[string]*aranyagopb.PeripheralEnsureCmd)
				ensuredMRs = make(map[string]*aranyagopb.PeripheralEnsureCmd)

				err error
			)

			msgCh, _, err := m.ConnectivityManager.PostCmd(
				0, aranyagopb.CMD_PERIPHERAL_LIST, aranyagopb.NewPeripheralListCmd(),
			)
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

				sl := msg.GetPeripheralStatusList()
				if sl == nil {
					return true
				}

				for _, ds := range sl.Peripherals {
					switch ds.Kind {
					case aranyagopb.PERIPHERAL_TYPE_NORMAL:
						if d, ok := m.requestedPeripherals[ds.Name]; ok {
							if ds.Name == d.Name && ds.State == aranyagopb.PERIPHERAL_STATE_CONNECTED {
								ensuredPeripherals[d.Name] = d
							} else {
								failedPeripherals[d.Name] = d
							}
						} else {
							devicesToRemove.Insert(ds.Name)
						}
					case aranyagopb.PERIPHERAL_TYPE_METRICS_REPORTER:
						if d, ok := m.requestedPeripherals[ds.Name]; ok {
							if ds.Name == d.Name && ds.State == aranyagopb.PERIPHERAL_STATE_CONNECTED {
								ensuredMRs[d.Name] = d
							} else {
								failedMRs[d.Name] = d
							}
						} else {
							devicesToRemove.Insert(ds.Name)
						}
					default:
						devicesToRemove.Insert(ds.Name)
					}
				}

				return false
			}, nil, connectivity.HandleUnknownMessage(logger))

			if err != nil {
				goto waitUntilDisconnected
			}

			if len(devicesToRemove) > 0 {
				go func() {
					for len(devicesToRemove) > 0 {
						time.Sleep(5 * time.Second)
						select {
						case <-m.Context().Done():
							return
						case <-m.ConnectivityManager.Disconnected():
							return
						default:
							devicesToRemove = m.removePeripherals(devicesToRemove)
						}
					}
				}()
			}

			if len(failedMRs) > 0 || len(failedPeripherals) > 0 {
				go func() {
					// ensure metrics reporters first
					for len(failedMRs) > 0 {
						// ensure failed device with timeout
						time.Sleep(5 * time.Second)
						select {
						case <-m.Context().Done():
							return
						case <-m.ConnectivityManager.Disconnected():
							return
						default:
							failedMRs = m.ensurePeripherals(failedMRs)
						}
					}

					for len(failedPeripherals) > 0 {
						// ensure failed device with timeout
						time.Sleep(5 * time.Second)
						select {
						case <-m.Context().Done():
							return
						case <-m.ConnectivityManager.Disconnected():
							return
						default:
							failedPeripherals = m.ensurePeripherals(failedPeripherals)
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

func (m *Manager) removePeripherals(deviceToRemove sets.String) sets.String {
	if deviceToRemove.Len() == 0 {
		return deviceToRemove
	}

	var (
		devices = deviceToRemove.UnsortedList()
	)

	logger := m.Log.WithFields(log.Strings("devices", devices))

	logger.D("removing unwanted devices")
	msgCh, _, err := m.ConnectivityManager.PostCmd(
		0, aranyagopb.CMD_PERIPHERAL_DELETE, aranyagopb.NewPeripheralDeleteCmd(devices...),
	)
	if err != nil {
		logger.I("failed to post device remove cmd", log.Error(err))
	} else {
		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("failed to remove device", log.Error(msgErr))
				return true
			}

			dsl := msg.GetPeripheralStatusList()
			if dsl == nil {
				return true
			}

			// TODO: update pod status
			for _, ds := range dsl.Peripherals {
				deviceToRemove = deviceToRemove.Delete(ds.Name)
			}

			return false
		}, nil, connectivity.HandleUnknownMessage(logger))
	}

	return deviceToRemove
}

func (m *Manager) ensurePeripherals(
	failedPeripherals map[string]*aranyagopb.PeripheralEnsureCmd,
) map[string]*aranyagopb.PeripheralEnsureCmd {
	if len(failedPeripherals) == 0 {
		return nil
	}

	nextRound := make(map[string]*aranyagopb.PeripheralEnsureCmd)

	for _, dev := range failedPeripherals {
		d := dev
		logger := m.Log.WithFields(log.String("device", d.Name))

		msgCh, _, err := m.ConnectivityManager.PostCmd(
			0, aranyagopb.CMD_PERIPHERAL_ENSURE, d,
		)
		if err != nil {
			logger.I("failed to post device ensure cmd", log.Error(err))
			nextRound[d.Name] = failedPeripherals[d.Name]
		}

		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				logger.I("failed to ensure device", log.Error(msgErr))
				nextRound[d.Name] = failedPeripherals[d.Name]
				return true
			}

			status := msg.GetPeripheralStatus()
			if status == nil {
				nextRound[d.Name] = failedPeripherals[d.Name]
				logger.I("unexpected non device status msg", log.Any("msg", msg))
				return true
			}

			logger.D("ensured device")
			switch status.State {
			case aranyagopb.PERIPHERAL_STATE_CONNECTED:
				// TODO: update pod status
			default:
				nextRound[d.Name] = failedPeripherals[d.Name]
			}

			return false
		}, nil, connectivity.HandleUnknownMessage(logger))
	}

	return nextRound
}
