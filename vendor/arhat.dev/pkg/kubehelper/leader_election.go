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
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

type LeaderElectionLockConfig struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Type      string `json:"type" yaml:"type"`
}

type LeaderElectionLeaseConfig struct {
	Expiration       time.Duration `json:"expiration" yaml:"expiration"`
	RenewDeadline    time.Duration `json:"renewDeadline" yaml:"renewDeadline"`
	RenewInterval    time.Duration `json:"renewInterval" yaml:"renewInterval"`
	ExpiryToleration time.Duration `json:"expiryToleration" yaml:"expiryToleration"`
}

type LeaderElectionConfig struct {
	Identity string                    `json:"identity" yaml:"identity"`
	Lock     LeaderElectionLockConfig  `json:"lock" yaml:"lock"`
	Lease    LeaderElectionLeaseConfig `json:"lease" yaml:"lease"`
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
