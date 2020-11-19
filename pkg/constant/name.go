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
	ContainerNameAbbot = "abbot"
)

// virtual images
const (
	// VirtualImageNameHost is the special image name respected by aranya
	VirtualImageNameHost = "virtualimage.arhat.dev/host"

	// VirtualImageNamePeripheral is the special image name respected by aranya,
	// aranya will treat exec commands to the container with this image name
	// as device operation, if command is valid, will send operate command to
	// arhat, then arhat will perform the requested device operation
	VirtualImageNamePeripheral = "virtualimage.arhat.dev/peripheral"

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
	VirtualImageIDHost       = "virtualimage://host"
	VirtualImageIDPeripheral = "virtualimage://peripheral"
	VirtualImageIDHostExec   = "virtualimage://hostexec"
)

const (
	VirtualContainerNameHost     = "host"
	VirtualContainerIDHost       = "virtualcontainer://host"
	VirtualContainerIDPeripheral = "virtualcontainer://peripheral"
)

const (
	TLSCaCertKey = "ca.crt"
)

const (
	NetworkMeshDriverWireguard = "wireguard"
)

const (
	MeshConfigKeyWireguardPrivateKey = "wireguard.private-key"
)
