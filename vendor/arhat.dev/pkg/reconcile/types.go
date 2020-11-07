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

package reconcile

import (
	"errors"
	"time"

	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/log"
	"arhat.dev/pkg/queue"
)

var (
	resultCacheNotFound = &Result{Err: errors.New("cache not found")}
)

type Interface interface {
	Start() error
	ReconcileUntil(stop <-chan struct{})
	Schedule(job queue.Job, delay time.Duration) error
	CancelSchedule(job queue.Job) bool
}

type Options struct {
	Logger          log.Interface
	BackoffStrategy *backoff.Strategy
	Workers         int
	RequireCache    bool
	Handlers        HandleFuncs
	OnBackoffStart  BackoffStartCallback
	OnBackoffReset  BackoffResetCallback
}

func (o Options) ResolveNil() *Options {
	result := &Options{
		Logger:          o.Logger,
		BackoffStrategy: o.BackoffStrategy,
		Workers:         o.Workers,
		RequireCache:    o.RequireCache,
		Handlers:        o.Handlers,
		OnBackoffStart:  o.OnBackoffStart,
		OnBackoffReset:  o.OnBackoffReset,
	}

	if result.Logger == nil {
		result.Logger = log.NoOpLogger
	}

	if result.BackoffStrategy == nil {
		result.BackoffStrategy = backoff.NewStrategy(300*time.Millisecond, 10*time.Second, 1.5, 3)
	}

	if result.OnBackoffStart == nil {
		result.OnBackoffStart = backoffStartCallbackNoOp
	}

	if result.OnBackoffReset == nil {
		result.OnBackoffReset = backoffResetCallbackNoOp
	}

	return result
}

type Result struct {
	NextAction    queue.JobAction
	ScheduleAfter time.Duration
	Err           error
}

type (
	SingleObjectHandleFunc  func(obj interface{}) *Result
	CompareObjectHandleFunc func(old, new interface{}) *Result
)

func singleObjectAlwaysSuccess(_ interface{}) *Result {
	return nil
}

func compareObjectAlwaysSuccess(_, _ interface{}) *Result {
	return nil
}

type (
	BackoffStartCallback func(key interface{}, err error)
	BackoffResetCallback func(key interface{})
)

func backoffStartCallbackNoOp(key interface{}, err error) {}
func backoffResetCallbackNoOp(key interface{})            {}

type HandleFuncs struct {
	OnAdded    SingleObjectHandleFunc
	OnUpdated  CompareObjectHandleFunc
	OnDeleting SingleObjectHandleFunc
	OnDeleted  SingleObjectHandleFunc
}

func (h *HandleFuncs) ResolveNil() *HandleFuncs {
	result := &HandleFuncs{
		OnAdded:    h.OnAdded,
		OnUpdated:  h.OnUpdated,
		OnDeleting: h.OnDeleting,
		OnDeleted:  h.OnDeleted,
	}

	if result.OnAdded == nil {
		result.OnAdded = singleObjectAlwaysSuccess
	}

	if result.OnUpdated == nil {
		result.OnUpdated = compareObjectAlwaysSuccess
	}

	if result.OnDeleting == nil {
		result.OnDeleting = singleObjectAlwaysSuccess
	}

	if result.OnDeleted == nil {
		result.OnDeleted = singleObjectAlwaysSuccess
	}

	return result
}
