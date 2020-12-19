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

import corev1 "k8s.io/api/core/v1"

// +kubebuilder:validation:Enum=WithNodeMetrics;WithArhatConnectivity;WithStandaloneClient
type PeripheralMetricsReportMethod string

const (
	// ReportViaNodeMetrics will report metrics when arhat receive node metrics collect command
	ReportViaNodeMetrics PeripheralMetricsReportMethod = "node"

	// ReportViaArhatConnectivity will publish data using arhat's connectivity
	// useful when you want realtime metrics but do not want to create standalone metrics reporter
	ReportViaArhatConnectivity PeripheralMetricsReportMethod = "arhat"

	// ReportViaStandaloneClient will create a connectivity client for metrics reporting
	ReportViaStandaloneClient PeripheralMetricsReportMethod = "standalone"
)

// +kubebuilder:validation:Enum=counter;"";gauge;unknown
type PeripheralMetricValueType string

const (
	PeripheralMetricValueTypeGauge   PeripheralMetricValueType = "gauge"
	PeripheralMetricValueTypeCounter PeripheralMetricValueType = "counter"
	PeripheralMetricValueTypeUnknown PeripheralMetricValueType = "unknown"
)

type (
	PeripheralBaseSpec struct {
		// Name of the peripheral, and this name will become available in virtual pod as a container name
		// NOTE: name `host` is reserved by the aranya
		// +kubebuilder:validation:Pattern=[a-z0-9]([-a-z0-9]*[a-z0-9])?
		Name string `json:"name"`

		// Connector instructs how to connect this peripheral
		Connector PeripheralConnectivitySpec `json:"connector,omitempty"`
	}

	// PeripheralConnectivitySpec configure how to connect the peripheral
	PeripheralConnectivitySpec struct {
		// Method interacting with this peripheral
		Method string `json:"method,omitempty"`

		// Target value for transport, its value depends on the transport method you chose
		Target string `json:"target,omitempty"`

		// Params for this connectivity (can be overridden by the )
		// +optional
		Params map[string]string `json:"params,omitempty"`

		// TLS config for network related connectivity
		// +optional
		TLS *PeripheralConnectivityTLSConfig `json:"tls,omitempty"`
	}

	// PeripheralSpec is the peripheral to be managed by this edge device (e.g. sensors, switches)
	PeripheralSpec struct {
		PeripheralBaseSpec `json:",inline"`

		// Operations supported by this peripheral
		// +optional
		// +listType=map
		// +listMapKey=name
		Operations []PeripheralOperationSpec `json:"operations,omitempty"`

		// Metrics collection/report from this peripheral
		// +optional
		// +listType=map
		// +listMapKey=name
		Metrics []PeripheralMetricSpec `json:"metrics,omitempty"`
	}

	// PeripheralOperationSpec defines operation we can perform on the peripheral
	PeripheralOperationSpec struct {
		// Name of the operation (e.g. "on", "off" ...)
		Name string `json:"name"`

		// PseudoCommand used to trigger this operation, so you can trigger this operation by executing
		// `kubectl exec <virtual pod> -c <peripheral name> -- <pseudo command>`
		// Defaults to operation name
		// +optional
		PseudoCommand string `json:"pseudoCommand,omitempty"`

		// Params to override ..connector.params
		// +optional
		Params map[string]string `json:"params,omitempty"`
	}

	// PeripheralMetricSpec to upload peripheral metrics for prometheus
	PeripheralMetricSpec struct {
		// Name of the metrics for prometheus
		// +kubebuilder:validation:Pattern=[a-z0-9]([_a-z0-9]*[a-z0-9])?
		Name string `json:"name"`

		// ValueType of this metric
		ValueType PeripheralMetricValueType `json:"valueType,omitempty"`

		// Params to override ..connectivity.params to retrieve this metric
		// +optional
		Params map[string]string `json:"params,omitempty"`

		// ReportMethod for this metrics
		// +optional
		ReportMethod PeripheralMetricsReportMethod `json:"reportMethod,omitempty"`

		// ReporterName the name reference to a metrics reporter used when ReportMethod is standalone
		// +optional
		ReporterName string `json:"reporterName,omitempty"`

		// ReporterParams
		// +optional
		ReporterParams map[string]string `json:"reporterParams,omitempty"`
	}
)

type (
	PeripheralConnectivityTLSConfig struct {
		// +optional
		ServerName string `json:"serverName"`

		// CertSecretRef for pem encoded x.509 certificate key pair
		// +optional
		CertSecretRef *corev1.LocalObjectReference `json:"certSecretRef,omitempty"`

		// +optional
		CipherSuites []string `json:"cipherSuites,omitempty"`

		// +optional
		InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`

		// +optional
		MinVersion string `json:"minVersion,omitempty"`

		// +optional
		MaxVersion string `json:"maxVersion,omitempty"`

		// +optional
		NextProtos []string `json:"nextProtos,omitempty"`

		// +optional
		PreSharedKey PeripheralConnectivityTLSPreSharedKey `json:"preSharedKey,omitempty"`
	}

	PeripheralConnectivityTLSPreSharedKey struct {
		// map server hint(s) to pre shared key(s)
		// column separated base64 encoded key value pairs
		// +optional
		ServerHintMapping []string `json:"serverHintMapping"`
		// the client hint provided to server, base64 encoded value
		// +optional
		IdentityHint string `json:"identityHint"`
	}
)
