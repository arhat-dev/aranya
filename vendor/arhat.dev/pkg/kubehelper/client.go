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

package kubehelper

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
)

type KubeClientRateLimitConfig struct {
	Enabled bool    `json:"enabled" yaml:"enabled"`
	QPS     float32 `json:"qps" yaml:"qps"`
	Burst   int     `json:"burst" yaml:"burst"`
}

type KubeClientConfig struct {
	// Fake to create a fake client instead of creating real kubernetes client
	Fake           bool                      `json:"fake" yaml:"fake"`
	KubeconfigPath string                    `json:"kubeconfig" yaml:"kubeconfig"`
	RateLimit      KubeClientRateLimitConfig `json:"rateLimit" yaml:"rateLimit"`
}

// NewKubeClient creates a kubernetes client with/without existing kubeconfig
// if nil kubeconfig was provided, then will retrieve kubeconfig from configured
// path and will fallback to in cluster kubeconfig
// you can choose whether rate limit config is applied, if not, will use default
// rate limit config
func (c *KubeClientConfig) NewKubeClient(
	kubeconfig *rest.Config,
	applyRateLimitConfig bool,
) (client kubernetes.Interface, _ *rest.Config, err error) {
	if c.Fake {
		return fake.NewSimpleClientset(), new(rest.Config), nil
	}

	if kubeconfig != nil {
		kubeconfig = rest.CopyConfig(kubeconfig)
	} else {
		if c.KubeconfigPath != "" {
			kubeconfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: c.KubeconfigPath},
				&clientcmd.ConfigOverrides{}).ClientConfig()

			if err != nil {
				return nil, nil, fmt.Errorf("failed to load kubeconfig from file %q: %w", c.KubeconfigPath, err)
			}
		}

		// fallback to in cluster config
		if kubeconfig == nil {
			kubeconfig, err = rest.InClusterConfig()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load in cluster kubeconfig: %w", err)
			}
		}
	}

	if applyRateLimitConfig {
		if c.RateLimit.Enabled {
			kubeconfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(c.RateLimit.QPS, c.RateLimit.Burst)
		} else {
			kubeconfig.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		}
	}

	client, err = kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kube client from kubeconfig: %w", err)
	}

	return client, kubeconfig, nil
}
