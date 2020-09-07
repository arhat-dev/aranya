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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MQTTSpec struct {
	// TopicNamespace to share with arhat agent
	TopicNamespace string `json:"topicNamespace,omitempty"`
	// Broker address in the form of host:port
	Broker string `json:"broker,omitempty"`
	// Version of MQTT protocol can be one of [3.1.1]
	// +kubebuilder:validation:Enum="3.1.1"
	// +optional
	Version string `json:"version,omitempty"`
	// Transport protocol underlying the MQTT protocol, one of [tcp, websocket]
	// +kubebuilder:validation:Enum=tcp;websocket
	// +optional
	Transport string `json:"transport,omitempty"`
	// +optional
	ClientID string `json:"clientID,omitempty"`
	// +optional
	Keepalive int32 `json:"keepalive,omitempty"`
	// Secret for tls cert-key pair
	// +optional
	TLSSecretRef *corev1.LocalObjectReference `json:"tlsSecretRef,omitempty"`
	// +optional
	UserPassRef *corev1.LocalObjectReference `json:"userPassRef,omitempty"`
}

type GRPCSpec struct {
	// Secret for server side tls cert-key pair
	// +optional
	TLSSecretRef *corev1.LocalObjectReference `json:"tlsSecretRef,omitempty"`
}

type (
	GCPIoTCoreSpec struct {
		ProjectID string `json:"projectID,omitempty"`

		// PubSub service used to receive messages sent from device
		PubSub GCPPubSubConfig `json:"pubSub,omitempty"`

		// CloudIoT service used to send commands to device
		CloudIoT GCPCloudIoTConfig `json:"cloudIoT,omitempty"`
	}

	GCPPubSubConfig struct {
		TelemetryTopicID        string                  `json:"telemetryTopicID,omitempty"`
		CredentialsSecretKeyRef LocalSecretKeyReference `json:"credentialsSecretKeyRef,omitempty"`

		// +optional
		StateTopicID string `json:"stateTopicID,omitempty"`
	}

	GCPCloudIoTConfig struct {
		Region                  string                  `json:"region,omitempty"`
		RegistryID              string                  `json:"registryID,omitempty"`
		DeviceID                string                  `json:"deviceID,omitempty"`
		CredentialsSecretKeyRef LocalSecretKeyReference `json:"credentialsSecretKeyRef,omitempty"`

		// poll interval to get device info, default to 1 minute
		// +optional
		DeviceStatusPollInterval metav1.Duration `json:"deviceStatusPollInterval,omitempty" swaggertype:"primitive,string"`
	}
)

type (
	AzureIoTHubSpec struct {
		// DeviceID of the iot hub device
		DeviceID string `json:"deviceID,omitempty"`

		IoTHub   AzureIoTHubConfig   `json:"iotHub,omitempty"`
		EventHub AzureEventHubConfig `json:"eventHub,omitempty"`
	}

	AzureIoTHubConfig struct {
		ConnectionStringSecretKeyRef LocalSecretKeyReference `json:"connectionStringSecretKeyRef,omitempty"`

		// poll interval to get device twin info, default to 1 minute
		// +optional
		DeviceStatusPollInterval metav1.Duration `json:"deviceStatusPollInterval,omitempty" swaggertype:"primitive,string"`
	}

	AzureEventHubConfig struct {
		ConnectionStringSecretKeyRef LocalSecretKeyReference `json:"connectionStringSecretKeyRef,omitempty"`
		// +optional
		ConsumerGroup string `json:"consumerGroup,omitempty"`
	}
)

type AMQPSpec struct {
	// Version of AMQP
	Version string `json:"version,omitempty"`
	// Exchange in AMQP, if exists, MUST be a topic exchange
	Exchange string `json:"exchange,omitempty"`
	// TopicNamespace to share with arhat agent
	TopicNamespace string `json:"topicNamespace,omitempty"`
	// Broker address in the form of host:port
	Broker string `json:"broker,omitempty"`
	// +optional
	VHost string `json:"vhost,omitempty"`
	// +optional
	UserPassRef *corev1.LocalObjectReference `json:"userPassRef,omitempty"`
	// Secret for tls cert-key pair
	// +optional
	TLSSecretRef *corev1.LocalObjectReference `json:"tlsSecretRef,omitempty"`
}

type LocalSecretKeyReference struct {
	// Name of the referent
	Name string `json:"name,omitempty"`
	// Key of the data
	Key string `json:"key,omitempty"`
}

type ConnectivityBackoff struct {
	// +optional
	InitialDelay *metav1.Duration `json:"initialDelay,omitempty" swaggertype:"primitive,string"`
	// +optional
	MaxDelay *metav1.Duration `json:"maxDelay,omitempty" swaggertype:"primitive,string"`

	// +kubebuilder:validation:Type=number
	// +kubebuilder:validation:Format=double
	// +optional
	Factor string `json:"factor,omitempty"`
}

// +kubebuilder:validation:Enum:mqtt;grpc;amqp;azure-iot-hub;gcp-iot-core
type ConnectivityMethod string

const (
	MQTT        ConnectivityMethod = "mqtt"
	GRPC        ConnectivityMethod = "grpc"
	AMQP        ConnectivityMethod = "amqp"
	AzureIoTHub ConnectivityMethod = "azure-iot-hub"
	GCPIoTCore  ConnectivityMethod = "gcp-iot-core"
)

type ConnectivityTimers struct {
	// +optional
	UnarySessionTimeout *metav1.Duration `json:"unarySessionTimeout,omitempty" swaggertype:"primitive,string"`
}

type ConnectivitySpec struct {
	// +optional
	Timers ConnectivityTimers `json:"timers,omitempty"`
	// +optional
	Backoff ConnectivityBackoff `json:"backoff,omitempty"`

	// Method of how to establish communication channel between server and devices
	Method ConnectivityMethod `json:"method,omitempty"`

	// MQTT to tell aranya how to create mqtt client
	// +optional
	MQTT MQTTSpec `json:"mqtt,omitempty"`
	// GRPC to tell aranya how to create grpc server
	// +optional
	GRPC GRPCSpec `json:"grpc,omitempty"`
	// AMQP to tell aranya how to create rabbitMQ client
	// +optional
	AMQP AMQPSpec `json:"amqp,omitempty"`
	// AzureIoTHub for azure iot hub connection
	// +optional
	AzureIoTHub AzureIoTHubSpec `json:"azureIoTHub,omitempty"`
	// GCPIoTCore for google cloud pub/sub connection
	// +optional
	GCPIoTCore GCPIoTCoreSpec `json:"gcpIoTCore,omitempty"`
}
