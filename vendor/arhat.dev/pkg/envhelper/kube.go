// +build !noenvhelper_kube

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

package envhelper

import (
	"os"
)

const (
	EnvKeyPodName      = "POD_NAME"
	EnvKeyPodNamespace = "POD_NAMESPACE"
)

var (
	podName string
	podNS   string
)

func init() {
	var ok bool
	podName = os.Getenv(EnvKeyPodName)

	podNS, ok = os.LookupEnv(EnvKeyPodNamespace)
	if !ok {
		podNS = "default"
	}
}

func ThisPodName() string {
	return podName
}

func ThisPodNS() string {
	return podNS
}
