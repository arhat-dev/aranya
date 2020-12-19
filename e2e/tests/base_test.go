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

package tests

import (
	"context"
	"fmt"
	"os"
	"strings"

	"arhat.dev/pkg/kubehelper"
	"github.com/blang/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"arhat.dev/aranya/pkg/constant"
)

const EnvKeyKubeConfig = "ARANYA_E2E_KUBECONFIG"

const (
	_namePrefix = "e2e-"

	edgeDeviceNameAlice        = _namePrefix + "alice"
	edgeDeviceNameBob          = _namePrefix + "bob"
	aranyaNamespaceDefault     = "default"
	edgeDeviceNamespaceDefault = aranyaNamespaceDefault
	virtualPodNamespaceDefault = edgeDeviceNamespaceDefault

	edgeDeviceNameFoo      = _namePrefix + "foo"
	edgeDeviceNameBar      = _namePrefix + "bar"
	aranyaNamespaceFull    = "full"
	edgeDeviceNamespaceSys = "sys"
	virtualPodNamespaceSys = edgeDeviceNamespaceSys
)

// nolint:deadcode,varcheck,unused
var (
	v1_14 = semver.MustParse("1.14.0")
	v1_15 = semver.MustParse("1.15.0")
	v1_16 = semver.MustParse("1.16.0")
	v1_17 = semver.MustParse("1.17.0")
	v1_18 = semver.MustParse("1.18.0")
	v1_19 = semver.MustParse("1.19.0")
	v1_20 = semver.MustParse("1.20.0")
)

func createClient() (kubernetes.Interface, semver.Version) {
	kubeConfigFile := os.Getenv(EnvKeyKubeConfig)
	if len(kubeConfigFile) == 0 {
		panic(fmt.Sprintf("no e2e kubeconfig provided, please set env %q", EnvKeyKubeConfig))
	}

	config := &kubehelper.KubeClientConfig{
		Fake:           false,
		KubeconfigPath: kubeConfigFile,
		RateLimit: kubehelper.KubeClientRateLimitConfig{
			// disable client side rate limiting
			Enabled: false,
		},
	}

	kc, _, err := config.NewKubeClient(nil, true)
	if err != nil {
		panic(err)
	}

	ver, err := kc.Discovery().ServerVersion()
	if err != nil {
		panic(err)
	}

	return kc, semver.MustParse(strings.TrimLeft(ver.GitVersion, "v"))
}

func getAranyaLeaderPod(kubeClient kubernetes.Interface, namespace string) (*corev1.Pod, error) {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.FormatLabels(map[string]string{
			constant.LabelAranyaLeadership: constant.LabelAranyaLeadershipLeader,
		}),
	})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) != 1 {
		return nil, fmt.Errorf("unexpected %d leader pods", len(pods.Items))
	}

	return &pods.Items[0], nil
}
