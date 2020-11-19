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
	"context"
	"fmt"
	"sync/atomic"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/log"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/util/cache"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Options struct {
	NodeMetrics      []conf.VirtualnodeNodeMetricsConfig
	ContainerMetrics *aranyaapi.MetricsConfig
	GetOS            func() string
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

		nodeMetricsCache:      cache.NewMetricsCache(),
		containerMetricsCache: cache.NewMetricsCache(),
	}
}

// Manager the metrics manager
type Manager struct {
	*manager.BaseManager

	options *Options

	nodeMetricsCache      *cache.MetricsCache
	containerMetricsCache *cache.MetricsCache

	supportNodeMetrics      uint32
	supportContainerMetrics uint32
}

func (m *Manager) nodeMetricsSupported() bool {
	return atomic.LoadUint32(&m.supportNodeMetrics) == 1
}

func (m *Manager) containerMetricsSupported() bool {
	return atomic.LoadUint32(&m.supportContainerMetrics) == 1
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
				m.Log.I("configuring device node metrics")
				m.configureMetricsCollection(
					&aranyagopb.MetricsConfigCmd{
						Target:    aranyagopb.METRICS_TARGET_NODE,
						Collect:   nodeMetricsCollect,
						ExtraArgs: nodeMetricsExtraArgs,
					},
				)
			} else {
				m.Log.I("skipped node metrics configuration")
			}

			if m.options.ContainerMetrics.Enabled {
				m.Log.I("configuring device container metrics")
				m.configureMetricsCollection(
					&aranyagopb.MetricsConfigCmd{
						Target:    aranyagopb.METRICS_TARGET_NODE,
						Collect:   m.options.ContainerMetrics.Collect,
						ExtraArgs: m.options.ContainerMetrics.ExtraArgs,
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
				atomic.StoreUint32(&m.supportContainerMetrics, 0)
			}
		}
		return nil
	})
}

// Close the metrics manager
func (m *Manager) Close() {
	m.OnClose(nil)
}

func (m *Manager) configureMetricsCollection(cmd *aranyagopb.MetricsConfigCmd) {
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_METRICS_CONFIG, cmd)
	if err != nil {
		m.Log.I("failed to post metrics config cmd", log.Error(err))
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if err := msg.GetError(); err != nil {
			// no more metrics cmd should be issued
			switch cmd.Target {
			case aranyagopb.METRICS_TARGET_NODE:
				m.Log.I("failed to configure node metrics collection", log.Error(err))
				atomic.StoreUint32(&m.supportNodeMetrics, 0)
			case aranyagopb.METRICS_TARGET_CONTAINER:
				m.Log.I("failed to configure container metrics collection", log.Error(err))
				atomic.StoreUint32(&m.supportContainerMetrics, 0)
			}

			return true
		}

		if msg.Kind != aranyagopb.MSG_DONE {
			return true
		}

		switch cmd.Target {
		case aranyagopb.METRICS_TARGET_NODE:
			m.Log.D("node metrics collection configured")
			atomic.StoreUint32(&m.supportNodeMetrics, 1)
		case aranyagopb.METRICS_TARGET_CONTAINER:
			m.Log.D("container metrics collection configured")
			atomic.StoreUint32(&m.supportContainerMetrics, 1)
		}

		return false
	}, nil, connectivity.LogUnknownMessage(m.Log))
}

func (m *Manager) retrieveDeviceMetrics(cmd *aranyagopb.MetricsCollectCmd) {
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, aranyagopb.CMD_METRICS_COLLECT, cmd)
	if err != nil {
		m.Log.I("failed to post metrics collect cmd", log.Error(err))
	}

	var metricsData []byte
	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if err := msg.GetError(); err != nil {
			m.Log.I("failed to get metrics", log.Error(err))
			if err.Kind == aranyagopb.ERR_NOT_SUPPORTED {
				// no more metrics cmd should be issued
				switch cmd.Target {
				case aranyagopb.METRICS_TARGET_NODE:
					atomic.StoreUint32(&m.supportNodeMetrics, 0)
				case aranyagopb.METRICS_TARGET_CONTAINER:
					atomic.StoreUint32(&m.supportContainerMetrics, 0)
				}
			}

			return true
		}

		if msg.Kind != aranyagopb.MSG_DONE {
			m.Log.I("failed to collect metrics")
			return true
		}

		return false
	}, func(data *connectivity.Data) (exit bool) {
		metricsData = append(metricsData, data.Payload...)
		return false
	}, connectivity.LogUnknownMessage(m.Log))

	err = m.UpdateMetrics(cmd.Target, metricsData)
	if err != nil {
		m.Log.I("failed to update metrics", log.Error(err))
	}
}

// UpdateMetrics cache the newly collected metrics
func (m *Manager) UpdateMetrics(target aranyagopb.MetricsTarget, metricsData []byte) error {
	if len(metricsData) == 0 {
		return nil
	}

	switch target {
	case aranyagopb.METRICS_TARGET_NODE:
		return m.nodeMetricsCache.Update(metricsData)
	case aranyagopb.METRICS_TARGET_CONTAINER:
		return m.containerMetricsCache.Update(metricsData)
	default:
		return fmt.Errorf("unknow metrics type")
	}
}
