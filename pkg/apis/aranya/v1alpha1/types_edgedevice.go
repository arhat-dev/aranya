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

package v1alpha1

import (
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/constant"
)

type (
	// FiledHook to update a node filed (currently only annotations/labels supported)
	// if aranya was able to perform a successful query on some field(s) (all node object fields)
	FieldHook struct {
		// Query in jq syntax, result is always an array of unstructured objects ([]interface{})
		Query string `json:"query"`

		// TargetFieldPath
		TargetFieldPath string `json:"targetFieldPath"`

		// Value the plain text value for target field
		// +optional
		Value string `json:"value"`

		// ValueExpression jq expression applied to values found by Query, only the first result
		// will be used as the final value
		// +optional
		ValueExpression string `json:"valueExpression"`
	}

	// NodeSpec is the node related info for each EdgeDevice
	NodeSpec struct {
		// +optional
		Timers NodeTimers `json:"timers,omitempty"`

		// Cert for this node
		// +optional
		Cert NodeCertSpec `json:"cert,omitempty"`

		// Labels expected to be applied to node object
		// +optional
		Labels map[string]string `json:"labels,omitempty"`

		// Annotations expected to be applied to node object
		// +optional
		Annotations map[string]string `json:"annotations,omitempty"`

		// Taints add extra taints in addition to default taint arhat.dev/namespace=<EdgeDevice Namespace>:NoSchedule
		// +optional
		Taints []corev1.Taint `json:"taints,omitempty"`

		// +optional
		Metrics *MetricsConfig `json:"metrics,omitempty"`

		// FieldHooks to update node object attributes according to
		// current field value
		// +optional
		FieldHooks []FieldHook `json:"fieldHooks,omitempty"`

		// RBAC setup for this node, an override of aranya configuration
		//
		// +optional
		RBAC NodeRBACSpec `json:"rbac,omitempty"`
	}

	NodeTimers struct {
		// +optional
		ForceSyncInterval *metav1.Duration `json:"forceSyncInterval,omitempty"`
	}

	// NodeCertSpec is the extra location info for kubelet node cert
	NodeCertSpec struct {
		// +optional
		Country string `json:"country,omitempty"`
		// +optional
		State string `json:"state,omitempty"`
		// +optional
		Locality string `json:"locality,omitempty"`
		// +optional
		Org string `json:"org,omitempty"`
		// +optional
		OrgUnit string `json:"orgUnit,omitempty"`
	}

	NodeRBACSpec struct {
		// ClusterRolePermissions set cluster roles and their verbs for this node
		//
		// +optional
		ClusterRolePermissions map[string]NodeClusterRolePermissions `json:"clusterRolePermissions,omitempty"`
	}

	NodeClusterRolePermissions struct {
		// NodeVerbs allowed verbs for this node
		//
		// +optional
		NodeVerbs []string `json:"nodeVerbs,omitempty" yaml:"nodeVerbs"`

		// StatusVerbs allowed verbs for this nodeStatus
		//
		// +optional
		StatusVerbs []string `json:"statusVerbs,omitempty" yaml:"statusVerbs"`
	}
)

type (
	PodSpec struct {
		// +optional
		Timers PodTimers `json:"timers,omitempty"`

		// DNS config for edge device
		// +optional
		DNS *PodDNSConfig `json:"dns,omitempty"`

		// IPv4CIDR pod ipv4 range in this edge device
		// +optional
		IPv4CIDR string `json:"ipv4CIDR,omitempty"`

		// +optional
		IPv6CIDR string `json:"ipv6CIDR,omitempty"`

		// +optional
		Metrics *MetricsConfig `json:"metrics,omitempty"`

		// +kubebuilder:validation:Minimum=0
		// +optional
		Allocatable int `json:"allocatable,omitempty"`

		// +optional
		RBAC PodRBACSpec `json:"rbac,omitempty"`
	}

	PodTimers struct {
		// +optional
		ForceSyncInterval *metav1.Duration `json:"forceSyncInterval,omitempty"`
	}

	// PodDNSConfig for edge device
	PodDNSConfig struct {
		// A list of DNS name server IP addresses.
		// +optional
		Servers []string `json:"servers,omitempty" yaml:"servers"`

		// A list of DNS search domains for host-name lookup.
		// +optional
		Searches []string `json:"searches,omitempty" yaml:"searches"`

		// A list of DNS resolver options.
		// +optional
		Options []string `json:"options,omitempty" yaml:"options"`
	}

	PodRBACSpec struct {
		// ClusterRolePermissions for the pods admitted by this edge device
		//
		// +optional
		RolePermissions map[string]PodRolePermissions `json:"rolePermissions,omitempty"`

		// VirtualPodRolePermissions for host related operations (will restrict resource name to edge device's name)
		//
		// +optional
		VirtualPodRolePermissions map[string]PodRolePermissions `json:"virtualpodRolePermissions,omitempty"`
	}

	PodRolePermissions struct {
		// PodVerbs allowed verbs for resource pods
		//
		// +optional
		PodVerbs []string `json:"podVerbs,omitempty" yaml:"podVerbs"`

		// PodStatus allowed verbs for resource pods/status
		//
		// +optional
		StatusVerbs []string `json:"statusVerbs,omitempty" yaml:"statusVerbs"`

		// AllowExec to allow "create" for resource pods/exec
		AllowExec *bool `json:"allowExec,omitempty" yaml:"allowExec"`

		// AllowAttach to allow "create" for resource pods/attach
		AllowAttach *bool `json:"allowAttach,omitempty" yaml:"allowAttach"`

		// AllowPortForward to allow "create" for resource pods/port-forward
		AllowPortForward *bool `json:"allowPortForward,omitempty" yaml:"allowPortForward"`

		// AllowLog to allow "create" for resource pods/log
		AllowLog *bool `json:"allowLog,omitempty" yaml:"allowLog"`
	}
)

func FlagsForPodDNSConfig(prefix string, config *PodDNSConfig) *pflag.FlagSet {
	flags := pflag.NewFlagSet("edgedevice.pod.dns", pflag.ExitOnError)

	flags.StringSliceVar(&config.Servers, prefix+"servers",
		[]string{constant.DefaultNameServer1, constant.DefaultNameServer2}, "set default pod name servers")
	flags.StringSliceVar(&config.Searches, prefix+"searches",
		[]string{constant.DefaultSearchDomain1}, "set default pod search domains")
	flags.StringSliceVar(&config.Options, prefix+"options",
		[]string{constant.DefaultDNSOption1}, "set default pod dns options")

	return flags
}

type (
	MetricsConfig struct {
		// Enabled flag to enable metrics collection
		// +optional
		Enabled bool `json:"enabled,omitempty" yaml:"enabled"`

		// Collect is the list of prometheus node-exporter collectors to use
		// +optional
		Collect []string `json:"collect,omitempty" yaml:"collect"`

		// ExtraArgs to provide additional cli args for node_exporter
		// +optional
		ExtraArgs []string `json:"extraArgs,omitempty" yaml:"extraArgs"`
	}
)

func FlagsForMetricsConfig(prefix string, config *MetricsConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("edgedevice.metrics", pflag.ExitOnError)

	fs.BoolVar(&config.Enabled, prefix+"enabled", true, "enabled metrics collection")
	fs.StringSliceVar(&config.Collect, prefix+"collect", nil, "set collectors used")
	fs.StringSliceVar(&config.ExtraArgs, prefix+"extraArgs", nil, "set extra args for exporter")

	return fs
}

type (
	// StorageSpec
	StorageSpec struct {
		// +optional
		Enabled bool `json:"enabled"`
	}
)

type (
	NetworkSpec struct {
		// +optional
		Enabled bool `json:"enabled"`

		// +optional
		Mesh NetworkMeshSpec `json:"mesh"`
	}

	NetworkMeshSpec struct {
		// +optional
		InterfaceName string `json:"interfaceName"`

		// +kubebuilder:validation:Minimum=0
		// +optional
		MTU int `json:"mtu"`

		// ListenPort for receiving traffic from other mesh members
		// +kubebuilder:validation:Minimum=0
		// +optional
		ListenPort int `json:"port"`

		// +optional
		RoutingTable int `json:"routingTable"`

		// +optional
		FirewallMark int `json:"firewallMark"`

		// IPv4 address of the mesh interface endpoint (usually the vpn endpoint address)
		// +optional
		IPv4Addr string `json:"ipv4Addr"`

		// IPv6 address of the mesh interface endpoint (usually the vpn endpoint address)
		// +optional
		IPv6Addr string `json:"ipv6Addr"`

		// ExtraAllowedCIDRs in addition to pod CIDR(s)
		// +optional
		ExtraAllowedCIDRs []string `json:"extraAllowedCIDRs"`
	}
)

// EdgeDeviceSpec defines the desired state of EdgeDevice
type EdgeDeviceSpec struct {
	// Connectivity config how we serve the according agent
	Connectivity ConnectivitySpec `json:"connectivity,omitempty"`

	// Node related settings for kubernetes node resource
	// +optional
	Node NodeSpec `json:"node,omitempty"`

	// Pod related settings for kubernetes pod creation in edge device
	// +optional
	Pod PodSpec `json:"pod,omitempty"`

	// Storage settings for remote CSI integration
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// Network settings for network mesh in same namespace
	// +optional
	Network NetworkSpec `json:"network,omitempty"`

	// Peripherals managed by this EdgeDevice
	// +optional
	Peripherals []PeripheralSpec `json:"peripherals,omitempty"`

	// MetricsReporters are virtual peripherals used to push metrics collected from
	// peripherals
	// +optional
	MetricsReporters []PeripheralBaseSpec `json:"metricsReporters,omitempty"`
}

type (
	NetworkStatus struct {
		// +optional
		PodCIDRv4 string `json:"podCIDRv4"`

		// +optional
		PodCIDRv6 string `json:"podCIDRv6"`

		// +optional
		MeshIPv4Addr string `json:"meshIPv4Addr"`

		// +optional
		MeshIPv6Addr string `json:"meshIPv6Addr"`
	}
)

// EdgeDeviceStatus defines the observed state of EdgeDevice
type EdgeDeviceStatus struct {
	// +optional
	HostNode string `json:"hostNode"`

	// +optional
	Network NetworkStatus `json:"network"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EdgeDevice is the Schema for the edgedevices API
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced,shortName=edev
// +kubebuilder:printcolumn:name="Connectivity",type=string,JSONPath=".spec.connectivity.method"
// +kubebuilder:printcolumn:name="Host-Node",type=string,JSONPath=".status.hostNode"
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=edgedevices,shortName={ed,eds}
type EdgeDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeDeviceSpec   `json:"spec,omitempty"`
	Status EdgeDeviceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EdgeDeviceList contains a list of EdgeDevice
type EdgeDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeDevice `json:"items"`
}
