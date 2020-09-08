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

	"arhat.dev/pkg/log"

	"arhat.dev/aranya-proto/aranyagopb"
	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/util/cache"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Options struct {
	NodeMetrics      *aranyaapi.MetricsConfig
	ContainerMetrics *aranyaapi.MetricsConfig
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
			if m.options.NodeMetrics.Enabled {
				m.Log.I("configuring device node metrics")
				m.configureDeviceMetricsCollection(
					aranyagopb.NewMetricsConfigureCmd(
						aranyagopb.CONFIGURE_NODE_METRICS_COLLECTION,
						&aranyagopb.MetricsConfigOptions{
							Collect:   m.options.NodeMetrics.Collect,
							ExtraArgs: m.options.NodeMetrics.ExtraArgs,
						},
					),
				)
			}

			if m.options.ContainerMetrics.Enabled {
				m.Log.I("configuring device container metrics")
				m.configureDeviceMetricsCollection(
					aranyagopb.NewMetricsConfigureCmd(
						aranyagopb.CONFIGURE_CONTAINER_METRICS_COLLECTION,
						&aranyagopb.MetricsConfigOptions{
							Collect:   m.options.ContainerMetrics.Collect,
							ExtraArgs: m.options.ContainerMetrics.ExtraArgs,
						},
					),
				)
			}

			// device connected, serve until disconnected
			select {
			case <-m.Context().Done():
				return m.Context().Err()
			case <-m.ConnectivityManager.Disconnected():
				// disable metrics collections
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

func (m *Manager) configureDeviceMetricsCollection(cmd *aranyagopb.MetricsCmd) {
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, cmd)
	if err != nil {
		m.Log.I("failed to post metrics config cmd", log.Error(err))
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if err := msg.GetError(); err != nil {
			// no more metrics cmd should be issued
			switch cmd.Action {
			case aranyagopb.CONFIGURE_NODE_METRICS_COLLECTION:
				m.Log.I("failed to configure node metrics collection", log.Error(err))
				atomic.StoreUint32(&m.supportNodeMetrics, 0)
			case aranyagopb.COLLECT_CONTAINER_METRICS:
				m.Log.I("failed to configure container metrics collection", log.Error(err))
				atomic.StoreUint32(&m.supportContainerMetrics, 0)
			}

			return true
		}

		mc := msg.GetMetrics()
		if mc == nil {
			return true
		}

		if mc.Kind == aranyagopb.METRICS_COLLECTION_CONFIGURED {
			switch cmd.Action {
			case aranyagopb.CONFIGURE_NODE_METRICS_COLLECTION:
				m.Log.D("node metrics collection configured")
				atomic.StoreUint32(&m.supportNodeMetrics, 1)
			case aranyagopb.CONFIGURE_CONTAINER_METRICS_COLLECTION:
				m.Log.D("container metrics collection configured")
				atomic.StoreUint32(&m.supportContainerMetrics, 1)
			}
		}

		return false
	}, nil, connectivity.HandleUnknownMessage(m.Log))
}

func (m *Manager) retrieveDeviceMetrics(cmd *aranyagopb.MetricsCmd) {
	msgCh, _, err := m.ConnectivityManager.PostCmd(0, cmd)
	if err != nil {
		m.Log.I("failed to post metrics collect cmd", log.Error(err))
	}

	connectivity.HandleMessages(msgCh, func(msg *aranyagopb.Msg) (exit bool) {
		if err := msg.GetError(); err != nil {
			m.Log.I("failed to get metrics", log.Error(err))
			if err.Kind == aranyagopb.ERR_NOT_SUPPORTED {
				// no more metrics cmd should be issued
				switch cmd.Action {
				case aranyagopb.COLLECT_NODE_METRICS:
					atomic.StoreUint32(&m.supportNodeMetrics, 0)
				case aranyagopb.COLLECT_CONTAINER_METRICS:
					atomic.StoreUint32(&m.supportContainerMetrics, 0)
				}
			}

			return true
		}

		mc := msg.GetMetrics()
		if mc == nil {
			return true
		}

		err = m.UpdateMetrics(mc)
		if err != nil {
			m.Log.I("failed to update metrics", log.Error(err))
		}

		return false
	}, nil, connectivity.HandleUnknownMessage(m.Log))
}

// UpdateMetrics cache the newly collected metrics
func (m *Manager) UpdateMetrics(metrics *aranyagopb.Metrics) error {
	if metrics == nil || len(metrics.Data) == 0 {
		return fmt.Errorf("empty metrics bytes")
	}

	switch metrics.Kind {
	case aranyagopb.METRICS_NODE:
		return m.nodeMetricsCache.Update(metrics.Data)
	case aranyagopb.METRICS_CONTAINER:
		return m.containerMetricsCache.Update(metrics.Data)
	default:
		return fmt.Errorf("unknown metrics kind: %v", metrics.Kind)
	}
}
