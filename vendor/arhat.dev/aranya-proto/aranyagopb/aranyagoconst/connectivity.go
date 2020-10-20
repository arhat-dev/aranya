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

package aranyagoconst

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

const (
	VariantAzureIoTHub = "azure-iot-hub"
	VariantGCPIoTCore  = "gcp-iot-core"
	VariantAWSIoTCore  = "aws-iot-core"
)

const (
	ConnectivityMethodGRPC = "grpc"
	ConnectivityMethodMQTT = "mqtt"
	ConnectivityMethodCoAP = "coap"
)

func MQTTTopics(ns string) (cmdTopic, msgTopic, statusTopic string) {
	join := func(s string) string {
		return ns + "/" + s
	}
	return join("cmd"), join("msg"), join("status")
}

func CoAPTopics(ns string) (cmdPath, msgPath, statusPath string) {
	return MQTTTopics(ns)
}

func AMQPTopics(ns string) (cmdTopic, msgTopic, statusTopic string) {
	join := func(s string) string {
		return ns + "." + s
	}
	return join("cmd"), join("msg"), join("status")
}
