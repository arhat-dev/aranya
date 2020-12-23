// +build !noqueue_timeoutqueue

/*
Copyright 2019 The arhat.dev Authors.

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

package queue

import (
	"errors"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Errors for timeout queue
var (
	ErrNotStarted    = errors.New("not started")
	ErrStopped       = errors.New("stopped")
	ErrKeyNotAllowed = errors.New("key not allowed")
)

// NewTimeoutQueue returns an idle TimeoutQueue
func NewTimeoutQueue() *TimeoutQueue {
	t := time.NewTimer(0)
	if !t.Stop() {
		<-t.C
	}

	return &TimeoutQueue{
		stop:    nil,
		running: 0,

		mu:             new(sync.RWMutex),
		hasExpiredData: make(chan struct{}),

		blackList: make(map[interface{}]struct{}),
		index:     make(map[interface{}]int),
		data:      make([]*TimeoutData, 0, 16),

		timer:   t,
		resetCh: make(chan int64, 1),

		timeGot: 1,

		timeoutCh: make(chan *TimeoutData, 1),
	}
}

// TimeoutData is the data set used internally
type TimeoutData struct {
	Key  interface{}
	Data interface{}

	expireAt int64 // utc unix nano
}

// TimeoutQueue to arrange timeout events in a single queue, then you can
// access them in sequence with channel
type TimeoutQueue struct {
	stop    <-chan struct{}
	running uint32

	expireNotified uint32
	// protected by expireNotified
	hasExpiredData chan struct{}
	expiredData    []*TimeoutData

	mu *sync.RWMutex

	nextSort  int64 // utc unix nano
	blackList map[interface{}]struct{}
	index     map[interface{}]int
	data      []*TimeoutData

	timer   *time.Timer
	resetCh chan int64
	timeGot uint32

	timeoutCh chan *TimeoutData
}

// Start routine to generate timeout data
func (q *TimeoutQueue) Start(stop <-chan struct{}) {
	if !atomic.CompareAndSwapUint32(&q.running, 0, 1) {
		// already running and not stopped
		return
	}

	q.mu.Lock()
	q.stop = stop
	q.mu.Unlock()

	wg := new(sync.WaitGroup)

	// handle delivery of expired data
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				return
			case <-q.hasExpiredData:
				for _, d := range q.expiredData {
					data := d
					select {
					case <-stop:
						return
					case q.timeoutCh <- data:
					}
				}

				q.expiredData = nil
				q.hasExpiredData = make(chan struct{})
				atomic.StoreUint32(&q.expireNotified, 0)
			}
		}
	}()

	// handle timer reset
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-stop:
				if !q.timer.Stop() {
					<-q.timer.C
				}
				return
			case newWait := <-q.resetCh:
				if !q.timer.Stop() {
					// timer already fired
					if atomic.LoadUint32(&q.timeGot) == 0 {
						// no sort performed, sort now
						<-q.timer.C
						atomic.StoreUint32(&q.timeGot, 1)
						q.sort()
					}
				}

				// reset timer to new values
				q.timer.Reset(time.Duration(newWait))
				atomic.StoreUint32(&q.timeGot, 0)
			case <-q.timer.C:
				atomic.StoreUint32(&q.timeGot, 1)
				q.sort()
			}
		}
	}()

	go func() {
		wg.Wait()

		atomic.StoreUint32(&q.running, 0)
	}()
}

// Len is used internally for timeout data sort
func (q *TimeoutQueue) Len() int {
	return len(q.data)
}

// Less is used internally for timeout data sort
func (q *TimeoutQueue) Less(i, j int) bool {
	return q.data[i].expireAt < q.data[j].expireAt
}

// Swap is used internally for timeout data sort
func (q *TimeoutQueue) Swap(i, j int) {
	// swap index
	q.index[q.data[i].Key], q.index[q.data[j].Key] = j, i
	// swap data
	q.data[i], q.data[j] = q.data[j], q.data[i]
}

// sort timeout data and find expired
func (q *TimeoutQueue) sort() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.data) == 0 {
		return
	}

	// make sure the timeout data is sorted
	sort.Sort(q)

	now := time.Now().UTC().UnixNano()
	expiredCount := 0
	for i, c := range q.data {
		if c.expireAt-now < int64(time.Millisecond) {
			expiredCount = i + 1
			continue
		}

		// not expired, stop iteration and reset timer for the next
		q.nextSort = c.expireAt
		select {
		case <-q.stop:
			return
		case q.resetCh <- c.expireAt - now:
		}

		break
	}

	if expiredCount > 0 {
		// has expired data, signal to send
		select {
		case <-q.stop:
			return
		default:
			for atomic.LoadUint32(&q.expireNotified) == 1 {
				// wait for last expired data sent
				runtime.Gosched()
			}
		}

		q.expiredData = q.data[:expiredCount]
		q.data = q.data[expiredCount:]

		for _, d := range q.expiredData {
			delete(q.index, d.Key)
		}

		// rebuild index
		for i, d := range q.data {
			q.index[d.Key] = i
		}

		atomic.StoreUint32(&q.expireNotified, 1)
		close(q.hasExpiredData)
	}
}

// OfferWithTime to enqueue key-value pair with time, timeout at `time`, if you
// would like to call Remove to delete the timeout object, `key` must be unique
// in this queue
func (q *TimeoutQueue) OfferWithTime(key, val interface{}, at time.Time) error {
	if atomic.LoadUint32(&q.running) == 0 {
		return ErrNotStarted
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	select {
	case <-q.stop:
		return ErrStopped
	default:
		if _, ok := q.blackList[key]; ok {
			return ErrKeyNotAllowed
		}
	}
	expireAt := at.UTC().UnixNano()
	q.data = append(q.data, &TimeoutData{Key: key, Data: val, expireAt: expireAt})
	q.index[key] = len(q.data) - 1

	if len(q.data) == 1 || q.nextSort > expireAt {
		// the added item is the only one in the queue or
		// the next sort will be after this expiration
		q.nextSort = expireAt
		select {
		case <-q.stop:
			return ErrStopped
		case q.resetCh <- expireAt - time.Now().UTC().UnixNano():
		}
	}

	return nil
}

// OfferWithDelay to enqueue key-value pair, timeout after `wait`, if you
// would like to call Remove to delete the timeout object, `key` must be unique
// in this queue
func (q *TimeoutQueue) OfferWithDelay(key, val interface{}, wait time.Duration) error {
	return q.OfferWithTime(key, val, time.Now().Add(wait))
}

// TakeCh returns the channel from which you can get key-value pairs timed out
// one by one
func (q *TimeoutQueue) TakeCh() <-chan *TimeoutData {
	return q.timeoutCh
}

// Clear out all timeout key-value pairs
func (q *TimeoutQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.blackList = make(map[interface{}]struct{})
	q.index = make(map[interface{}]int)
	q.data = make([]*TimeoutData, 0, 16)
}

// Remove a timeout object from the queue according to the key
func (q *TimeoutQueue) Remove(key interface{}) (interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if i, ok := q.index[key]; ok {
		toRemove := q.data[i].Data

		delete(q.index, key)
		q.data = append(q.data[:i], q.data[i+1:]...)

		// rebuild index
		for i, d := range q.data {
			q.index[d.Key] = i
		}
		return toRemove, true
	}
	return nil, false
}

// Allow allow tasks with key, future tasks with the key can be offered
func (q *TimeoutQueue) Allow(key interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.blackList, key)
}

// Forbid forbid tasks with key, future tasks with the key cannot be offered
func (q *TimeoutQueue) Forbid(key interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.blackList[key] = struct{}{}
}

// Find timeout key-value pair according to the key
func (q *TimeoutQueue) Find(key interface{}) (interface{}, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	idx, ok := q.index[key]
	if ok {
		return q.data[idx], true
	}

	return nil, false
}

// Remains shows key-value pairs not timed out
func (q *TimeoutQueue) Remains() []TimeoutData {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.data) == 0 {
		return nil
	}

	result := make([]TimeoutData, len(q.data))
	for i, d := range q.data {
		result[i] = *d
	}
	return result
}
