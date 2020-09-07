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

const (
	ContainerNamePause = "_pause"
	ContainerNameAbbot = "abbot"
)

// virtual images
const (
	// VirtualImageNameHost is the special image name respected by aranya
	VirtualImageNameHost = "virtualimage.arhat.dev/host"

	// VirtualImageNameDevice is the special image name respected by aranya,
	// aranya will treat exec commands to the container with this image name
	// as device operation, if command is valid, will send operate command to
	// arhat, then arhat will perform the requested device operation
	VirtualImageNameDevice = "virtualimage.arhat.dev/device"

	// VirtualImageNameHostExec is the special image name respected by aranya,
	// aranya will treat exec commands to the container with this image name
	// as host exec, and will send pod exec command to arhat, then arhat will
	// execute the commands specified
	//
	// specify the script interpreter in `command` and commands to execute in
	// args, if no `command` is specified, will default to `sh`
	// (usually /bin/sh in your host)
	VirtualImageNameHostExec = "virtualimage.arhat.dev/hostexec"
)

const (
	VirtualImageIDHost     = "virtualimage://host"
	VirtualImageIDDevice   = "virtualimage://device"
	VirtualImageIDHostExec = "virtualimage://hostexec"
)

const (
	VirtualContainerNameHost = "host"
	VirtualContainerIDHost   = "virtualcontainer://host"
	VirtualContainerIDDevice = "virtualcontainer://device"
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

func PrevLogFile(name string) string {
	return name + ".old"
}

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