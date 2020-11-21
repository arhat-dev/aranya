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

import (
	"github.com/spf13/pflag"

	"arhat.dev/pkg/kubehelper"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
)

type Config struct {
	Aranya      AppConfig         `json:"aranya" yaml:"aranya"`
	VirtualNode VirtualnodeConfig `json:"virtualnode" yaml:"virtualnode"`
}

// nolint:lll
type AppConfig struct {
	kubehelper.ControllerConfig `json:",inline" yaml:",inline"`

	MaxVirtualnodeCreatingInParallel int  `json:"maxVirtualnodeCreatingInParallel" yaml:"maxVirtualnodeCreatingInParallel"`
	RunAsCloudProvider               bool `json:"runAsCloudProvider" yaml:"runAsCloudProvider"`

	Managed struct {
		ConnectivityService struct {
			// aranya will only manage Service.spec.{selector, ports[].{name, port, targetPort}}
			Name string `json:"name" yaml:"name"`
		} `json:"connectivityService" yaml:"connectivityService"`

		SFTPService struct {
			// aranya will only manage Service.spec.{selector, ports[].{name, port, targetPort}}
			Name string `json:"name" yaml:"name"`
		} `json:"sftpService" yaml:"sftpService"`

		// NodeClusterRoles managed for node access
		// aranya will add nodes related to the edgedevices in watch namespace to this cluster role
		NodeClusterRoles map[string]aranyaapi.NodeClusterRolePermissions `json:"nodeClusterRoles" yaml:"nodeClusterRoles"`
		PodRoles         map[string]aranyaapi.PodRolePermissions         `json:"podRoles" yaml:"podRoles"`
		VirtualPodRoles  map[string]aranyaapi.PodRolePermissions         `json:"virtualPodRoles" yaml:"virtualPodRoles"`
	} `json:"managed" yaml:"managed"`
}

func FlagsForAranyaAppConfig(prefix string, config *AppConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("aranya.app", pflag.ExitOnError)

	flags.IntVar(&config.MaxVirtualnodeCreatingInParallel, "maxVirtualnodeCreatingInParallel", 0,
		"set how many virtualnode can be creating in parallel, values <= 0 means no limit")

	flags.StringVar(&config.Managed.SFTPService.Name, prefix+"managed.sftpService.name",
		"edgedevice-sftp", "set sftp service resource name managed by aranya (for remote csi storage feature)")
	flags.StringVar(&config.Managed.ConnectivityService.Name, prefix+"managed.connectivity.name",
		"edgedevice-connectivity",
		"set the grpc service resource name managed by aranya (for EdgeDevices with grpc connectivity method)")

	return flags
}
