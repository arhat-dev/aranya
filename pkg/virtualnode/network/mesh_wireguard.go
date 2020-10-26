package network

import (
	"encoding/base64"
	"net"
	"strconv"
	"time"

	"arhat.dev/abbot-proto/abbotgopb"
	"arhat.dev/pkg/log"
	"golang.org/x/crypto/curve25519"
	"k8s.io/apimachinery/pkg/util/sets"
)

type WireguardOpts struct {
	PrivateKey   string
	LogLevel     string
	PreSharedKey string
	Keepalive    time.Duration

	ListenPort   int32
	RoutingTable int32
	FirewallMark int32
}

func newWireguardMeshDriver(
	logger log.Interface,
	provider, publicIP string,
	addresses []string,
	options *WireguardOpts,
) MeshDriver {
	return &wireguardMeshDriver{
		logger:    logger.WithFields(log.String("driver", "wireguard")),
		provider:  provider,
		publicIP:  publicIP,
		addresses: addresses,

		options: options,
	}
}

type wireguardMeshDriver struct {
	logger    log.Interface
	provider  string
	publicIP  string
	addresses []string

	options *WireguardOpts
}

func (d *wireguardMeshDriver) GenerateEnsureRequest(
	ifname string, mtu int32,
	cloudMembers, edgeMembers [][]*abbotgopb.HostNetworkInterface,
) *abbotgopb.HostNetworkConfigEnsureRequest {
	var peers []*abbotgopb.DriverWireguard_Peer

	// check cloud members
	for _, memberIfaces := range cloudMembers {
		addresses := sets.NewString(d.publicIP)

		var managedIfaces []*abbotgopb.HostNetworkInterface
		for i, iface := range memberIfaces {
			if iface.Provider == d.provider {
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

			if len(pk) != wireguardKeyLength {
				d.logger.I("invalid wireguard cloud member private key length")
				continue
			}

			pubKey := base64.StdEncoding.EncodeToString(wireguardKey(pk).PublicKey())
			for _, addr := range addresses.List() {
				peers = append(peers, &abbotgopb.DriverWireguard_Peer{
					PublicKey:    pubKey,
					PreSharedKey: d.options.PreSharedKey,
					Endpoint:     net.JoinHostPort(addr, strconv.FormatInt(int64(md.Wireguard.ListenPort), 10)),

					PersistentKeepaliveInterval: int32(d.options.Keepalive.Seconds()),

					AllowedIps: []string{
						// TODO: add addresses
					},
				})
			}
		}
	}

	// edge members may only have private addresses
	for _, memberIfaces := range edgeMembers {
		addresses := sets.NewString()

		var managedIfaces []*abbotgopb.HostNetworkInterface
		for i, iface := range memberIfaces {
			if iface.Provider == d.provider {
				// managed interfaces don't have ip address accessible from outside
				managedIfaces = append(managedIfaces, memberIfaces[i])
				continue
			}

			for _, addr := range iface.Metadata.Addresses {
				ip, _, err := net.ParseCIDR(addr)
				if err != nil {
					continue
				}

				if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
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
				d.logger.D("no listen port in managed edge wireguard interface")
				continue
			}

			pk, err := base64.StdEncoding.DecodeString(md.Wireguard.PrivateKey)
			if err != nil {
				d.logger.I("invalid wireguard edge member private key", log.Error(err))
				continue
			}

			if len(pk) != wireguardKeyLength {
				d.logger.I("invalid wireguard edge member private key length")
				continue
			}

			pubKey := base64.StdEncoding.EncodeToString(wireguardKey(pk).PublicKey())
			for _, addr := range addresses.List() {
				peers = append(peers, &abbotgopb.DriverWireguard_Peer{
					PublicKey:    pubKey,
					PreSharedKey: d.options.PreSharedKey,
					Endpoint:     net.JoinHostPort(addr, strconv.FormatInt(int64(md.Wireguard.ListenPort), 10)),

					PersistentKeepaliveInterval: int32(d.options.Keepalive.Seconds()),

					AllowedIps: []string{
						// TODO: add cluster network address
					},
				})
			}
		}
	}

	return abbotgopb.NewHostNetworkConfigEnsureRequest(&abbotgopb.HostNetworkInterface{
		Metadata: &abbotgopb.NetworkInterface{
			Name:            ifname,
			Mtu:             mtu,
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

const (
	wireguardKeyLength = 32
)

type wireguardKey []byte

func (k wireguardKey) PublicKey() wireguardKey {
	var (
		pub  [wireguardKeyLength]byte
		priv [wireguardKeyLength]byte
	)

	_ = copy(priv[:], k)
	curve25519.ScalarBaseMult(&pub, &priv)

	return pub[:]
}
