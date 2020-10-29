package mesh

import "arhat.dev/abbot-proto/abbotgopb"

type Driver interface {
	GenerateEnsureRequest(
		// os (GOOS)
		os string,

		// CIDRs for wireguard mesh devices (configured in aranya config)
		meshCIDRs []string,

		// key: provider
		// value: allowed ips (including pod CIDRs)
		peerCIDRs map[string][]string,

		// members in this mesh
		cloudMembers, edgeMembers [][]*abbotgopb.HostNetworkInterface,
	) *abbotgopb.HostNetworkConfigEnsureRequest
}
