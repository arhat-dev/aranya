// +build !rt_none

package aranyagopb

func NewContainerNetworkStatusMsg(ipv4CIDR, ipv6CIDR string) *ContainerNetworkStatusMsg {
	return &ContainerNetworkStatusMsg{
		Ipv4Cidr: ipv4CIDR,
		Ipv6Cidr: ipv6CIDR,
	}
}
