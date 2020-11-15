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
	"sync"
)

// NewSeqQueue returns a empty SeqQueue
func NewSeqQueue() *SeqQueue {
	return &SeqQueue{
		next: 0,
		max:  math.MaxUint64,
		data: make([]*seqData, 0, 16),
		mu:   new(sync.Mutex),
	}
}

type seqData struct {
	seq  uint64
	data interface{}
}

// SeqQueue is the sequence queue for unordered data
type SeqQueue struct {
	next uint64
	max  uint64
	data []*seqData
	mu   *sync.Mutex
}

// Offer an unordered data with its sequence
func (q *SeqQueue) Offer(seq uint64, data interface{}) (out []interface{}, completed bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	switch {
	case q.next > q.max:
		// complete, discard
		return nil, true
	case seq > q.max, seq < q.next:
		// exceeded or duplicated, discard
		return nil, false
	case seq == q.next:
		// is expected next chunk, pop it and its following chunks
		q.next++
		out = []interface{}{data}
		for _, d := range q.data {
			if d.seq != q.next || d.seq > q.max {
				break
			}
			out = append(out, d.data)
			q.next++
		}
		q.data = q.data[len(out)-1:]

		return out, q.next > q.max
	}

	insertAt := 0
	for i, d := range q.data {
		if d.seq > seq {
			insertAt = i
			break
		}

		// duplicated
		if d.seq == seq {
			return nil, false
		}

		insertAt = i + 1
	}

	q.data = append(q.data[:insertAt], append([]*seqData{{seq: seq, data: data}}, q.data[insertAt:]...)...)

	return nil, false
}

// SetMaxSeq sets when should this queue stop adding data
func (q *SeqQueue) SetMaxSeq(maxSeq uint64) (completed bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.next > maxSeq {
		// existing seq data already exceeds maxSeq
		q.max = q.next
		return true
	}

	q.max = maxSeq
	return false
}

// Reset the SeqQueue for new sequential data
func (q *SeqQueue) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.next = 0
	q.max = math.MaxUint64
	q.data = make([]*seqData, 0, 16)
}
