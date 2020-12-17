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

	"arhat.dev/pkg/kubehelper"
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

func createClient() kubernetes.Interface {
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

	return kc
}

func getAranyaLeaderPod(kubeClient kubernetes.Interface, namespace string) (*corev1.Pod, error) {
	pods, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labels.FormatLabels(map[string]string{
			constant.LabelControllerLeadership: constant.LabelControllerLeadershipLeader,
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
