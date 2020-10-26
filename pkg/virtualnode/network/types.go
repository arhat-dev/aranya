package network

import "arhat.dev/abbot-proto/abbotgopb"

type MeshDriver interface {
	GenerateEnsureRequest(
		ifname string,
		mtu int32,
		cloudMembers, edgeMembers [][]*abbotgopb.HostNetworkInterface,
	) *abbotgopb.HostNetworkConfigEnsureRequest
}
