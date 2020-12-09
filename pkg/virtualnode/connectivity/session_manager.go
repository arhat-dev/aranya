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
	"encoding/binary"
	"io"
	"io/ioutil"
	"runtime"
	"sort"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/wellknownerrors"
)

func newSession(epoch uint64, isStream, keepPacket bool) *session {
	return &session{
		epoch: epoch,
		msgCh: make(chan *aranyagopb.Msg, 1),

		// initialized when necessary
		msgBuffer: nil,
		seqQ:      nil,

		dataOut:      nil,
		errOut:       nil,
		streamReady:  nil,
		_streamReady: 0,
		canWrite:     nil,

		_working: 0,
		_closed:  make(chan struct{}),

		isStream:    isStream,
		prependSize: keepPacket,
	}
}

// nolint:maligned
type session struct {
	epoch uint64
	msgCh chan *aranyagopb.Msg

	msgBuffer *bytes.Buffer
	// seq queue is resued by both msg and data streaming
	seqQ *queue.SeqQueue

	// data streaming
	dataOut io.Writer
	errOut  io.Writer
	// once it's set, this session can accept data stream
	streamReady  chan struct{}
	_streamReady uint32
	canWrite     <-chan struct{}

	_working uint32
	_closed  chan struct{}

	isStream    bool
	prependSize bool
}

func (s *session) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&s._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&s._working, 0)
}

func (s *session) closed() bool {
	select {
	case <-s._closed:
		return true
	default:
		return false
	}
}

func (s *session) setStream(dataOut, errOut io.Writer, ready chan struct{}, canWrite <-chan struct{}) bool {
	set := false
	s.doExclusive(func() {
		if s.streamReady != nil {
			return
		}

		set = true
		if dataOut == nil {
			dataOut = ioutil.Discard
		}

		if errOut == nil {
			errOut = dataOut
		}

		s.dataOut = dataOut
		s.errOut = errOut
		s.streamReady = ready
		s._streamReady = 0
		s.canWrite = canWrite
	})

	return set
}

func (s *session) writeToStream(kind aranyagopb.MsgType, data []byte) error {
	if !s.isStream {
		return wellknownerrors.ErrNotSupported
	}

	if atomic.CompareAndSwapUint32(&s._streamReady, 0, 1) {
		// notify ready on first data write
		select {
		case <-s.streamReady:
			// closed by others, this stream is discarded
			return wellknownerrors.ErrNotSupported
		default:
			close(s.streamReady)
		}
	}

	if s.canWrite != nil {
		// wait until can write to data
		// TODO: currently only custom port-forward will use this and will be closed
		//       immediately after wrote response header, we may need to update the lock
		//       implementation once we have other time consuming preparation
		select {
		case <-s.canWrite:
		case <-s._closed:
			return nil
		}
	}

	target := s.dataOut
	if kind == aranyagopb.MSG_DATA_STDERR {
		target = s.errOut
	}

	var (
		n    int
		size = len(data)
		err  error
	)

	if s.prependSize {
		sizeBuf := make([]byte, 10)
		n = binary.PutUvarint(sizeBuf, uint64(size))
		_, err = target.Write(sizeBuf[:n])
		if err != nil {
			// do not write multiple times for size
			return err
		}
	}

	var sum int
	for sum != size {
		n, err = target.Write(data[sum:])
		if err != nil {
			return err
		}

		sum += n
	}

	return nil
}

type data struct {
	kind    aranyagopb.MsgType
	payload []byte
}

func (s *session) deliverMsg(msg *aranyagopb.Msg) (delivered, complete bool) {
	if s.closed() {
		// session closed, no message shall be delivered
		return false, true
	}

	if msg.Seq == 0 && msg.Complete {
		// all in one packet, no need to reassemble packets
		switch msg.Kind {
		case aranyagopb.MSG_DATA, aranyagopb.MSG_DATA_STDERR:
			err := s.writeToStream(msg.Kind, msg.Payload)
			if err != nil {
				errMsgData, _ := (&aranyagopb.ErrorMsg{
					Kind:        aranyagopb.ERR_COMMON,
					Description: err.Error(),
				}).Marshal()

				select {
				case <-s._closed:
				case s.msgCh <- &aranyagopb.Msg{
					Kind:     aranyagopb.MSG_ERROR,
					Sid:      msg.Sid,
					Seq:      0,
					Complete: true,
					Payload:  errMsgData,
				}:
				}
			}
		default:
			select {
			case <-s._closed:
			case s.msgCh <- msg:
			}
		}

		s.close()
		return true, true
	}

	// deliver message exclusively to ensure data order
	s.doExclusive(func() {
		if s.msgBuffer == nil {
			s.msgBuffer = new(bytes.Buffer)
		}

		if s.seqQ != nil {
			return
		}

		s.seqQ = queue.NewSeqQueue(func(seq uint64, d interface{}) {
			data := d.(*data)

			switch data.kind {
			case aranyagopb.MSG_DATA, aranyagopb.MSG_DATA_STDERR:
				// is stream data, send directly
				err := s.writeToStream(data.kind, data.payload)
				if err != nil {
					errMsgData, _ := (&aranyagopb.ErrorMsg{
						Kind:        aranyagopb.ERR_COMMON,
						Description: err.Error(),
					}).Marshal()

					select {
					case <-s._closed:
					case s.msgCh <- &aranyagopb.Msg{
						Kind:     aranyagopb.MSG_ERROR,
						Sid:      msg.Sid,
						Seq:      0,
						Complete: true,
						Payload:  errMsgData,
					}:
					}

					return
				}
			default:
				// is message data, collect until complete
				// message data can be the last several data chunks in a stream
				// if there are multiple message data chunks with different types
				// usually will cause unmarshal error, so we don't take care of
				// it here

				s.msgBuffer.Write(data.payload)
			}
		})
	})

	if msg.Complete {
		_ = s.seqQ.SetMaxSeq(msg.Seq)
	}

	complete = s.seqQ.Offer(msg.Seq, &data{
		kind:    msg.Kind,
		payload: msg.Payload,
	})

	if !complete {
		return true, false
	}

	// session complete, no more message shall be sent through this session
	// check if we have any thing remaining in the message buffer

	s.close()

	s.doExclusive(func() {
		if s.msgBuffer == nil {
			return
		}
		s.msgCh <- &aranyagopb.Msg{
			Kind:     msg.Kind,
			Sid:      msg.Sid,
			Seq:      0,
			Complete: true,
			Payload:  s.msgBuffer.Bytes(),
		}
	})

	return true, true
}

func (s *session) close() {
	select {
	case <-s._closed:
		return
	default:
		close(s._closed)
	}

	if atomic.CompareAndSwapUint32(&s._streamReady, 0, 1) {
		if s.streamReady != nil {
			// release stream preparation
			select {
			case <-s.streamReady:
				// could be canceled by others
			default:
				close(s.streamReady)
			}
		}
	}

	if s.msgBuffer != nil {
		s.msgBuffer.Reset()
		s.msgBuffer = nil
	}

	s.dataOut = nil
	s.errOut = nil
	s.seqQ = nil

	if s.msgCh != nil {
		close(s.msgCh)
		s.msgCh = nil
	}
}

func NewSessionManager(
	addTimedSession TimedSessionAddFunc,
	delTimedSession TimedSessionDelFunc,
	getTimedSessions TimedSessionGetFunc,
	addTimedStreamCreation TimedSessionAddFunc,
) *SessionManager {
	now := uint64(time.Now().UTC().UnixNano())
	return &SessionManager{
		sessions: make(map[uint64]*session),

		addTimedSession:  addTimedSession,
		delTimedSession:  delTimedSession,
		getTimedSessions: getTimedSessions,

		addTimedStreamCreation: addTimedStreamCreation,

		sid:   now,
		epoch: now,
	}
}

type SessionManager struct {
	sessions map[uint64]*session

	addTimedSession  TimedSessionAddFunc
	delTimedSession  TimedSessionDelFunc
	getTimedSessions TimedSessionGetFunc

	addTimedStreamCreation TimedSessionAddFunc

	sid   uint64
	epoch uint64

	_working uint32
}

func (m *SessionManager) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&m._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&m._working, 0)
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

func (m *SessionManager) SetStream(
	sid uint64, dataOut, errOut io.Writer, canWrite <-chan struct{},
) (<-chan struct{}, bool) {
	var (
		session *session
		ok      bool
	)
	m.doExclusive(func() {
		session, ok = m.sessions[sid]
	})

	if !ok {
		return nil, false
	}

	ret := make(chan struct{})
	if !session.setStream(dataOut, errOut, ret, canWrite) {
		return nil, false
	}

	m.addTimedStreamCreation(sid, func() {
		select {
		case <-ret:
			// can be closed by session
		default:
			close(ret)
		}
	})

	return ret, true
}

// Add or reuse a session
func (m *SessionManager) Add(
	sid uint64,
	stream bool,
	keepPacket bool,
) (
	realSid uint64,
	ch chan *aranyagopb.Msg,
) {
	m.doExclusive(func() {
		realSid = sid
		// check if old sid valid
		if oldSession, ok := m.sessions[realSid]; ok {
			// reuse old channel if valid
			ch = oldSession.msgCh
		} else {
			// create new channel if invalid
			session := newSession(m.epoch, stream, keepPacket)

			ch = session.msgCh
			realSid = m.nextSid()

			if !stream {
				m.addTimedSession(realSid, func() {
					data, _ := (&aranyagopb.ErrorMsg{
						Kind:        aranyagopb.ERR_TIMEOUT,
						Description: "timeout",
						Code:        0,
					}).Marshal()

					m.Dispatch(&aranyagopb.Msg{
						Kind:     aranyagopb.MSG_ERROR,
						Sid:      sid,
						Seq:      0,
						Complete: true,
						Payload:  data,
					})

					m.Delete(sid)
				})
			}

			m.sessions[realSid] = session
		}
	})

	return
}

func (m *SessionManager) Dispatch(msg *aranyagopb.Msg) bool {
	if msg == nil {
		// ignore invalid Msgs
		return true
	}

	sid := msg.Sid

	var (
		session *session
		ok      bool
	)
	m.doExclusive(func() {
		session, ok = m.sessions[sid]
	})

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
	m.doExclusive(func() {
		if session, ok := m.sessions[sid]; ok {
			session.doExclusive(session.close)

			m.delTimedSession(sid)
			delete(m.sessions, sid)
		}
	})
}

func (m *SessionManager) Cleanup() {
	m.doExclusive(func() {
		allSid := make([]uint64, len(m.sessions))
		i := 0
		for sid, session := range m.sessions {
			if session.isStream {
				// do not close streams
				continue
			}

			session.doExclusive(session.close)

			m.delTimedSession(sid)
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
	})
}

func (m *SessionManager) Remains() []uint64 {
	var result []uint64

	m.doExclusive(func() {
		if len(m.sessions) == 0 {
			return
		}

		result = make([]uint64, len(m.sessions))
		i := 0
		for sid := range m.sessions {
			result[i] = sid
			i++
		}
	})

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result
}

func (m *SessionManager) TimedRemains() []uint64 {
	ret := m.getTimedSessions()

	sort.Slice(ret, func(i, j int) bool {
		return ret[i] < ret[j]
	})

	return ret
}
