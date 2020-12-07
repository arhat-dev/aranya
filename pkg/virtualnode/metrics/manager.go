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

package metrics

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya-proto/aranyagopb/runtimepb"
	"arhat.dev/pkg/log"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/util/cache"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Options struct {
	NodeMetrics    []conf.VirtualnodeNodeMetricsConfig
	RuntimeMetrics *aranyaapi.MetricsConfig
	GetOS          func() string
}

// NewManager creates a new metrics manager for virtual node
func NewManager(
	parentCtx context.Context,
	name string,
	connectivityManager connectivity.Manager,
	options *Options,
) *Manager {
	return &Manager{
		BaseManager: manager.NewBaseManager(parentCtx, fmt.Sprintf("metrics.%s", name), connectivityManager),

		options: options,

		nodeMetricsCache:    cache.NewMetricsCache(),
		runtimeMetricsCache: cache.NewMetricsCache(),
	}
}

// Manager the metrics manager
type Manager struct {
	*manager.BaseManager

	options *Options

	nodeMetricsCache    *cache.MetricsCache
	runtimeMetricsCache *cache.MetricsCache

	supportNodeMetrics    uint32
	supportRuntimeMetrics uint32
}

func (m *Manager) nodeMetricsSupported() bool {
	return atomic.LoadUint32(&m.supportNodeMetrics) == 1
}

func (m *Manager) containerMetricsSupported() bool {
	return atomic.LoadUint32(&m.supportRuntimeMetrics) == 1
}

// Start the metrics manager
func (m *Manager) Start() error {
	return m.OnStart(func() error {
		for !m.Closing() {
			// wait until device connected
			select {
			case <-m.Context().Done():
				return m.Context().Err()
			case <-m.ConnectivityManager.Connected():
			}

			m.Log.I("handling device metrics")

			// assume client support metrics collecting
			var (
				presets              = m.options.NodeMetrics
				nodeMetricsCollect   []string
				nodeMetricsExtraArgs []string
			)
			switch {
			case len(presets) == 1 && presets[0].OS == "":
				// overridden by EdgeDevice spec
				if presets[0].Enabled {
					nodeMetricsCollect = presets[0].Collect
					nodeMetricsExtraArgs = presets[0].ExtraArgs
				}
			case len(presets) > 0:
				// lookup metrics edge device's OS
				for _, p := range presets {
					if p.Enabled && p.OS == m.options.GetOS() {
						nodeMetricsCollect = p.Collect
						nodeMetricsExtraArgs = p.ExtraArgs
						break
					}
				}
			default:
				// no node metrics configured, skip
			}
			if len(nodeMetricsCollect) > 0 {
				m.Log.I("configuring node metrics")
				m.configureMetricsCollection(
					false,
					&aranyagopb.MetricsConfigCmd{
						Collect:   nodeMetricsCollect,
						ExtraArgs: nodeMetricsExtraArgs,
					},
				)
			} else {
				m.Log.I("skipped node metrics configuration")
			}

			if m.options.RuntimeMetrics.Enabled {
				m.Log.I("configuring runtime metrics")
				m.configureMetricsCollection(
					true,
					&aranyagopb.MetricsConfigCmd{
						Collect:   m.options.RuntimeMetrics.Collect,
						ExtraArgs: m.options.RuntimeMetrics.ExtraArgs,
					},
				)
			} else {
				m.Log.I("skipped container metrics configuration")
			}

			// device connected, serve until disconnected
			select {
			case <-m.Context().Done():
				return m.Context().Err()
			case <-m.ConnectivityManager.Disconnected():
				// disable metrics collection, since we cannot reach the edge device
				atomic.StoreUint32(&m.supportNodeMetrics, 0)
				atomic.StoreUint32(&m.supportRuntimeMetrics, 0)
			}
		}
		return nil
	})
}

// Close the metrics manager
func (m *Manager) Close() {
	m.OnClose(nil)
}

func (m *Manager) configureMetricsCollection(forRuntime bool, cmd *aranyagopb.MetricsConfigCmd) {
	if forRuntime {
		payload, err := cmd.Marshal()
		if err != nil {
			m.Log.I("failed to marshal metrics config cmd", log.Error(err))
			return
		}

		msgCh, _, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_RUNTIME, &runtimepb.Packet{
			Kind:    runtimepb.CMD_METRICS_CONFIG,
			Payload: payload,
		})

		if err != nil {
			m.Log.I("failed to post runtime metrics config cmd", log.Error(err))
			return
		}

		connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
			if msgErr := msg.GetError(); msgErr != nil {
				// no more metrics cmd should be issued
				m.Log.I("failed to configure runtime metrics collection", log.Error(msgErr))
				atomic.StoreUint32(&m.supportRuntimeMetrics, 0)

				return true
			}

			if msg.Kind != aranyagopb.MSG_RUNTIME {
				m.Log.I("unexpected non runtime message", log.Int32("kind", int32(msg.Kind)))
				atomic.StoreUint32(&m.supportRuntimeMetrics, 0)

				return true
			}

			pkt := new(runtimepb.Packet)
			err = pkt.Unmarshal(msg.Payload)
			if err != nil {
				m.Log.D("failed to unmarshal runtime packet", log.Error(err))
				atomic.StoreUint32(&m.supportRuntimeMetrics, 0)

				return true
			}

			switch pkt.Kind {
			case runtimepb.MSG_ERROR:
				atomic.StoreUint32(&m.supportRuntimeMetrics, 0)

				msgErr := new(aranyagopb.ErrorMsg)
				err = msgErr.Unmarshal(pkt.Payload)
				if err != nil {
					m.Log.D("failed to unmarshal runtime error message", log.Error(err))
					return true
				}

				m.Log.I("runtime metrics not supported", log.Error(err))
			case runtimepb.MSG_DONE:
				m.Log.D("configured runtime metrics")
				atomic.StoreUint32(&m.supportRuntimeMetrics, 1)
				return false
			default:
				m.Log.I("unexpected runtime message", log.Int32("kind", int32(pkt.Kind)))
				atomic.StoreUint32(&m.supportRuntimeMetrics, 0)
			}

			return true
		})

		return
	}

	msgCh, _, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_METRICS_CONFIG, cmd)
	if err != nil {
		m.Log.I("failed to post node metrics config cmd", log.Error(err))
		return
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if msgErr := msg.GetError(); msgErr != nil {
			// no more metrics cmd should be issued
			m.Log.I("failed to configure node metrics collection", log.Error(msgErr))
			atomic.StoreUint32(&m.supportNodeMetrics, 0)
			return true
		}

		switch msg.Kind {
		case aranyagopb.MSG_DONE:
			m.Log.I("configured node metrics")
			atomic.StoreUint32(&m.supportNodeMetrics, 1)
			return false
		default:
			m.Log.I("unexpected message response", log.Int32("kind", int32(msg.Kind)))
			atomic.StoreUint32(&m.supportNodeMetrics, 0)
			return true
		}
	})
}

func (m *Manager) retrieveDeviceMetrics(forRuntime bool) {
	var (
		msgCh <-chan *aranyagopb.Msg
		err   error
		buf   = new(bytes.Buffer)
	)
	if forRuntime {
		msgCh, _, _, err = m.ConnectivityManager.PostStreamCmd(aranyagopb.CMD_RUNTIME, &runtimepb.Packet{
			Kind:    runtimepb.CMD_METRICS_COLLECT,
			Payload: nil,
		}, buf, nil, true, nil)
	} else {
		msgCh, _, _, err = m.ConnectivityManager.PostStreamCmd(
			aranyagopb.CMD_METRICS_COLLECT, nil, buf, nil, true, nil,
		)
	}
	if err != nil {
		m.Log.I("failed to post metrics collect cmd", log.Error(err))
		return
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if err := msg.GetError(); err != nil {
			m.Log.I("failed to get metrics", log.Error(err))
			if err.Kind == aranyagopb.ERR_NOT_SUPPORTED {
				// no more metrics cmd should be issued
				if forRuntime {
					atomic.StoreUint32(&m.supportRuntimeMetrics, 0)
				} else {
					atomic.StoreUint32(&m.supportNodeMetrics, 0)
				}
			}

			return true
		}

		if msg.Kind != aranyagopb.MSG_DONE {
			m.Log.I("failed to collect metrics")
			return true
		}

		return false
	})

	err = m.UpdateMetrics(forRuntime, buf.Bytes())
	if err != nil {
		m.Log.I("failed to update metrics", log.Error(err))
	}
}

// UpdateMetrics cache the newly collected metrics
func (m *Manager) UpdateMetrics(forRuntime bool, metricsData []byte) error {
	if len(metricsData) == 0 {
		return nil
	}

	if forRuntime {
		return m.runtimeMetricsCache.Update(metricsData)
	}

	return m.nodeMetricsCache.Update(metricsData)
}
