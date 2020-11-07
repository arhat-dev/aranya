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

package iohelper

import (
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// TimeoutReader is a reader with read timeout
//
// It is designed for those want to read some data from a stream, and the size
// of the data is unknown, but still want to pipe data to some destination.
type TimeoutReader struct {
	buf       []byte
	chunkSize int
	r         io.Reader

	// signal to notify user can do ReadUntilTimeout operation
	hasData chan struct{}
	// signal to notify the size of buffered data has reached chunkSize
	dataFull chan struct{}

	started uint32
	err     *atomic.Value
	mu      *sync.RWMutex
}

// NewTimeoutReader creates a new idle timeout reader
func NewTimeoutReader(r io.Reader, chunkSize int) *TimeoutReader {
	return &TimeoutReader{
		chunkSize: chunkSize,
		r:         r,

		hasData:  make(chan struct{}),
		dataFull: make(chan struct{}),

		started: 0,
		err:     new(atomic.Value),
		mu:      new(sync.RWMutex),
	}
}

// Error returns the error happened during reading in background
func (t *TimeoutReader) Error() error {
	if e := t.err.Load(); e != nil {
		return e.(error)
	}

	return nil
}

// StartBackgroundReading until EOF or error returned, should be called in a goroutine
// other than the one you are reading
func (t *TimeoutReader) StartBackgroundReading() {
	if !atomic.CompareAndSwapUint32(&t.started, 0, 1) {
		return
	}

	var (
		n   int
		err error
	)

	oneByte := make([]byte, 1)
	for {
		// read one byte a time to avoid being blocked
		n, err = t.r.Read(oneByte)
		switch n {
		case 0:
			// no bytes read or error happened
			if err == nil {
				err = io.EOF
			}

			t.mu.RLock()

			select {
			case <-t.hasData:
			default:
				close(t.hasData)
			}

			t.mu.RUnlock()
		case 1:
			t.mu.Lock()

			select {
			case <-t.hasData:
			default:
				close(t.hasData)
			}

			// rely on the default slice grow
			t.buf = append(t.buf, oneByte[0])
			if len(t.buf) >= t.chunkSize {
				select {
				case <-t.dataFull:
				default:
					close(t.dataFull)
				}
			}

			t.mu.Unlock()
		}

		if err != nil {
			t.err.Store(err)
			break
		}
	}

}

func (t *TimeoutReader) hasDataInBuf() bool {
	// take a snapshot of the channel pointer in case it got closed and recreated
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.buf) != 0
}

// WaitUntilHasData is a helper function used to check if there is data available,
// to reduce actual call of ReadUntilTimeout when the timeout is a short duration
func (t *TimeoutReader) WaitUntilHasData(stopSig <-chan struct{}) bool {
	if t.Error() != nil {
		// error happened, no more data will be read from background
		return t.hasDataInBuf()
	}

	// take a snapshot of the channel pointer in case it got closed and recreated
	t.mu.RLock()
	hasData := t.hasData
	t.mu.RUnlock()

	select {
	case <-stopSig:
		return false
	case <-hasData:
		t.mu.RLock()
		defer t.mu.RUnlock()
		return len(t.buf) != 0
	}
}

// ReadUntilTimeout perform a read operation on buffered data, return a chunk
// of data if
//
// the size of the buffered data has reached or maxed out the chunkSize,
// then the size of returned data chunk will be chunkSize
//
// or
//
// the stop signaled, but buffer is not full, will return all buffered data
func (t *TimeoutReader) ReadUntilTimeout(stop <-chan time.Time) (data []byte, isTimeout bool) {
	for {
		// take a snapshot for the size of buffered data
		t.mu.RLock()
		size := len(t.buf)
		t.mu.RUnlock()

		n := size
		if n > t.chunkSize {
			n = t.chunkSize
		}

		switch {
		case n == t.chunkSize, isTimeout:
			if n == 0 {
				return
			}

			if size < n {
				n = size
			}

			t.mu.Lock()

			data = t.buf[:n]
			t.buf = t.buf[n:]

			size = len(t.buf)
			if size < t.chunkSize {
				t.dataFull = make(chan struct{})
			}

			if size == 0 && t.Error() == nil {
				t.hasData = make(chan struct{})
			}

			t.mu.Unlock()
			return
		}

		select {
		case <-stop:
			isTimeout = true
		case <-t.dataFull:
			// wait until data full
		}
	}
}
