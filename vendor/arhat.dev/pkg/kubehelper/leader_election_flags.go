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

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"arhat.dev/pkg/envhelper"
)

func FlagsForLeaderElectionLock(name, prefix string, c *LeaderElectionLockConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("leader-election-lock", pflag.ExitOnError)

	// lock
	fs.StringVar(&c.Type, prefix+"lock.type", "leases",
		"set resource lock type for leader election, possible values are "+
			"[configmaps, endpoints, leases, configmapsleases, endpointsleases]")
	fs.StringVar(&c.Name, prefix+"lock.name", fmt.Sprintf("%s-leader-election", name), "set resource lock name")
	fs.StringVar(&c.Namespace, prefix+"lock.namespace", envhelper.ThisPodNS(), "set resource lock namespace")

	return fs
}

func FlagsForLeaderElectionLease(prefix string, c *LeaderElectionLeaseConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("leader-election-lease", pflag.ExitOnError)

	fs.DurationVar(&c.Expiration, prefix+"expiration", 15*time.Second,
		"set duration a lease is valid for, all non-leader need to wait at least this long to attempt to become leader")
	fs.DurationVar(&c.RenewDeadline, prefix+"renewDeadline", 10*time.Second,
		"set max time duration for a successful renew, or will lose leader election, MUST < expiration")
	fs.DurationVar(&c.RenewInterval, prefix+"renewInterval", 2*time.Second,
		"set intervals between renew operations (update lock resource)")
	fs.DurationVar(&c.ExpiryToleration, prefix+"expiryToleration", 10*time.Second,
		"set how long we will wait until try to acquire lease after lease has expired")

	return fs
}

func FlagsForLeaderElection(name, prefix string, c *LeaderElectionConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("leader-election", pflag.ExitOnError)

	fs.StringVar(&c.Identity, prefix+"identity", envhelper.ThisPodName(), "set identity used for leader election")
	fs.AddFlagSet(FlagsForLeaderElectionLock(name, prefix+"lock.", &c.Lock))
	fs.AddFlagSet(FlagsForLeaderElectionLease(prefix+"lease.", &c.Lease))

	return fs
}
