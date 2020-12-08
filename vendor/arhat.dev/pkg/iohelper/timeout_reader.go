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
	"bufio"
	"io"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

// NewTimeoutReader creates a new idle timeout reader from a blocking reader
func NewTimeoutReader(r io.Reader) *TimeoutReader {
	var (
		setReadDeadline   func(t time.Time) error
		cannotSetDeadline = make(chan struct{})
	)

	// check if can set read deadline
	rs, canSetReadDeadline := r.(interface {
		SetReadDeadline(t time.Time) error
	})

	// clear read deadline in advance to check set deadline capability
	if canSetReadDeadline && rs.SetReadDeadline(time.Time{}) == nil {
		setReadDeadline = rs.SetReadDeadline
	} else {
		// mark set read deadline not working
		close(cannotSetDeadline)
	}

	// create an idle timer when reader doesn't support SetReadDeadline
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	return &TimeoutReader{
		buf: make([]byte, 0, 8),

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

		err: new(atomic.Value),

		_working: 0,
	}
}

// TimeoutReader is a reader with read timeout support
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
// 	but for packet oriented connections (in this case MQTT), send one byte per packet
// 	will significantly increase protocol overhead.
//
// 	with TimeoutReader we can read data generated in some interval (say 20ms), no
//  real-time experience lost while still keep protocol overhead at a reasonable level
type TimeoutReader struct {
	buf []byte

	timer *time.Timer

	setReadDeadline   func(t time.Time) error
	cannotSetDeadline chan struct{}

	r io.Reader

	// signal to notify user can cal Read
	hasData chan struct{}

	maxRead  int
	dataFull chan struct{}

	readOneByteReqCh chan struct{}
	firstByteReady   chan struct{}
	firstByte        chan byte

	err *atomic.Value

	_working uint32
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
	var ret <-chan struct{}
	t.doExclusive(func() {
		t.maxRead = size

		select {
		case <-t.dataFull:
			t.dataFull = make(chan struct{})
		default:
			// reuse sig channel
		}

		ret = t.dataFull
	})

	return ret
}

func (t *TimeoutReader) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&t._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&t._working, 0)
}

type bufferedReader interface {
	io.Reader
	io.ByteReader
	Buffered() int
}

type fdBufferedReader struct {
	fd uintptr
	io.Reader

	oneByteBuf []byte
}

func (r *fdBufferedReader) ReadByte() (byte, error) {
	_, err := io.ReadFull(r, r.oneByteBuf[:1])
	return r.oneByteBuf[0], err
}

func (r *fdBufferedReader) Buffered() int {
	n, _ := CheckBytesToRead(r.fd)
	return n
}

// FallbackReading is a helper routine for data reading/waiting
// for readers with SetReadDeadline support, it is used for waiting until data prepared (WaitForData)
// for other readers without SetReadDeadline, it is in (buffered) one byte read mode (WaitForData and Read)
//
// this function will block until EOF or error reading, so must be called in a goroutine other than
// the one you are reading data, and once called, you should never access the raw reader you have provided
// directly unless you are sure it is not read
//
// NOTE: this function MUST be called exactly once
// nolint:gocyclo
func (t *TimeoutReader) FallbackReading(stopSig <-chan struct{}) {
	var (
		n          int
		err        error
		oneByteBuf [1]byte
	)

loop:
	for {
		select {
		case <-stopSig:
			return
		case <-t.cannotSetDeadline:
			break loop
		case <-t.readOneByteReqCh:
			// user called WaitForData
			//
			// clear read deadline, read in block mode
			err = t.setReadDeadline(time.Time{})
			if err != nil {
				// set read deadline not working
				close(t.cannotSetDeadline)
				break loop
			}

			// read one byte in blocking mode
			n, err = t.r.Read(oneByteBuf[:1])
			if n == 1 {
				// notify wait for data
				t.firstByteReady <- struct{}{}
				t.firstByte <- oneByteBuf[0]
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
	_, err = t.r.Read(oneByteBuf[:0])
	if err != nil {
		// reader got some error, usually closed
		close(t.hasData)
		t.err.Store(err)
		return
	}

	br, isBufferedReader := t.r.(bufferedReader)
	if !isBufferedReader {

		var (
			fd    uintptr
			hasFd = false
		)
		switch r := t.r.(type) {
		case interface{ Fd() uintptr }:
			fd = r.Fd()

			// test if syscall supported
			_, err = CheckBytesToRead(fd)
			hasFd = err == nil
		case interface {
			SyscallConn() (syscall.RawConn, error)
		}:
			var (
				rawConn syscall.RawConn
			)
			rawConn, err = r.SyscallConn()
			if err != nil {
				break
			}

			err = rawConn.Control(func(_fd uintptr) {
				fd = _fd
			})
			if err != nil {
				break
			}

			// test if syscall supported
			_, err = CheckBytesToRead(fd)
			hasFd = err == nil
		}

		if hasFd {
			br = &fdBufferedReader{
				fd:         fd,
				Reader:     t.r,
				oneByteBuf: oneByteBuf[:],
			}
		} else {
			br = bufio.NewReader(t.r)
		}
	}

	var initialByte byte
	for {
		// read one byte to avoid being blocked
		initialByte, err = br.ReadByte()
		if err != nil {
			t.err.Store(err)

			// release signal to cancel potential wait
			t.doExclusive(func() {
				select {
				case <-t.hasData:
				default:
					close(t.hasData)
				}
			})

			return
		}

		// avoid unexpected access to t.buf
		t.doExclusive(func() {
			// rely on the default slice grow
			t.buf = append(t.buf, initialByte)

			// read all buffered data
			start := len(t.buf)
			n = br.Buffered()
			end := start + n

			// notify read wait
			select {
			case <-t.hasData:
			default:
				close(t.hasData)
			}

			if n == 0 {
				// notify data full
				if end >= t.maxRead {
					select {
					case <-t.dataFull:
					default:
						close(t.dataFull)
					}
				}
				return
			}

			if c := cap(t.buf); c < end {
				// grow slice for reading buffered data
				buf := make([]byte, end, 2*c+n)
				_ = copy(buf, t.buf)
				t.buf = buf
			}

			// usually should read end-start bytes, record n just in case
			n, err = br.Read(t.buf[start:end])
			if err != nil {
				t.err.Store(err)
				// usually should not happen since has been buffered
				//
				// do not return here since we wiil check error outside
			}

			end = start + n
			t.buf = t.buf[:end]

			// notify data full
			if end >= t.maxRead {
				select {
				case <-t.dataFull:
				default:
					close(t.dataFull)
				}
			}
		})

		if err != nil {
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
	var ret bool
	t.doExclusive(func() {
		ret = len(t.buf) != 0
	})

	return ret
}

// WaitForData is a helper function used to check if there is data available in reader
// so we can reduce actual call of Read when the timeout is a short duration
//
// when return value is true, you MUST call Read to read data and you can read at least
// one byte from the underlying reader
//
// otherwise, false means the provided cancel has signaled or error happened, you can no
// longer read from the underlying reader if Error() returned non-nil value.
func (t *TimeoutReader) WaitForData(cancel <-chan struct{}) bool {
	select {
	case <-cancel:
		return false
	case <-t.cannotSetDeadline:
		// not using set read deadline
		break
	case t.readOneByteReqCh <- struct{}{}:
		// using set read deadline, wait for first byte
		select {
		case <-cancel:
			return false
		case <-t.cannotSetDeadline:
			// in one byte read mode, wait until has data
			break
		case _, ok := <-t.firstByteReady:
			// can set read timeout and have read first byte
			//
			// NOTE: this case branch must be after t.cannotSetDeadline
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
	var hasData <-chan struct{}
	t.doExclusive(func() { hasData = t.hasData })

	select {
	case <-cancel:
		return false
	case <-hasData:
		if t.Error() != nil {
			// if no data buffered but signal released, usually error happened when reading in background
			return t.hasDataInBuf()
		}

		return true
	}
}

// Read performs a read operation with timeout option, will block until maxWait exceeded or p is full
//
// if the function returned because of timeout, the returned error is ErrDeadlineExceeded
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
			if !IsDeadlineExceeded(err) {
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
	var size int
	t.doExclusive(func() { size = len(t.buf) })

	n = size
	if n > maxReadSize {
		n = maxReadSize
		// can fill provided buffer (p), return now

		err = t.Error()

		t.doExclusive(func() {
			data = t.buf[:n]
			t.buf = t.buf[n:]

			// handle has data check for WaitForData
			if size == 0 && err == nil {
				// no data buffered, need to wait for it next time
				t.hasData = make(chan struct{})
			}
		})

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

	// take a snapshot for the size of buffered data again after timeout
	t.doExclusive(func() { size = len(t.buf) })

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

	t.doExclusive(func() {
		data = t.buf[:n]
		t.buf = t.buf[n:]

		// handle has data check for WaitForData
		if len(t.buf) == 0 && err == ErrDeadlineExceeded {
			// no data buffered, need to wait for it next time
			t.hasData = make(chan struct{})
		}
	})

	return data, false, err
}
