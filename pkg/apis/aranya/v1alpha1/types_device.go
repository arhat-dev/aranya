package v1alpha1

import corev1 "k8s.io/api/core/v1"

// +kubebuilder:validation:Enum=WithNodeMetrics;WithArhatConnectivity;WithStandaloneClient
type DeviceMetricsUploadMethod string

const (
	// UploadAlongWithNodeMetrics will report metrics when arhat receive node metrics collect command
	UploadWithNodeMetrics DeviceMetricsUploadMethod = "WithNodeMetrics"

	// UploadAlongWithArhatConnectivity will publish data using arhat's connectivity
	// useful when you want realtime metrics but do not want to create
	//
	// (not supported if client connectivity method is gRPC)
	UploadWithArhatConnectivity DeviceMetricsUploadMethod = "WithArhatConnectivity"

	// UploadWithStandaloneClient will create a connectivity client for metrics reporting
	UploadWithStandaloneClient DeviceMetricsUploadMethod = "WithStandaloneClient"
)

// +kubebuilder:validation:Enum=counter;"";gauge
type DeviceMetricsValueType string

const (
	DeviceMetricsValueTypeGauge   DeviceMetricsValueType = "gauge"
	DeviceMetricsValueTypeCounter DeviceMetricsValueType = "counter"
	DeviceMetricsValueTypeUnknown DeviceMetricsValueType = ""
)

type (
	// DeviceSpec is the physical device related to (managed by) this edge device (e.g. sensors, switches)
	DeviceSpec struct {
		// Name of the physical device, and this name will become available in virtual pod as a container name
		// NOTE: name `host` is reserved by the aranya
		// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?
		Name string `json:"name"`

		// Connectivity instructs how to connect to this device
		Connectivity DeviceConnectivity `json:"connectivity,omitempty"`

		// Operations supported by this device
		// +optional
		// +listType=map
		// +listMapKey=name
		Operations []DeviceOperation `json:"operations,omitempty"`

		// Metrics collection/report from this device
		// +optional
		// +listType=map
		// +listMapKey=name
		Metrics []DeviceMetrics `json:"metrics,omitempty"`

		// UploadConnectivity instructs how to upload metrics
		// +optional
		UploadConnectivity *DeviceConnectivity `json:"uploadConnectivity,omitempty"`
	}

	// DeviceConnectivity configure how to connect the physical device
	DeviceConnectivity struct {
		// Method interacting with this physical device
		Method string `json:"method,omitempty"`

		// Target value for transport, its value depends on the transport method you chose
		Target string `json:"target,omitempty"`

		// Params for this connectivity (can be overridden by the )
		// +optional
		Params map[string]string `json:"params,omitempty"`

		// TLS config for network related connectivity
		// +optional
		TLS *DeviceConnectivityTLSConfig `json:"tls,omitempty"`
	}

	// DeviceOperation defines operation we can perform on the device
	DeviceOperation struct {
		// Name of the operation (e.g. "on", "off" ...)
		Name string `json:"name"`

		// PseudoCommand used to trigger this operation, so you can trigger this operation by executing
		// `kubectl exec <virtual pod> -c <device name> -- <pseudo command>`
		// Defaults to operation name
		// +optional
		PseudoCommand string `json:"pseudoCommand,omitempty"`

		// Params to override ..connectivity.params
		// +optional
		Params map[string]string `json:"params,omitempty"`
	}

	// DeviceMetrics to upload device metrics for prometheus
	DeviceMetrics struct {
		// Name of the metrics for prometheus
		// +kubebuilder:validation:Pattern=[a-z0-9]([_a-z0-9]*[a-z0-9])?
		Name string `json:"name"`

		// ValueType of this metric
		ValueType DeviceMetricsValueType `json:"valueType,omitempty"`

		// DeviceParams to override ..connectivity.params to get metrics
		// +optional
		DeviceParams map[string]string `json:"deviceParams,omitempty"`

		// ReportMethod for this metrics
		ReportMethod DeviceMetricsUploadMethod `json:"reportMethod,omitempty"`

		// ReportParams
		// +optional
		ReportParams map[string]string `json:"reportParams,omitempty"`
	}
)

type (
	DeviceConnectivityTLSConfig struct {
		// +optional
		PreSharedKey DeviceConnectivityTLSPreSharedKey `json:"preSharedKey"`

		// +optional
		CipherSuites []string `json:"cipherSuites"`

		// +optional
		ServerName string `json:"serverName"`

		// write tls session shared key to this file
		// +optional
		KeyLogFile string `json:"keyLogFile"`

		// CertSecretRef for pem encoded x.509 certificate key pair
		// +optional
		CertSecretRef *corev1.LocalObjectReference `json:"certSecretRef,omitempty"`

		// +optional
		InsecureSkipVerify bool `json:"insecureSkipVerify"`

		// options for dtls
		// +optional
		AllowInsecureHashes bool `json:"allowInsecureHashes"`
	}

	DeviceConnectivityTLSPreSharedKey struct {
		// map server hint(s) to pre shared key(s)
		// column separated base64 encoded key value pairs
		// +optional
		ServerHintMapping []string `json:"serverHintMapping"`
		// the client hint provided to server, base64 encoded value
		// +optional
		IdentityHint string `json:"identityHint"`
	}
)
