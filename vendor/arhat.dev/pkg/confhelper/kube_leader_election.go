// +build !nocloud,!nokube

package confhelper

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"arhat.dev/pkg/envhelper"
)

type LeaderElectionLockConfig struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Type      string `json:"type" yaml:"type"`
}

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

type LeaderElectionLeaseConfig struct {
	Expiration       time.Duration `json:"expiration" yaml:"expiration"`
	RenewDeadline    time.Duration `json:"renewDeadline" yaml:"renewDeadline"`
	RenewInterval    time.Duration `json:"renewInterval" yaml:"renewInterval"`
	ExpiryToleration time.Duration `json:"expiryToleration" yaml:"expiryToleration"`
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

type LeaderElectionConfig struct {
	Identity string                    `json:"identity" yaml:"identity"`
	Lock     LeaderElectionLockConfig  `json:"lock" yaml:"lock"`
	Lease    LeaderElectionLeaseConfig `json:"lease" yaml:"lease"`
}

func FlagsForLeaderElection(name, prefix string, c *LeaderElectionConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("leader-election", pflag.ExitOnError)

	fs.StringVar(&c.Identity, prefix+"identity", envhelper.ThisPodName(), "set identity used for leader election")
	fs.AddFlagSet(FlagsForLeaderElectionLock(name, prefix+"lock.", &c.Lock))
	fs.AddFlagSet(FlagsForLeaderElectionLease(prefix+"lease.", &c.Lease))

	return fs
}

func (c *LeaderElectionConfig) CreateElector(
	name string,
	kubeClient kubernetes.Interface,
	eventRecorder record.EventRecorder,
	onElected func(context.Context),
	onEjected func(),
	onNewLeader func(identity string),
) (*leaderelection.LeaderElector, error) {
	lock, err := resourcelock.New(c.Lock.Type,
		c.Lock.Namespace,
		c.Lock.Name,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      c.Identity,
			EventRecorder: eventRecorder,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create resource lock: %w", err)
	}

	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Name:            name,
		WatchDog:        leaderelection.NewLeaderHealthzAdaptor(c.Lease.ExpiryToleration),
		Lock:            lock,
		LeaseDuration:   c.Lease.Expiration,
		RenewDeadline:   c.Lease.RenewDeadline,
		RetryPeriod:     c.Lease.RenewInterval,
		ReleaseOnCancel: true,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onElected,
			OnStoppedLeading: onEjected,
			OnNewLeader:      onNewLeader,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create leader elector: %w", err)
	}

	return elector, nil
}
