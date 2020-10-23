package abbotgopb

func NewHostNetworkInterface(
	provider string,
	metadata *NetworkInterface,
	config interface{},
) *HostNetworkInterface {
	var cfg isHostNetworkInterface_Config
	switch c := config.(type) {
	case *DriverBridge:
		cfg = &HostNetworkInterface_Bridge{
			Bridge: c,
		}
	case *DriverWireguard:
		cfg = &HostNetworkInterface_Wireguard{
			Wireguard: c,
		}
	default:
		cfg = &HostNetworkInterface_Unknown{
			Unknown: &DriverUnknown{},
		}
	}

	return &HostNetworkInterface{
		Metadata: metadata,
		Provider: provider,
		Config:   cfg,
	}
}
