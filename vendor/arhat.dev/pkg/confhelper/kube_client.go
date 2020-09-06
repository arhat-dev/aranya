// +build !nocloud,!nokube

package confhelper

import (
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
)

func FlagsForKubeClient(prefix string, c *KubeClientConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("kube.client", pflag.ExitOnError)

	fs.StringVar(&c.KubeconfigPath, prefix+"kubeconfig", "", "set path to kubeconfig file")

	fs.BoolVar(&c.RateLimit.Enabled, prefix+"rateLimit.enable", true, "enable rate limit for kubernetes api client")
	fs.Float32Var(&c.RateLimit.QPS, prefix+"rateLimit.qps", 5, "set requests per second limit")
	fs.IntVar(&c.RateLimit.Burst, prefix+"rateLimit.burst", 10, "set burst requests per second")

	return fs
}

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
