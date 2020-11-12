// +build !noflaghelper

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

import "github.com/spf13/pflag"

func FlagsForKubeClient(prefix string, c *KubeClientConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("kube.client", pflag.ExitOnError)

	fs.StringVar(&c.KubeconfigPath, prefix+"kubeconfig", "", "set path to kubeconfig file")

	fs.BoolVar(&c.RateLimit.Enabled, prefix+"rateLimit.enable", true, "enable rate limit for kubernetes api client")
	fs.Float32Var(&c.RateLimit.QPS, prefix+"rateLimit.qps", 5, "set requests per second limit")
	fs.IntVar(&c.RateLimit.Burst, prefix+"rateLimit.burst", 10, "set burst requests per second")

	return fs
}
