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

// Label role values identifies resource object's functionality in `aranya`
const (
	LabelRole = "arhat.dev/role"

	// Use upper case for role values by deliberation
	LabelRoleValueConnectivity    = "Connectivity"
	LabelRoleValueCertificate     = "NodeCertificate"
	LabelRoleValueNode            = "Node"
	LabelRoleValueAbbot           = "Abbot"
	LabelRoleValueNodeClusterRole = "NodeClusterRole"
	LabelRoleValuePodRole         = "PodRole"
	LabelRoleValueNodeLease       = "NodeLease"
)

const (
	LabelNamespace = "arhat.dev/namespace"
	LabelName      = "arhat.dev/name"
	LabelArch      = "arhat.dev/arch"
)

const (
	LabelControllerLeadership = "controller.arhat.dev/leadership"

	LabelControllerLeadershipLeader = "leader"
)

// Well-known kubernetes role label for node objects
const (
	LabelKubeRole = "kubernetes.io/role"
)
