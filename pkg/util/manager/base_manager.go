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

package manager

import (
	"context"
	"sync"

	"arhat.dev/pkg/log"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func NewBaseManager(parentCtx context.Context, name string, connectivityManager connectivity.Manager) *BaseManager {
	ctx, exit := context.WithCancel(parentCtx)
	return &BaseManager{
		ctx:  ctx,
		exit: exit,

		Log:                 log.Log.WithName(name),
		ConnectivityManager: connectivityManager,

		once: new(sync.Once),
	}
}

type BaseManager struct {
	ctx  context.Context
	exit context.CancelFunc

	Log                 log.Interface
	ConnectivityManager connectivity.Manager

	once *sync.Once
}

func (m *BaseManager) Context() context.Context {
	return m.ctx
}

func (m *BaseManager) OnStart(doStart func() error) error {
	var err error
	m.once.Do(func() {
		err = doStart()
	})
	return err
}

func (m *BaseManager) OnClose(doClose func()) {
	m.exit()

	if doClose != nil {
		doClose()
	}
}

func (m *BaseManager) Closing() bool {
	select {
	case <-m.ctx.Done():
		return true
	default:
		return false
	}
}
