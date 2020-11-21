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

package conf

import "time"

type VirtualnodeNetworkConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	AbbotService   VirtualnodeNetworkAbbotServiceConfig `json:"abbotService" yaml:"abbotService"`
	NetworkService VirtualnodeNetworkServiceConfig      `json:"networkService" yaml:"networkService"`
	Mesh           VirtualnodeNetworkMeshConfig         `json:"mesh" yaml:"mesh"`
	Backend        VirtualnodeNetworkBackendConfig      `json:"backend" yaml:"backend"`
}

type VirtualnodeNetworkServiceConfig struct {
	Name      string   `json:"name" yaml:"name"`
	Type      string   `json:"type" yaml:"type"`
	Addresses []string `json:"addresses" yaml:"addresses"`
	Port      int      `json:"port" yaml:"port"`
}

type VirtualnodeNetworkAbbotServiceConfig struct {
	Name     string `json:"name" yaml:"name"`
	PortName string `json:"portName" yaml:"portName"`
}

type VirtualnodeNetworkBackendConfig struct {
	Driver string `json:"driver" yaml:"driver"`

	Wireguard struct {
		Name         string        `json:"name" yaml:"name"`
		MTU          int           `json:"mtu" yaml:"mtu"`
		ListenPort   int           `json:"listenPort" yaml:"listenPort"`
		PrivateKey   string        `json:"privateKey" yaml:"privateKey"`
		PreSharedKey string        `json:"preSharedKey" yaml:"preSharedKey"`
		Keepalive    time.Duration `json:"keepalive" yaml:"keepalive"`
	} `json:"wireguard" yaml:"wireguard"`
}

type VirtualnodeNetworkMeshConfig struct {
	// allocate ip addresses to mesh network device
	IPv4Blocks []AddressBlock `json:"ipv4Blocks" yaml:"ipv4Blocks"`
	IPv6Blocks []AddressBlock `json:"ipv6Blocks" yaml:"ipv6Blocks"`
}

type AddressBlock struct {
	CIDR  string `json:"cidr"`
	Start string `json:"start"`
	End   string `json:"end"`
}
