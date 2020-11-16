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

// NewTimeoutReader creates a new idle timeout reader
func NewTimeoutReader(r io.Reader) *TimeoutReader {
	var (
		setReadDeadline   func(t time.Time) error
		cannotSetDeadline = make(chan struct{})
	)

	// check if can set read deadline
	rs, ok := r.(interface {
		SetReadDeadline(t time.Time) error
	})

	// clear read deadline in advance to check set deadline capability
	if ok && rs.SetReadDeadline(time.Time{}) == nil {
		setReadDeadline = rs.SetReadDeadline
	} else {
		// mark set read deadline not working
		close(cannotSetDeadline)
	}

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	return &TimeoutReader{
		timer: timer,

		setReadDeadline:   setReadDeadline,
		cannotSetDeadline: cannotSetDeadline,

		r: r,

		readOneByteReqCh: make(chan struct{}),

		firstByteReady: make(chan struct{}),
		firstByte:      make(chan byte),

		hasData: make(chan struct{}),

		maxRead:  0,
		dataFull: make(chan struct{}),

		oneByte: make([]byte, 1),

		err: new(atomic.Value),
		mu:  new(sync.RWMutex),
	}
}

// TimeoutReader is a reader with read timeout
//
// It is designed for those want to read some data from a stream, and the size
// of the data is unknown, but still want to pipe data to some destination at
// certain interval
//
// example use case: data streaming for shell interaction over MQTT
//
// 	when user input/output is slow, shall we send one character a time for real-time
// 	interaction? what if the user executed `cat some-large-file`? what if user
// 	was sending a large chunk of data over stdin?
//
// 	for raw tcp connection that's fine if you have configured tcp buffering correctly,
// 	but for packet oriented connections (in this case MQTT), send one packet per byte
// 	will signaficantly increase protocol overhead.
//
// 	with TimeoutReader we can read data generated in some interval (say 20ms), no
//  real-time experience lost while still keep protocol overhead at a reasonable level
type TimeoutReader struct {
	buf []byte

	timer *time.Timer

	setReadDeadline   func(t time.Time) error
	cannotSetDeadline chan struct{}

	r io.Reader

	// signal to notify user can do ReadUntilTimeout operation
	hasData chan struct{}

	maxRead  int
	dataFull chan struct{}

	readOneByteReqCh chan struct{}
	firstByteReady   chan struct{}
	firstByte        chan byte

	oneByte []byte

	err *atomic.Value
	mu  *sync.RWMutex
}

// Error returns the error happened during reading in background
func (t *TimeoutReader) Error() error {
	if e := t.err.Load(); e != nil {
		if err, ok := e.(error); ok && err != nil {
			return err
		}
	}

	return nil
}

func (t *TimeoutReader) requestMaxRead(size int) <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.maxRead = size

	select {
	case <-t.dataFull:
		t.dataFull = make(chan struct{})
	default:
		// reuse sig channel
	}

	return t.dataFull
}

// FallbackReading is a helper routine for data reading from readers has no SetReadDeadline
// or SetReadDeadline failed when being called
//
// this function will block until EOF or error, so must be called in a goroutine other than
// the one you are reading data
//
// NOTE: this function MUST be called no more than once
func (t *TimeoutReader) FallbackReading(stopSig <-chan struct{}) {
	var (
		n   int
		err error
	)

loop:
	for {
		select {
		case <-stopSig:
			return
		case <-t.cannotSetDeadline:
			break loop
		case <-t.readOneByteReqCh:
			err = t.setReadDeadline(time.Time{})
			if err != nil {
				// set read deadline not working
				close(t.cannotSetDeadline)
				break loop
			}

			// read one byte in blocking mode
			n, err = t.r.Read(t.oneByte)
			if n == 1 {
				// notify wait for data
				t.firstByteReady <- struct{}{}
				t.firstByte <- t.oneByte[0]
			} else {
				// no data read, failed, try to fallback
				close(t.cannotSetDeadline)
				if err != nil {
					close(t.hasData)
					t.err.Store(err)
					return
				}

				// in case no error happened, check reader state twice
				break loop
			}
		}
	}

	close(t.firstByteReady)
	close(t.firstByte)

	// not able to set read deadline any more, fallback to one byte read mode
	// case 1: SetReadDeadline not supported
	// case 2: SetReadDeadline function call failed

	// check if reader is ok
	_, err = t.r.Read(t.oneByte[:0])
	if err != nil {
		// reader got some error, usually closed
		close(t.hasData)
		t.err.Store(err)
		return
	}

	for {
		// read one byte a time to avoid being blocked
		n, err = t.r.Read(t.oneByte)
		switch n {
		case 0:
			// no bytes read or error happened, exit now
			if err == nil {
				err = io.EOF
			}

			t.mu.Lock()

			select {
			case <-t.hasData:
			default:
				close(t.hasData)
			}

			t.mu.Unlock()
		case 1:
			t.mu.Lock()

			// rely on the default slice grow
			t.buf = append(t.buf, t.oneByte[0])

			select {
			case <-t.hasData:
			default:
				close(t.hasData)
			}

			// signal data full
			if len(t.buf) >= t.maxRead {
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
			return
		}

		select {
		case <-stopSig:
			// no more reading from reader
			return
		default:
		}
	}
}

func (t *TimeoutReader) hasDataInBuf() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.buf) != 0
}

// WaitForData is a helper function used to check if there is data available in reader
// so we can reduce actual call of Read when the timeout is a short duration
//
// when return value is true, you can call Read to read data, otherwise, false means
// the stopSig has signaled, and we have no idea whether you should continue Read
// from the reader
func (t *TimeoutReader) WaitForData(stopSig <-chan struct{}) bool {
	select {
	case <-stopSig:
		return false
	case <-t.cannotSetDeadline:
		// not using set read deadline
		break
	case t.readOneByteReqCh <- struct{}{}:
		// using set read deadline, wait for first byte
		select {
		case <-stopSig:
			return false
		case <-t.cannotSetDeadline:
			// in one byte read mode, wait until has data
			break
		case _, ok := <-t.firstByteReady:
			// can set read timeout and first byte read
			// NOTE: this channel branch must be after t.cannotSetDeadline
			if ok {
				return true
			}
			break
		}
	}

	// in one byte read mode

	if t.Error() != nil {
		// error happened, no more data will be read from background
		return t.hasDataInBuf()
	}

	// take a snapshot of the channel pointer in case it got recreated
	t.mu.RLock()
	hasData := t.hasData
	t.mu.RUnlock()

	select {
	case <-stopSig:
		return false
	case <-hasData:
		// if no data buffered but signal released
		// usually error happened when reading in background
		if t.Error() != nil {
			return t.hasDataInBuf()
		}

		return true
	}
}

// Read performs a read operation with timeout option, function will return when
// maxWait exceeded or p is full
// if the function returned because of timeout, the returned error is ErrDeadlineExceeded
// for go1.15 and on, it's the same as os.ErrDeadlineExceeded
func (t *TimeoutReader) Read(maxWait time.Duration, p []byte) (data []byte, shouldCopy bool, err error) {
	var (
		n           int
		firstByte   byte
		more        bool
		maxReadSize = len(p)
	)

	if maxReadSize == 0 {
		return p, false, t.Error()
	}

	select {
	case <-t.cannotSetDeadline:
		// not able to set read deadline any more, read from buffered data
		break
	default:
		// can set read deadline, wait for first byte
		firstByte, more = <-t.firstByte
		if !more {
			return p[:0], false, t.Error()
		}

		// try to set read deadline first
		if t.setReadDeadline(time.Now().Add(maxWait)) != nil {
			// signal not able to set read deadline
			close(t.cannotSetDeadline)

			// clear read deadline, best effort
			_ = t.setReadDeadline(time.Time{})

			// check if reader is alive
			_, err = t.r.Read(p[:0])
			if err != nil {
				t.err.Store(err)
				return p[:0], false, err
			}

			break
		}

		n, err = t.r.Read(p[1:])

		n++
		p[0] = firstByte

		if err != nil {
			if !isDeadlineExceeded(err) {
				// store unexpected error
				t.err.Store(err)
				// read failed, signal not able to set read deadline
				close(t.cannotSetDeadline)

				return p[:n], true, err
			}

			return p[:n], true, ErrDeadlineExceeded
		}

		return p[:n], true, nil
	}

	// take a snapshot for the size of buffered data
	t.mu.RLock()
	size := len(t.buf)
	t.mu.RUnlock()

	n = size
	if n > maxReadSize {
		n = maxReadSize
		// can fill provided buffer (p), return now

		err = t.Error()

		t.mu.Lock()

		data = t.buf[:n]
		t.buf = t.buf[n:]

		// handle has data check for WaitForData
		if size == 0 && err == nil {
			// no data buffered, need to wait for it next time
			t.hasData = make(chan struct{})
		}

		t.mu.Unlock()

		return data, false, err
	}

	// unable to fill provided buffer (p), read with timeout

	dataFullSig := t.requestMaxRead(maxReadSize)

	if !t.timer.Reset(maxWait) {
		// failed to rest, timer fired, try to fix
		if !t.timer.Stop() {
			select {
			case <-t.timer.C:
			default:
				// should not happened, just in case
			}
		}

		// best effort
		_ = t.timer.Reset(maxWait)
	}

	defer func() {
		// clean up timer no matter fired or not
		if !t.timer.Stop() {
			select {
			case <-t.timer.C:
			default:
				// should not happened, just in case
			}
		}
	}()

	isTimeout := false
	select {
	case <-dataFullSig:
	case <-t.timer.C:
		isTimeout = true
	}

	// take a snapshot again for the size of buffered data
	t.mu.RLock()
	size = len(t.buf)
	t.mu.RUnlock()

	err = t.Error()

	if err == nil && isTimeout {
		err = ErrDeadlineExceeded
	}

	if size == 0 {
		// should only happen when fallback reading got some error
		return p[:0], true, err
	}

	n = size
	if n > maxReadSize {
		// do not overflow
		n = maxReadSize
	}

	t.mu.Lock()

	data = t.buf[:n]
	t.buf = t.buf[n:]

	// handle has data check for WaitForData
	if len(t.buf) == 0 && err == ErrDeadlineExceeded {
		// no data buffered, need to wait for it next time
		t.hasData = make(chan struct{})
	}

	t.mu.Unlock()

	return data, false, err
}
