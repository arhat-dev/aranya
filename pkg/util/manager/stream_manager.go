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
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"arhat.dev/pkg/iohelper"
	"arhat.dev/pkg/queue"

	"arhat.dev/aranya-proto/aranyagopb"
)

const (
	activeStreamCloseDelay    = 5 * time.Second
	completedStreamCloseDelay = 500 * time.Millisecond
)

type Stream struct {
	sid uint64
	mgr *StreamManager

	startedAt time.Time
	ctx       context.Context
	cancel    context.CancelFunc

	r   io.ReadCloser
	w   io.WriteCloser
	seq *queue.SeqQueue

	resizeCh chan *aranyagopb.TtyResizeOptions

	toBeClosed uint32
}

func (s *Stream) Context() context.Context {
	return s.ctx
}

func (s *Stream) Reader() io.Reader {
	return s.r
}

func (s *Stream) ResizeCh() <-chan *aranyagopb.TtyResizeOptions {
	return s.resizeCh
}

func (s *Stream) resize(size *aranyagopb.TtyResizeOptions) bool {
	if s.resizeCh == nil {
		return false
	}

	select {
	case <-s.ctx.Done():
		return false
	case s.resizeCh <- size:
	}

	return true
}

func (s *Stream) write(seq uint64, data []byte) bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
	}

	switch {
	case s.r == nil, s.w == nil, s.seq == nil:
		return false
	}

	seqData, completed := s.seq.Offer(seq, data)
	for _, da := range seqData {
		select {
		case <-s.ctx.Done():
			return false
		default:
			_, err := s.w.Write(da.([]byte))
			if err != nil {
				return false
			}
		}
	}

	if completed {
		if atomic.LoadUint32(&s.toBeClosed) == 1 {
			if _, removed := s.mgr.tq.Remove(s.sid); removed {
				_ = s.mgr.tq.OfferWithDelay(s.sid, s, completedStreamCloseDelay)
			}
		} else {
			_ = s.mgr.tq.OfferWithDelay(s.sid, s, completedStreamCloseDelay)
		}
	}

	return true
}

func (s *Stream) scheduleClose(maxSeq uint64) {
	if !atomic.CompareAndSwapUint32(&s.toBeClosed, 0, 1) {
		// no input attached or already scheduled
		return
	}

	closeDelay := activeStreamCloseDelay
	if s.seq != nil && s.seq.SetMaxSeq(maxSeq) {
		// stream completed, only delay for local data transmission
		closeDelay = completedStreamCloseDelay
	}

	_ = s.mgr.tq.OfferWithDelay(s.sid, s, closeDelay)
}

func (s *Stream) close() {
	if s.w != nil {
		if f, ok := s.w.(*os.File); ok {
			_ = f.Sync()
		}

		_ = s.w.Close()
	}

	if s.r != nil {
		if f, ok := s.r.(*os.File); ok {
			_ = f.Sync()
			_ = f.SetReadDeadline(time.Now())
		}

		_ = s.r.Close()
	}

	if s.resizeCh != nil {
		select {
		case <-s.ctx.Done():
		default:
			close(s.resizeCh)
		}
	}

	s.cancel()
}

func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[uint64]*Stream),
		tq:      queue.NewTimeoutQueue(),
		mu:      new(sync.RWMutex),
	}
}

type StreamManager struct {
	streams map[uint64]*Stream
	tq      *queue.TimeoutQueue
	mu      *sync.RWMutex
}

func (m *StreamManager) NewStream(parentCtx context.Context, sid uint64, hasInput, hasResize bool) *Stream {
	ctx, cancel := context.WithCancel(parentCtx)
	s := &Stream{
		sid: sid,
		mgr: m,

		startedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	if hasInput {
		s.r, s.w = iohelper.Pipe()
		s.seq = queue.NewSeqQueue()

		if hasResize {
			s.resizeCh = make(chan *aranyagopb.TtyResizeOptions, 1)
		}
	}

	s.mgr.add(sid, s)

	return s
}

func (m *StreamManager) Start(stopCh <-chan struct{}) {
	m.tq.Start(stopCh)
	go func() {
		takeCh := m.tq.TakeCh()

		for t := range takeCh {
			m.Close(t.Key.(uint64))
		}
	}()
}

func (m *StreamManager) Resize(sid uint64, opts *aranyagopb.TtyResizeOptions) bool {
	if s, ok := m.get(sid); ok {
		return s.resize(opts)
	}

	return false
}

func (m *StreamManager) Write(sid, seq uint64, data []byte) bool {
	if s, ok := m.get(sid); ok {
		return s.write(seq, data)
	}

	return false
}

func (m *StreamManager) Close(sid uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.streams[sid]
	if !ok {
		return
	}

	if atomic.LoadUint32(&s.toBeClosed) == 1 {
		m.tq.Remove(sid)
		s.close()
		delete(m.streams, sid)
	} else {
		s.scheduleClose(0)
	}
}

func (m *StreamManager) CloseRead(sid, maxSeq uint64) {
	s, ok := m.get(sid)
	if !ok {
		return
	}

	s.scheduleClose(maxSeq)
}

func (m *StreamManager) add(sid uint64, s *Stream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if prevStream, ok := m.streams[sid]; ok {
		if prevStream != s {
			prevStream.close()
			delete(m.streams, sid)
		}
	}

	if s != nil {
		m.streams[sid] = s
	}
}

func (m *StreamManager) get(sid uint64) (*Stream, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.streams[sid]
	if !ok {
		return nil, false
	}

	return s, true
}
