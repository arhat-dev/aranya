// +build !noqueue_seqqueue

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
	"math"
	"runtime"
	"sync"
	"sync/atomic"
)

type SeqDataHandleFunc func(seq uint64, d interface{})

// NewSeqQueue returns a empty SeqQueue
func NewSeqQueue(handleData SeqDataHandleFunc) *SeqQueue {
	return &SeqQueue{
		next: 0,
		max:  math.MaxUint64,

		handleData:   handleData,
		_snapshoting: 0,

		m: new(sync.Map),
	}
}

// SeqQueue is the sequence queue for unordered data
type SeqQueue struct {
	next uint64
	max  uint64

	handleData   SeqDataHandleFunc
	_snapshoting uint32

	m *sync.Map

	mu sync.Mutex
}

func (q *SeqQueue) handleExpectedNext(seq, max uint64, data interface{}) bool {
	q.mu.Lock()
	if !atomic.CompareAndSwapUint64(&q.next, seq, seq+1) {
		// lost competition, discard
		q.mu.Unlock()

		return seq > max
	}

	q.handleData(seq, data)

	for seq++; ; seq++ {
		v, ok := q.m.Load(seq)
		if !ok {
			break
		}

		if !atomic.CompareAndSwapUint64(&q.next, seq, seq+1) {
			break
		}

		q.m.Delete(seq)

		q.handleData(seq, v)
	}
	q.mu.Unlock()

	return seq > max
}

// Offer an unordered data with its sequence
func (q *SeqQueue) Offer(seq uint64, data interface{}) (complete bool) {
	var (
		next, max uint64
	)

	// take a snapshot of existing values to ensure all concurrent goroutines
	// have the same values
	q.doSnapshot(func() {
		next = atomic.LoadUint64(&q.next)
		max = atomic.LoadUint64(&q.max)
	})

	switch {
	case next > max:
		// already complete, discard
		return true
	case seq < next:
		// already set, discard
		return false
	case seq > max:
		// exceeded or duplicated, discard
		return false
	case seq == next:
		// is expected next chunk, pop it and its following chunks
		return q.handleExpectedNext(next, max, data)
	default:
		// cache unordered data chunk
		q.m.Store(seq, data)
	}

	return false
}

// SetMaxSeq set when should this queue stop enqueuing data
func (q *SeqQueue) SetMaxSeq(maxSeq uint64) (complete bool) {
	if next := atomic.LoadUint64(&q.next); next > maxSeq {
		// existing seq data already exceeds maxSeq
		atomic.StoreUint64(&q.max, next)
		return true
	}

	atomic.StoreUint64(&q.max, maxSeq)
	return false
}

// Reset the SeqQueue for new sequential data
func (q *SeqQueue) Reset() {
	q.next = 0
	q.max = math.MaxUint64
	q.m = new(sync.Map)
}

func (q *SeqQueue) doSnapshot(f func()) {
	for !atomic.CompareAndSwapUint32(&q._snapshoting, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&q._snapshoting, 0)
}
