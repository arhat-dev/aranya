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

package constant

import (
	"time"
)

// date formats
const (
	TimeLayout = time.RFC3339Nano
)

// default dns config
const (
	DefaultNameServer1   = "1.1.1.1"
	DefaultNameServer2   = "8.8.8.8"
	DefaultSearchDomain1 = "cluster.local"
	DefaultDNSOption1    = "ndots:5"
)

// Default file and dirs
const (
	// aranya defaults
	DefaultAranyaConfigFile  = "/etc/aranya/config.yaml"
	DefaultKubeletRegDir     = "/var/lib/kubelet/plugins_registry"
	DefaultKubeletPluginsDir = "/var/lib/kubelet/plugins"
	DefaultAranyaStorageDir  = "/var/lib/aranya"
	DefaultAranyaPodLogDir   = "/var/log/pods"
	DefaultSFTPHostKeyFile   = "/etc/ssh/ssh_host_ed25519_key"
)

// Defaults intervals
const (
	DefaultNodeLeaseDuration       = 40 * time.Second
	DefaultNodeLeaseUpdateInterval = 10 * time.Second
	DefaultMirrorNodeSyncInterval  = 10 * time.Second
)

// Default timeouts
const (
	DefaultUnarySessionTimeout   = 10 * time.Minute
	DefaultStreamIdleTimeout     = 4 * time.Hour
	DefaultStreamCreationTimeout = 30 * time.Second
)

const (
	DefaultBackoffInitialDelay = 5 * time.Second
	DefaultBackoffMaxDelay     = 30 * time.Second
	DefaultBackoffFactor       = float64(1.2)
)

const (
	DefaultInteractiveStreamReadTimeout    = 20 * time.Millisecond
	DefaultNonInteractiveStreamReadTimeout = 200 * time.Millisecond
	DefaultPortForwardStreamReadTimeout    = 50 * time.Millisecond
)

// Default channel size
const (
	DefaultConnectivityMsgChannelSize = 32
)

const (
	DefaultExitCodeOnError = 128
)

const (
	// max gRPC message size
	// https://github.com/grpc/grpc.github.io/issues/371
	MaxGRPCDataSize = 64 * 1024

	// max MQTT message size (make it compatible with aws mqtt)
	MaxMQTTDataSize = MaxAwsIoTCoreC2DDataSize
	// https://docs.aws.amazon.com/general/latest/gr/aws_service_limits.html#protocol
	MaxAwsIoTCoreD2CDataSize = 128 * 1024 // device to cloud
	MaxAwsIoTCoreC2DDataSize = 128 * 1024 // cloud to device
	// https://docs.microsoft.com/en-us/azure/iot-hub/iot-hub-devguide-quotas-throttling
	MaxAzureIoTHubD2CDataSize = 256 * 1024 // device to cloud
	MaxAzureIoTHubC2DDataSize = 64 * 1024  // cloud to device
	// https://cloud.google.com/iot/quotas
	MaxGCPIoTCoreD2CDataSize = 256 * 1024 // device to cloud
	MaxGCPIoTCoreC2DDataSize = 256 * 1024 // cloud to device

	// max CoAP message (will split into chunks)
	MaxCoAPDataSize = 64 * 1024
)
