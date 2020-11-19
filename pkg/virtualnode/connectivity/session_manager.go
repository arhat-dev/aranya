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
	"sync"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/queue"
)

func newSession(epoch uint64) *session {
	return &session{
		msgCh: make(chan interface{}, 1),
		epoch: epoch,
		seqQ:  queue.NewSeqQueue(),
		mu:    new(sync.RWMutex),
	}
}

type session struct {
	epoch uint64

	closed bool
	msgCh  chan interface{}

	msgBuffer *bytes.Buffer
	seqQ      *queue.SeqQueue
	mu        *sync.RWMutex
}

func (s *session) deliverMsg(msg *aranyagopb.Msg) (delivered, complete bool) {
	s.mu.RLock()
	if s.closed {
		// session closed, no message shall be delivered
		return false, true
	}

	var (
		sid  = msg.Sid
		seq  = msg.Seq
		kind = msg.Kind
	)

	dataChunks, complete := s.seqQ.Offer(seq, msg.Payload)

	if msg.Completed {
		complete = s.seqQ.SetMaxSeq(seq)
	}

	switch kind {
	case aranyagopb.MSG_DATA_DEFAULT, aranyagopb.MSG_DATA_STDERR:
		// stream data
		for _, ck := range dataChunks {
			// nolint:gosimple
			select {
			case s.msgCh <- &Data{
				Kind:    kind,
				Payload: ck.([]byte),
			}:
				//	TODO: add context cancel
			}
		}

		s.mu.RUnlock()
		if complete {
			s.mu.Lock()
		}
	default:
		s.mu.RUnlock()
		s.mu.Lock()
		if s.msgBuffer == nil {
			s.msgBuffer = new(bytes.Buffer)
		}

		// normal msg
		for _, ck := range dataChunks {
			s.msgBuffer.Write(ck.([]byte))
		}

		if complete {
			// nolint:gosimple
			select {
			// deliver a new message with complete msgBytes
			case s.msgCh <- &aranyagopb.Msg{
				Kind:      kind,
				Sid:       sid,
				Seq:       seq,
				Completed: true,
				Payload:   s.msgBuffer.Bytes(),
			}:
				//	TODO: add context cancel
			}
		} else {
			s.mu.Unlock()
		}
	}

	if complete {
		s.closed = true
		if s.msgBuffer != nil {
			s.msgBuffer.Reset()
		}

		if s.msgCh != nil {
			close(s.msgCh)
			s.msgCh = nil
		}

		s.mu.Unlock()
	}

	return true, complete
}

func (s *session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	if s.msgCh != nil {
		close(s.msgCh)
		s.msgCh = nil
	}
}

func NewSessionManager() *SessionManager {
	now := uint64(time.Now().UTC().UnixNano())
	return &SessionManager{
		m:  make(map[uint64]*session),
		tq: queue.NewTimeoutQueue(),
		mu: new(sync.RWMutex),

		sid:   now,
		epoch: now,
	}
}

type SessionManager struct {
	m  map[uint64]*session
	tq *queue.TimeoutQueue

	sid   uint64
	epoch uint64

	mu *sync.RWMutex
}

// nextSid is only intended to be used when s.mu has been locked
func (m *SessionManager) nextSid() uint64 {
	used := true
	for used {
		m.sid++
		_, used = m.m[m.sid]
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
) (realSid uint64, ch chan interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	realSid = sid
	// check if old sid valid
	if oldSession, ok := m.m[realSid]; ok {
		// reuse old channel if valid
		ch = oldSession.msgCh
	} else {
		// create new channel if invalid
		session := newSession(m.epoch)

		ch = session.msgCh
		realSid = m.nextSid()

		if timeout > 0 {
			_ = m.tq.OfferWithDelay(realSid, session, timeout)
		}
		m.m[realSid] = session
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
	session, ok := m.m[sid]
	m.mu.RUnlock()

	if ok {
		// only deliver msg for this epoch or it is stream message
		session.mu.RLock() // protect session.msgBuffer
		if session.epoch == m.epoch || session.msgBuffer == nil {
			session.mu.RUnlock()
			delivered, complete := session.deliverMsg(msg)

			if complete {
				m.Delete(sid)
			}
			return delivered
		}

		session.mu.RUnlock()
	}

	m.Delete(sid)
	return false
}

func (m *SessionManager) Delete(sid uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.m[sid]; ok {
		session.close()
		m.tq.Remove(sid)
		delete(m.m, sid)
	}
}

func (m *SessionManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	allSid := make([]uint64, len(m.m))
	i := 0
	for sid, session := range m.m {
		if session.msgBuffer == nil {
			// do not close streams
			continue
		}

		session.close()
		m.tq.Remove(sid)
		allSid[i] = sid
		i++
	}

	for _, sid := range allSid {
		delete(m.m, sid)
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

	if len(m.m) == 0 {
		return nil
	}

	result := make([]uint64, len(m.m))
	i := 0
	for sid := range m.m {
		result[i] = sid
		i++
	}
	return result
}

func (m *SessionManager) TimeoutRemains() []queue.TimeoutData {
	return m.tq.Remains()
}
