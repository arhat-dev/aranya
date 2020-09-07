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

package connectivity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"arhat.dev/aranya-proto/gopb"
)

func TestSessionManager_Add(t *testing.T) {
	mgr := NewSessionManager()
	sidA, chA := mgr.Add(gopb.NewPodListCmd("", "", true), time.Second, false)
	sidB, chB := mgr.Add(gopb.NewPodResizeCmd(sidA, 0, 0), time.Second, false)
	sidC, chC := mgr.Add(gopb.NewPodListCmd("", "", true), time.Second, false)

	assert.NotNil(t, sidA)
	assert.Equal(t, sidA, sidB)
	assert.Equal(t, chA, chB)
	assert.NotEqual(t, chA, chC)
	assert.NotEqual(t, sidA, sidC)

	start := time.Now()
	_, more := <-chC
	assert.True(t, more)
	if time.Since(start) < time.Second/2 {
		assert.Fail(t, "timout not match")
	}
	_, more = <-chC
	assert.False(t, more)
}

func TestSessionManager_Del(t *testing.T) {
	mgr := NewSessionManager()
	sid, ch := mgr.Add(gopb.NewPodListCmd("", "", true), 0, false)
	mgr.Delete(sid)
	ok := mgr.Dispatch(gopb.NewDataMsg(1, true, gopb.DATA_OTHER, 0, nil))
	assert.Equal(t, false, ok)

	_, more := <-ch
	assert.Equal(t, false, more)
}

func TestSessionManager_Get(t *testing.T) {
	// mgr := newSessionManager()
	// ctx := context.Background()
	// sidA, _ := mgr.Add(ctx, connectivity.NewPodListCmd("", "", true))
	// timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
	// defer cancel()
	// sidB, _ := mgr.Add(timeoutCtx, connectivity.NewPodListCmd("", "", true))

	// ok := mgr.dispatch(sidA)
	// assert.Equal(t, true, ok)
	//
	// _, ok = mgr.dispatch(sidB)
	// assert.Equal(t, true, ok)
	//
	// time.Sleep(time.Second)
	//
	// _, ok = mgr.dispatch(sidA)
	// assert.Equal(t, true, ok)
	//
	// _, ok = mgr.dispatch(sidB)
	// assert.Equal(t, false, ok)
}
