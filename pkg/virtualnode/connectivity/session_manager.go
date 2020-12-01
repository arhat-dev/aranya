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
	"bytes"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/queue"
)

func newSession(epoch uint64, isStream bool) *session {
	return &session{
		epoch: epoch,
		msgCh: make(chan interface{}, 1),

		// initialized when necessary
		msgBuffer: nil,
		seqQ:      nil,

		_closed:  0,
		_working: 0,

		isStream: isStream,
	}
}

type session struct {
	epoch     uint64
	msgCh     chan interface{}
	msgBuffer *bytes.Buffer
	seqQ      *queue.SeqQueue

	_working uint32
	_closed  uint32

	isStream bool
}

func (s *session) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&s._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&s._working, 0)
}

func (s *session) closed() bool {
	return atomic.LoadUint32(&s._closed) == 1
}

func (s *session) deliverMsg(msg *aranyagopb.Msg) (delivered, complete bool) {
	if s.closed() {
		// session closed, no message shall be delivered
		return false, true
	}

	// deliver message exclusively to ensure data order
	s.doExclusive(func() {
		// all in one packet, no need to reassemble packets
		if msg.Seq == 0 && msg.Completed {
			switch msg.Kind {
			case aranyagopb.MSG_DATA, aranyagopb.MSG_DATA_STDERR:
				s.msgCh <- &Data{
					Kind:    msg.Kind,
					Payload: msg.Payload,
				}
			default:
				s.msgCh <- msg
			}

			s.close()
			delivered, complete = true, true
			return
		}

		if s.seqQ == nil {
			s.seqQ = queue.NewSeqQueue()
		}

		var orderedChunks []interface{}
		orderedChunks, complete = s.seqQ.Offer(msg.Seq, &Data{
			Kind:    msg.Kind,
			Payload: msg.Payload,
		})

		if msg.Completed {
			complete = s.seqQ.SetMaxSeq(msg.Seq)
		}

		// handle ordered data chunks
		for _, ck := range orderedChunks {
			data := ck.(*Data)

			switch data.Kind {
			case aranyagopb.MSG_DATA, aranyagopb.MSG_DATA_STDERR:
				// is stream data, send directly
				s.msgCh <- data
			default:
				// is message data, collect until complete
				// message data can be the last several data chunks in a stream
				// if there are multiple message data chunks with different types
				// usually will cause unmarshal error, so we don't take care of
				// it here
				if s.msgBuffer == nil {
					s.msgBuffer = new(bytes.Buffer)
				}

				s.msgBuffer.Write(data.Payload)
			}
		}

		if !complete {
			delivered = true
			return
		}

		// session complete, no more message shall be sent through this session

		if s.msgBuffer != nil {
			s.msgCh <- &aranyagopb.Msg{
				Kind:      msg.Kind,
				Sid:       msg.Sid,
				Seq:       0,
				Completed: true,
				Payload:   s.msgBuffer.Bytes(),
			}
		}

		s.close()

		delivered = true
	})

	return delivered, complete
}

func (s *session) close() {
	if s.closed() {
		return
	}

	atomic.StoreUint32(&s._closed, 1)

	if s.msgBuffer != nil {
		s.msgBuffer.Reset()
		s.msgBuffer = nil
	}

	s.seqQ = nil

	if s.msgCh != nil {
		close(s.msgCh)
		s.msgCh = nil
	}
}

func NewSessionManager() *SessionManager {
	now := uint64(time.Now().UTC().UnixNano())
	return &SessionManager{
		sessions: make(map[uint64]*session),
		tq:       queue.NewTimeoutQueue(),

		sid:   now,
		epoch: now,

		mu: new(sync.RWMutex),
	}
}

type SessionManager struct {
	sessions map[uint64]*session
	tq       *queue.TimeoutQueue

	sid   uint64
	epoch uint64

	mu *sync.RWMutex
}

// nextSid is only intended to be used when s.mu has been locked
func (m *SessionManager) nextSid() uint64 {
	used := true
	for used {
		m.sid++
		_, used = m.sessions[m.sid]
	}

	return m.sid
}

func (m *SessionManager) Start(stopCh <-chan struct{}) error {
	m.tq.Start(stopCh)

	go func() {
		timeoutCh := m.tq.TakeCh()

		for {
			select {
			case session, more := <-timeoutCh:
				if !more {
					return
				}

				if sid, ok := session.Key.(uint64); ok {
					data, _ := (&aranyagopb.ErrorMsg{
						Kind:        aranyagopb.ERR_TIMEOUT,
						Description: "timeout",
						Code:        0,
					}).Marshal()

					m.Dispatch(&aranyagopb.Msg{
						Kind:      aranyagopb.MSG_ERROR,
						Sid:       sid,
						Seq:       0,
						Completed: true,
						Payload:   data,
					})
					m.Delete(sid)
				}
			case <-stopCh:
				for len(timeoutCh) > 0 {
					<-timeoutCh
				}
				return
			}
		}
	}()

	return nil
}

// Add or reuse a session
func (m *SessionManager) Add(
	sid uint64,
	timeout time.Duration,
) (
	realSid uint64,
	ch chan interface{},
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	realSid = sid
	// check if old sid valid
	if oldSession, ok := m.sessions[realSid]; ok {
		// reuse old channel if valid
		ch = oldSession.msgCh
	} else {
		// create new channel if invalid
		// if timeout is 0, it's a stream session
		session := newSession(m.epoch, timeout == 0)

		ch = session.msgCh
		realSid = m.nextSid()

		if timeout > 0 {
			_ = m.tq.OfferWithDelay(realSid, session, timeout)
		}

		m.sessions[realSid] = session
	}

	return realSid, ch
}

func (m *SessionManager) Dispatch(msg *aranyagopb.Msg) bool {
	if msg == nil {
		// ignore invalid Msgs
		return true
	}

	sid := msg.Sid

	m.mu.RLock()
	session, ok := m.sessions[sid]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	// only deliver msg for this epoch or it is stream message
	if session.epoch != m.epoch && !session.isStream {
		m.Delete(sid)
		return false
	}

	delivered, complete := session.deliverMsg(msg)
	if complete {
		m.Delete(sid)
	}

	return delivered
}

func (m *SessionManager) Delete(sid uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[sid]; ok {
		session.doExclusive(session.close)

		m.tq.Remove(sid)
		delete(m.sessions, sid)
	}
}

func (m *SessionManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	allSid := make([]uint64, len(m.sessions))
	i := 0
	for sid, session := range m.sessions {
		if session.isStream {
			// do not close streams
			continue
		}

		session.doExclusive(session.close)

		m.tq.Remove(sid)
		allSid[i] = sid
		i++
	}

	for _, sid := range allSid {
		delete(m.sessions, sid)
	}

	now := uint64(time.Now().UTC().UnixNano())

	m.epoch = now
	// m.sid > now is the rare case, but we have to make sure sid is
	// always increasing in this session manager
	if m.sid < now {
		m.sid = now
	}
}

func (m *SessionManager) Remains() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.sessions) == 0 {
		return nil
	}

	result := make([]uint64, len(m.sessions))
	i := 0
	for sid := range m.sessions {
		result[i] = sid
		i++
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}

func (m *SessionManager) TimedRemains() []queue.TimeoutData {
	return m.tq.Remains()
}
