package network

import (
	"context"
	"sync"
	"sync/atomic"

	"arhat.dev/pkg/reconcile"

	"arhat.dev/aranya/pkg/mesh"
	"arhat.dev/aranya/pkg/util/manager"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Options struct {
	InterfaceName string
	MTU           int
	Provider      string
	Addresses     []string

	ExtraAllowedCIDRs []string
	PublicAddresses   []string

	WireguardOpts *mesh.WireguardOpts
}

func NewManager(
	ctx context.Context,
	nodeName string,
	connectivityMgr connectivity.Manager,
	options *Options,
) *Manager {

	mgr := &Manager{
		BaseManager: manager.NewBaseManager(ctx, nodeName, connectivityMgr),

		meshDriver: nil,

		podIPv4CIDRStore: new(atomic.Value),
		podIPv6CIDRStore: new(atomic.Value),

		options: options,

		initialized: make(chan struct{}),
	}

	mgr.podIPv4CIDRStore.Store("")
	mgr.podIPv6CIDRStore.Store("")

	mgr.netRec = reconcile.NewCore(mgr.Context(), reconcile.Options{
		Logger:          mgr.Log,
		BackoffStrategy: nil,
		Workers:         1,
		RequireCache:    false,
		Handlers: reconcile.HandleFuncs{
			OnAdded:    nil,
			OnUpdated:  nil,
			OnDeleting: nil,
			OnDeleted:  nil,
		},
	}.ResolveNil())

	switch {
	case options.WireguardOpts != nil:
		mgr.meshDriver = mesh.NewWireguardMeshDriver(
			mgr.Log,
			options.InterfaceName,
			options.MTU,
			options.Provider,
			options.Addresses,
			options.PublicAddresses,
			options.WireguardOpts,
		)
	default:
		// no mesh driver
	}

	return mgr
}

type Manager struct {
	*manager.BaseManager

	meshDriver mesh.Driver

	netRec *reconcile.Core

	podIPv4CIDRStore *atomic.Value
	podIPv6CIDRStore *atomic.Value

	options *Options

	initialized chan struct{}

	mu *sync.RWMutex
}

func (m *Manager) Start() error {
	return m.OnStart(func() error {
		// nolint:staticcheck
		if m.meshDriver != nil {
			// detect mtu and determine mesh device name
		}

		// nolint:gosimple
		select {
		case <-m.Context().Done():
		}

		return nil
	})
}

func (m *Manager) Close() {
	m.OnClose(nil)
}

func (m *Manager) Initialized() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.initialized
}

// Retrieve allowed CIDRs including pod CIDRs
func (m *Manager) AllowedCIDRs() []string {
	allCIDRs := append([]string{}, m.options.ExtraAllowedCIDRs...)
	ipv4, ipv6 := m.GetPodCIDR(false), m.GetPodCIDR(true)
	if ipv4 != "" {
		allCIDRs = append(allCIDRs, ipv4)
	}

	if ipv6 != "" {
		allCIDRs = append(allCIDRs, ipv6)
	}

	return allCIDRs
}

func (m *Manager) SetPodCIDRs(ipv4, ipv6 string) {
	var updated bool
	if ipv4 != "" && ipv4 != m.GetPodCIDR(false) {
		m.podIPv4CIDRStore.Store(ipv4)
		updated = true
	}

	if ipv6 != "" && ipv6 != m.GetPodCIDR(true) {
		m.podIPv6CIDRStore.Store(ipv6)
		updated = true
	}

	_ = updated
	// if updated {
	// 	// offer will fail only because work duplicated
	// 	//_ = m.networkJobQ.Offer(queue.Job{Action: queue.ActionUpdate, Key: "TBD"})
	// }
}

func (m *Manager) GetPodCIDR(ipv6 bool) string {
	if ipv6 {
		return m.podIPv6CIDRStore.Load().(string)
	}
	return m.podIPv4CIDRStore.Load().(string)
}

// nolint:unused
func (m *Manager) hasPodCIDR() bool {
	return m.GetPodCIDR(false) != "" || m.GetPodCIDR(true) != ""
}
