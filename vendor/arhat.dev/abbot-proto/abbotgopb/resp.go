package abbotgopb

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

func NewResponse(resp proto.Marshaler) (*Response, error) {
	if resp == nil {
		return &Response{Kind: RESP_DONE, Body: nil}, nil
	}

	var kind ResponseType
	switch resp.(type) {
	case *ContainerNetworkConfigResponse:
		kind = RESP_CTR_NETWORK_CONFIG
	case *ContainerNetworkStatusResponse:
		kind = RESP_CTR_NETWORK_STATUS
	case *ContainerNetworkStatusListResponse:
		kind = RESP_CTR_NETWORK_STATUS_LIST
	case *HostNetworkConfigResponse:
		kind = RESP_HOST_NETWORK_CONFIG
	default:
		return nil, fmt.Errorf("unkonw response type")
	}

	data, err := resp.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response body: %w", err)
	}

	return &Response{
		Kind: kind,
		Body: data,
	}, nil
}

func NewContainerNetworkConfigResponse(
	ipv4Subnet, ipv6Subnet string,
) *ContainerNetworkConfigResponse {
	return &ContainerNetworkConfigResponse{
		Ipv4Subnet: ipv4Subnet,
		Ipv6Subnet: ipv6Subnet,
	}
}

func NewContainerNetworkStatusResponse(
	pid uint32,
	interfaces []*NetworkInterface,
) *ContainerNetworkStatusResponse {
	return &ContainerNetworkStatusResponse{Interfaces: interfaces}
}

func NewContainerNetworkStatusListResponse(
	nets map[string]*ContainerNetworkStatusResponse,
) *ContainerNetworkStatusListResponse {
	return &ContainerNetworkStatusListResponse{
		ContainerNetworks: nets,
	}
}

func NewHostNetworkConfigResponse(
	interfaces ...*HostNetworkInterface,
) *HostNetworkConfigResponse {
	return &HostNetworkConfigResponse{
		Actual: interfaces,
	}
}
