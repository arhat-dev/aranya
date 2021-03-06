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
	"os"

	"arhat.dev/pkg/envhelper"
)

const (
	EnvKeySysNamespace    = "SYS_NAMESPACE"
	EnvKeyTenantNamespace = "TENANT_NAMESPACE"
)

var (
	sysNS    string
	tenantNS string
)

func init() {
	var ok bool

	sysNS, ok = os.LookupEnv(EnvKeySysNamespace)
	if !ok {
		sysNS = envhelper.ThisPodNS()
	}

	tenantNS, ok = os.LookupEnv(EnvKeyTenantNamespace)
	if !ok {
		tenantNS = sysNS
	}
}

func SysNS() string {
	return sysNS
}

func TenantNS() string {
	return tenantNS
}
