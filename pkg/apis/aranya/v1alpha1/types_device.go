package v1alpha1

import corev1 "k8s.io/api/core/v1"

// +kubebuilder:validation:Enum=WithNodeMetrics;WithArhatConnectivity;WithStandaloneClient
type DeviceMetricsReportMethod string

const (
	// ReportViaNodeMetrics will report metrics when arhat receive node metrics collect command
	ReportViaNodeMetrics DeviceMetricsReportMethod = "node"

	// ReportViaArhatConnectivity will publish data using arhat's connectivity
	// useful when you want realtime metrics but do not want to create
	//
	// (not supported if client connectivity method is gRPC)
	ReportViaArhatConnectivity DeviceMetricsReportMethod = "arhat"

	// ReportViaStandaloneClient will create a connectivity client for metrics reporting
	ReportViaStandaloneClient DeviceMetricsReportMethod = "standalone"
)

// +kubebuilder:validation:Enum=counter;"";gauge;unknown
type DeviceMetricValueType string

const (
	DeviceMetricsValueTypeGauge   DeviceMetricValueType = "gauge"
	DeviceMetricsValueTypeCounter DeviceMetricValueType = "counter"
	DeviceMetricsValueTypeUnknown DeviceMetricValueType = "unknown"
)

type (
	BaseDeviceSpec struct {
		// Name of the physical device, and this name will become available in virtual pod as a container name
		// NOTE: name `host` is reserved by the aranya
		// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?
		Name string `json:"name"`

		// Connector instructs how to connect this device
		Connector DeviceConnectivity `json:"connector,omitempty"`
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

	// DeviceSpec is the physical device related to (managed by) this edge device (e.g. sensors, switches)
	DeviceSpec struct {
		BaseDeviceSpec `json:",inline"`

		// Operations supported by this device
		// +optional
		// +listType=map
		// +listMapKey=name
		Operations []DeviceOperation `json:"operations,omitempty"`

		// Metrics collection/report from this device
		// +optional
		// +listType=map
		// +listMapKey=name
		Metrics []DeviceMetric `json:"metrics,omitempty"`
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

	// DeviceMetric to upload device metrics for prometheus
	DeviceMetric struct {
		// Name of the metrics for prometheus
		// +kubebuilder:validation:Pattern=[a-z0-9]([_a-z0-9]*[a-z0-9])?
		Name string `json:"name"`

		// ValueType of this metric
		ValueType DeviceMetricValueType `json:"valueType,omitempty"`

		// DeviceParams to override ..connectivity.params to retrieve this metric
		// +optional
		DeviceParams map[string]string `json:"deviceParams,omitempty"`

		// ReportMethod for this metrics
		// +optional
		ReportMethod DeviceMetricsReportMethod `json:"reportMethod,omitempty"`

		// ReporterName the name reference to a metrics reporter used when ReportMethod is standalone
		// +optional
		ReporterName string `json:"reporterName,omitempty"`

		// ReporterParams
		// +optional
		ReporterParams map[string]string `json:"reporterParams,omitempty"`
	}
)

type (
	DeviceConnectivityTLSConfig struct {
		// +optional
		ServerName string `json:"serverName"`

		// CertSecretRef for pem encoded x.509 certificate key pair
		// +optional
		CertSecretRef *corev1.LocalObjectReference `json:"certSecretRef,omitempty"`

		// +optional
		CipherSuites []string `json:"cipherSuites,omitempty"`

		// +optional
		InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

		MinVersion string `json:"minVersion,omitempty"`

		MaxVersion string `json:"maxVersion,omitempty"`

		NextProtos []string `json:"nextProtos,omitempty"`

		// +optional
		PreSharedKey DeviceConnectivityTLSPreSharedKey `json:"preSharedKey,omitempty"`
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
