package mesh

import (
	"encoding/base64"
	"net"
	"strconv"
	"strings"

	"arhat.dev/abbot-proto/abbotgopb"
	"arhat.dev/pkg/log"
	"golang.org/x/crypto/curve25519"
	"k8s.io/apimachinery/pkg/util/sets"

	"arhat.dev/aranya/pkg/constant"
)

type WireguardOpts struct {
	PrivateKey   string
	LogLevel     string
	PreSharedKey string

	KeepaliveSeconds int32
	ListenPort       int32
	RoutingTable     int32
	FirewallMark     int32
}

func NewWireguardMeshDriver(
	logger log.Interface,
	ifname string,
	mtu int,
	provider string,
	addresses []string,
	publicAddresses []string,
	options *WireguardOpts,
) Driver {
	return &WireguardMeshDriver{
		logger: logger.WithFields(log.String("driver", "wireguard")),

		ifname:          ifname,
		mtu:             mtu,
		provider:        provider,
		addresses:       addresses,
		publicAddresses: publicAddresses,

		options: options,
	}
}

type WireguardMeshDriver struct {
	logger log.Interface

	ifname          string
	mtu             int
	provider        string
	addresses       []string
	publicAddresses []string

	options *WireguardOpts
}

// nolint:gocyclo
func (d *WireguardMeshDriver) GenerateEnsureRequest(
	// os (GOOS)
	os string,

	// CIDRs for wireguard mesh
	meshCIDRs []string,

	// key: provider
	// value: allowed ips (including pod CIDRs)
	peerCIDRs map[string][]string,

	// members in this mesh
	cloudMembers, edgeMembers [][]*abbotgopb.HostNetworkInterface,
) *abbotgopb.HostNetworkConfigEnsureRequest {
	var peers []*abbotgopb.DriverWireguard_Peer

	// check cloud members
	for _, memberIfaces := range cloudMembers {
		addresses := sets.NewString(d.publicAddresses...)

		var managedIfaces []*abbotgopb.HostNetworkInterface
		for i, iface := range memberIfaces {
			if iface.Provider == constant.PrefixMeshInterfaceProviderAranya+constant.WatchNS() {
				// managed interfaces don't have ip address accessible from outside
				managedIfaces = append(managedIfaces, memberIfaces[i])
				continue
			}

			for _, addr := range iface.Metadata.Addresses {
				ip, _, err := net.ParseCIDR(addr)
				if err != nil {
					continue
				}

				if isPrivateIP(ip) {
					continue
				}

				addresses.Insert(ip.String())
			}
		}

		for _, iface := range managedIfaces {
			md, ok := iface.Config.(*abbotgopb.HostNetworkInterface_Wireguard)
			if !ok {
				continue
			}

			if md.Wireguard.ListenPort <= 0 {
				d.logger.D("no listen port in managed cloud wireguard interface")
				continue
			}

			pk, err := base64.StdEncoding.DecodeString(md.Wireguard.PrivateKey)
			if err != nil {
				d.logger.I("invalid wireguard cloud member private key encoding", log.Error(err))
				continue
			}

			if len(pk) != constant.WireguardKeyLength {
				d.logger.I("invalid wireguard cloud member private key length")
				continue
			}

			for _, addr := range addresses.List() {
				peers = append(peers, &abbotgopb.DriverWireguard_Peer{
					PublicKey:    base64.StdEncoding.EncodeToString(wireguardKey(pk).PublicKey()),
					PreSharedKey: d.options.PreSharedKey,
					Endpoint:     net.JoinHostPort(addr, strconv.FormatInt(int64(md.Wireguard.ListenPort), 10)),

					PersistentKeepaliveInterval: d.options.KeepaliveSeconds,

					AllowedIps: append(append([]string{}, meshCIDRs...), peerCIDRs[iface.Provider]...),
				})
			}
		}
	}

	ifname := "wg"
	switch os {
	case "openbsd":
		ifname = "tun"
	case "darwin":
		ifname = "utun"
	}

	// edge members may only have private addresses but may be accessible from other edge devices
memberLoop:
	for _, memberIfaces := range edgeMembers {
		memberAddresses := sets.NewString()

		var managedIfaces []*abbotgopb.HostNetworkInterface
		for i, iface := range memberIfaces {
			if iface.Provider == d.provider {
				// it's me, check driver

				if _, ok := iface.Config.(*abbotgopb.HostNetworkInterface_Wireguard); ok {
					// note its interface name and ignore this member
					if iface.Metadata.Name != "" {
						ifname = iface.Metadata.Name
					}
				}

				continue memberLoop
			}

			if strings.HasPrefix(iface.Provider, constant.PrefixMeshInterfaceProviderAranya) {
				// managed interfaces don't have ip address accessible from outside
				managedIfaces = append(managedIfaces, memberIfaces[i])
			}

			for _, addr := range iface.Metadata.Addresses {
				ip, _, err := net.ParseCIDR(addr)
				if err != nil {
					continue
				}

				if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
					continue
				}

				memberAddresses.Insert(ip.String())
			}
		}

		for _, iface := range managedIfaces {
			md, ok := iface.Config.(*abbotgopb.HostNetworkInterface_Wireguard)
			if !ok {
				continue
			}

			logger := d.logger.WithFields(log.String("provider", iface.Provider))
			if md.Wireguard.ListenPort <= 0 {
				logger.D("no listen port in edge member wireguard interface")
				continue
			}

			pk, err := base64.StdEncoding.DecodeString(md.Wireguard.PrivateKey)
			if err != nil {
				logger.I("invalid wireguard edge member private key", log.Error(err))
				continue
			}

			if len(pk) != constant.WireguardKeyLength {
				logger.I("invalid wireguard edge member private key length")
				continue
			}

			pubKey := base64.StdEncoding.EncodeToString(wireguardKey(pk).PublicKey())
			for _, addr := range memberAddresses.List() {
				peers = append(peers, &abbotgopb.DriverWireguard_Peer{
					PublicKey:    pubKey,
					PreSharedKey: d.options.PreSharedKey,
					Endpoint:     net.JoinHostPort(addr, strconv.FormatInt(int64(md.Wireguard.ListenPort), 10)),

					PersistentKeepaliveInterval: d.options.KeepaliveSeconds,

					AllowedIps: []string{
						// TODO: add cluster network address
					},
				})
			}
		}
	}

	if d.ifname != "" {
		ifname = d.ifname
	}

	return abbotgopb.NewHostNetworkConfigEnsureRequest(&abbotgopb.HostNetworkInterface{
		Metadata: &abbotgopb.NetworkInterface{
			Name:            ifname,
			Mtu:             int32(d.mtu),
			HardwareAddress: "",
			Addresses:       d.addresses,
		},
		Provider: d.provider,
		Config: &abbotgopb.HostNetworkInterface_Wireguard{
			Wireguard: &abbotgopb.DriverWireguard{
				LogLevel:   d.options.LogLevel,
				PrivateKey: d.options.PrivateKey,
				Peers:      peers,
				ListenPort: d.options.ListenPort,
				Routing: &abbotgopb.DriverWireguard_Routing{
					Enabled:      true,
					Table:        d.options.RoutingTable,
					FirewallMark: d.options.FirewallMark,
				},
			},
		},
	})
}

type wireguardKey []byte

func (k wireguardKey) PublicKey() wireguardKey {
	var (
		pub  [constant.WireguardKeyLength]byte
		priv [constant.WireguardKeyLength]byte
	)

	_ = copy(priv[:], k)
	curve25519.ScalarBaseMult(&pub, &priv)

	return pub[:]
}
