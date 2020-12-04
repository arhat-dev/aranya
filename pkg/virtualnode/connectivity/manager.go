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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync/atomic"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"
	"arhat.dev/pkg/log"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/sets"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
)

var (
	ErrManagerClosed = errors.New("connectivity manager has been closed")
)

// runtime options
type (
	GRPCOpts struct {
		Server   *grpc.Server
		Listener net.Listener
	}
	MQTTOpts struct {
		TLSConfig *tls.Config
		Username  []byte
		Password  []byte
		Config    aranyaapi.MQTTSpec
	}
	AMQPOpts struct {
		TLSConfig *tls.Config
		Username  []byte
		Password  []byte
		Config    aranyaapi.AMQPSpec
	}
	AzureIoTHubOpts struct {
		IoTHubConnectionString   string
		EventHubConnectionString string
		Config                   aranyaapi.AzureIoTHubSpec
	}
	GCPIoTCoreOpts struct {
		PubSubCredentialsJSON   []byte
		CloudIoTCredentialsJSON []byte
		Config                  aranyaapi.GCPIoTCoreSpec
	}
)

type (
	SessionTimeoutHandleFunc func()
	TimedSessionAddFunc      func(sid uint64, onSessionTimeout SessionTimeoutHandleFunc)
	TimedSessionDelFunc      func(sid uint64)
	TimedSessionGetFunc      func() (timedSessionIDs []uint64)
)

type Options struct {
	AddTimedStreamCreation TimedSessionAddFunc

	AddTimedSession  TimedSessionAddFunc
	DelTimedSession  TimedSessionDelFunc
	GetTimedSessions TimedSessionGetFunc

	GRPCOpts        *GRPCOpts
	MQTTOpts        *MQTTOpts
	AMQPOpts        *AMQPOpts
	AzureIoTHubOpts *AzureIoTHubOpts
	GCPIoTCoreOpts  *GCPIoTCoreOpts
}

func NewManager(parentCtx context.Context, name string, options *Options) (Manager, error) {
	switch {
	case options.MQTTOpts != nil,
		options.AMQPOpts != nil,
		options.AzureIoTHubOpts != nil,
		options.GCPIoTCoreOpts != nil:
		return NewMessageQueueManager(parentCtx, name, options)
	case options.GRPCOpts != nil:
		return NewGRPCManager(parentCtx, name, options)
	default:
		return nil, ErrUnsupportedManager
	}
}

// Manager is the connectivity manager interface, and is designed for message queue based
// managers such as MQTT
type Manager interface {
	// Start manager and block until stopped
	Start() error

	// Close manager immediately
	Close()

	// Reject current device connection if any
	Reject(reason aranyagopb.RejectionReason, message string)

	// Connected signal
	Connected() <-chan struct{}

	// Disconnected signal
	Disconnected() <-chan struct{}

	// GlobalMessages message with no session attached
	GlobalMessages() <-chan *aranyagopb.Msg

	// PostData
	PostData(
		sid uint64,
		kind aranyagopb.CmdType,
		seq uint64, completed bool,
		payload []byte,
	) (
		msgCh <-chan *aranyagopb.Msg,
		realSID, lastSeq uint64,
		err error,
	)

	// PostCmd send a command to remote device with timeout
	// return a channel for messages to be received in the session
	PostCmd(
		sid uint64,
		kind aranyagopb.CmdType,
		payloadCmd proto.Marshaler,
	) (
		msgCh <-chan *aranyagopb.Msg,
		realSID uint64,
		err error,
	)

	// PostStreamCmd is like PostCmd, but will set session in streaming mode, received stream
	// data will be written to dataOut/errOut directly
	PostStreamCmd(
		kind aranyagopb.CmdType,
		payloadCmd proto.Marshaler,
		dataOut, errOut io.Writer,
	) (
		msgCh <-chan *aranyagopb.Msg,
		streamReady <-chan struct{},
		realSID uint64,
		err error,
	)

	// MaxPayloadSize of this kind connectivity method, used to reduce message overhead
	// when handling date streams for port-forward and command execution
	MaxPayloadSize() int

	// OnConnected called after device connected and finished
	//   - node sync initialization
	//   - network sync initialization
	//   - pod sync initialization
	OnConnected(initialize func() (id string))

	// OnDisconnected called after lost of device connection, `finalize`
	// function is used to determine which device lost connection by returning
	// its online id
	OnDisconnected(finalize func() (id string, all bool))
}

type baseManager struct {
	ctx      context.Context
	exit     context.CancelFunc
	config   *Options
	sessions *SessionManager

	maxDataSize int
	log         log.Interface

	// sendCmd registered by the actual implementation
	// it MUST block when connectivity lost
	// it MUST only return error when it is being terminated
	//
	// so the implementation MUST reconnect to server automatically
	sendCmd func(*aranyagopb.Cmd) error

	globalMsgChan    chan *aranyagopb.Msg
	connectedDevices sets.String

	// signals
	connected       chan struct{}
	disconnected    chan struct{}
	rejected        chan struct{}
	alreadyRejected bool

	// status
	_stopped uint32
	_working uint32
}

func newBaseManager(parentCtx context.Context, name string, config *Options) (*baseManager, error) {
	ctx, exit := context.WithCancel(parentCtx)
	disconnected := make(chan struct{})
	close(disconnected)

	var maxDataSize int
	switch {
	case config.MQTTOpts != nil:
		maxDataSize = config.MQTTOpts.Config.MaxPayloadSize
		if maxDataSize <= 0 {
			maxDataSize = aranyagoconst.MaxMQTTDataSize
		}
	case config.AMQPOpts != nil:
		// 128 MB
		maxDataSize = 128 * 1024 * 1024
	case config.AzureIoTHubOpts != nil:
		maxDataSize = aranyagoconst.MaxAzureIoTHubC2DDataSize
	case config.GCPIoTCoreOpts != nil:
		maxDataSize = aranyagoconst.MaxGCPIoTCoreC2DDataSize
	case config.GRPCOpts != nil:
		maxDataSize = aranyagoconst.MaxGRPCDataSize
	default:
		exit()
		return nil, fmt.Errorf("unknown connectivity config")
	}

	maxDataSize -= aranyagopb.EmptyCmdSize
	if maxDataSize <= 0 {
		exit()
		return nil, fmt.Errorf("max data size too small, must be greater than %d", aranyagopb.EmptyCmdSize)
	}

	return &baseManager{
		ctx:    ctx,
		exit:   exit,
		config: config,

		sessions: NewSessionManager(
			config.AddTimedSession, config.DelTimedSession,
			config.GetTimedSessions, config.AddTimedStreamCreation,
		),

		maxDataSize:      maxDataSize,
		log:              log.Log.WithName(fmt.Sprintf("conn.%s", name)),
		sendCmd:          nil,
		globalMsgChan:    make(chan *aranyagopb.Msg, 1),
		connectedDevices: sets.NewString(),

		connected:       make(chan struct{}),
		disconnected:    disconnected,
		rejected:        make(chan struct{}),
		alreadyRejected: false,

		_stopped: 0,
		_working: 0,
	}, nil
}

func (m *baseManager) stopped() bool {
	return atomic.LoadUint32(&m._stopped) == 1
}

func (m *baseManager) MaxPayloadSize() int {
	return m.maxDataSize
}

func (m *baseManager) GlobalMessages() <-chan *aranyagopb.Msg {
	// global message channel is never recreated, so no lock needed
	return m.globalMsgChan
}

func (m *baseManager) PostStreamCmd(
	kind aranyagopb.CmdType,
	payloadCmd proto.Marshaler,
	dataOut, errOut io.Writer,
) (
	msgCh <-chan *aranyagopb.Msg,
	streamReady <-chan struct{},
	realSID uint64,
	err error,
) {
	if m.stopped() {
		err = ErrManagerClosed
		return
	}

	var payload []byte
	switch kind {
	case aranyagopb.CMD_METRICS_COLLECT:
		// metrics stream
	case aranyagopb.CMD_EXEC, aranyagopb.CMD_ATTACH,
		aranyagopb.CMD_LOGS, aranyagopb.CMD_PORT_FORWARD,
		// runtime exec/attach/logs/port-forward
		aranyagopb.CMD_RUNTIME:
		if payloadCmd == nil {
			err = fmt.Errorf("invalid empty payload for stream operation")
			return
		}

		payload, err = payloadCmd.Marshal()
		if err != nil {
			err = fmt.Errorf("failed to marshal payload: %w", err)
			return
		}
	default:
		err = fmt.Errorf("invalid non stream cmd %q", kind.String())
		return
	}

	realSID, msgCh = m.sessions.Add(0, true)
	defer func() {
		if err != nil {
			m.sessions.Delete(realSID)
		}
	}()

	streamReady, ok := m.sessions.SetStream(realSID, dataOut, errOut)
	if !ok {
		err = fmt.Errorf("failed to set session to stream mode")
		return
	}

	_, err = m.doPostData(kind, realSID, 0, true, payload)
	return
}

func (m *baseManager) doExclusive(f func()) {
	for !atomic.CompareAndSwapUint32(&m._working, 0, 1) {
		runtime.Gosched()
	}

	f()

	atomic.StoreUint32(&m._working, 0)
}

// Connected notify when agent connected
func (m *baseManager) Connected() <-chan struct{} {
	var ret <-chan struct{}
	m.doExclusive(func() {
		ret = m.connected
	})

	return ret
}

// Disconnected notify when agent disconnected
func (m *baseManager) Disconnected() <-chan struct{} {
	var ret <-chan struct{}
	m.doExclusive(func() {
		ret = m.disconnected
	})

	return ret
}

// Rejected notify when agent get rejected
func (m *baseManager) Rejected() <-chan struct{} {
	var ret <-chan struct{}
	m.doExclusive(func() {
		ret = m.rejected
	})

	return ret
}

func (m *baseManager) onReject(reject func()) {
	m.doExclusive(func() {
		if m.alreadyRejected {
			return
		}

		m.alreadyRejected = true

		reject()

		close(m.rejected)
	})
}

func (m *baseManager) OnConnected(initialize func() (id string)) {
	id := initialize()
	if id == "" {
		// not initialized
		return
	}

	if m.stopped() {
		return
	}

	m.doExclusive(func() {
		// skip signal connected when there is already device
		// connected (different device id)
		skipSignal := m.connectedDevices.Len() != 0

		m.connectedDevices.Insert(id)

		if skipSignal {
			return
		}

		select {
		case <-m.ctx.Done():
			return
		case <-m.connected:
			return
		default:
			// signal device connected
			close(m.connected)
		}

		// refresh device disconnected signal
		m.disconnected = make(chan struct{})
		m.rejected = make(chan struct{})
		m.alreadyRejected = false
	})
}

func (m *baseManager) onRecvMsg(data []byte) {
	msg := new(aranyagopb.Msg)
	err := msg.Unmarshal(data)
	if err != nil {
		m.log.I("failed to unmarshal msg", log.Binary("data", data), log.Error(err))
		return
	}

	if m.stopped() {
		return
	}

	if dispatched := m.sessions.Dispatch(msg); !dispatched {
		m.globalMsgChan <- msg
	}

	switch msg.Kind {
	case aranyagopb.MSG_ERROR:
		if msg.GetError().GetKind() == aranyagopb.ERR_TIMEOUT {
			// close session with best effort
			_, _, _ = m.PostCmd(0, aranyagopb.CMD_SESSION_CLOSE, &aranyagopb.SessionCloseCmd{Sid: msg.Sid})
		}
	default:
	}
}

// onDisconnected delete device connection related jobs
func (m *baseManager) OnDisconnected(finalize func() (id string, all bool)) {
	// release device connection, refresh device connection signal
	// and orphan message channel (not including stream sessions)
	m.doExclusive(func() {
		if m.connectedDevices.Len() == 0 {
			return
		}

		id, all := finalize()
		if !all {
			m.connectedDevices.Delete(id)
		} else {
			m.connectedDevices = sets.NewString()
		}

		if m.connectedDevices.Len() != 0 {
			return
		}

		select {
		case <-m.disconnected:
		default:
			// signal device disconnected
			close(m.disconnected)
		}

		m.sessions.Cleanup()
		// refresh connected signal
		m.connected = make(chan struct{})
	})
}

func (m *baseManager) doPostData(
	kind aranyagopb.CmdType,
	realSid, lastSeq uint64,
	complete bool,
	payload []byte,
) (_ uint64, err error) {
	chunkSize := m.MaxPayloadSize()
	for len(payload) > chunkSize {
		err = m.sendCmd(&aranyagopb.Cmd{
			Kind:      kind,
			Sid:       realSid,
			Seq:       lastSeq,
			Completed: false,
			Payload:   payload[:chunkSize],
		})
		if err != nil {
			return lastSeq, fmt.Errorf("failed to post cmd chunk: %w", err)
		}
		lastSeq++
		payload = payload[chunkSize:]
	}

	err = m.sendCmd(&aranyagopb.Cmd{
		Kind:      kind,
		Sid:       realSid,
		Seq:       lastSeq,
		Completed: complete,
		Payload:   payload,
	})
	if err != nil {
		return lastSeq, fmt.Errorf("failed to post cmd chunk: %w", err)
	}

	return lastSeq, err
}

func (m *baseManager) PostData(
	sid uint64,
	kind aranyagopb.CmdType,
	seq uint64,
	completed bool,
	payload []byte,
) (
	msgCh <-chan *aranyagopb.Msg,
	realSid, lastSeq uint64,
	err error,
) {
	defer func() {
		if m.log.Enabled(log.LevelVerbose) {
			m.log.V("remaining sessions",
				log.Any("all", m.sessions.Remains()),
				log.Any("timed", m.sessions.TimedRemains()),
			)
		}
	}()

	lastSeq = seq

	if m.stopped() {
		err = ErrManagerClosed
		return
	}

	recordSession := true
	// session id should not be empty if it's a input or resize command
	switch kind {
	case aranyagopb.CMD_SESSION_CLOSE:
		recordSession = false
		realSid = sid

		m.sessions.Delete(realSid)
	case aranyagopb.CMD_EXEC, aranyagopb.CMD_ATTACH,
		aranyagopb.CMD_LOGS, aranyagopb.CMD_PORT_FORWARD,
		aranyagopb.CMD_METRICS_COLLECT:
		// do not handle stream commands here
		err = fmt.Errorf("use PostStreamCmd to start stream session")
		return
	case aranyagopb.CMD_TTY_RESIZE, aranyagopb.CMD_DATA_UPSTREAM:
		// only allowed in existing sessions
		if sid == 0 {
			// session must present, but got empty id
			err = fmt.Errorf("invalid zero sid")
			return
		}

		// avoid unexpected session creation when data sent after
		// real session closed
		recordSession = false
		realSid = sid
	}

	if recordSession {
		realSid, msgCh = m.sessions.Add(sid, false)
		defer func() {
			if err != nil {
				m.sessions.Delete(realSid)
			}
		}()
	}

	lastSeq, err = m.doPostData(kind, realSid, lastSeq, completed, payload)
	return
}

func (m *baseManager) PostCmd(
	sid uint64,
	kind aranyagopb.CmdType,
	payloadCmd proto.Marshaler,
) (
	msgCh <-chan *aranyagopb.Msg,
	realSID uint64,
	err error,
) {
	var payload []byte
	if payloadCmd != nil {
		payload, err = payloadCmd.Marshal()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal payload cmd: %w", err)
		}
	}

	msgCh, realSID, _, err = m.PostData(sid, kind, 0, true, payload)
	return
}

func (m *baseManager) onClose(closeManager func()) {
	if m.stopped() {
		return
	}

	m.doExclusive(func() {
		m.exit()
		closeManager()
		atomic.StoreUint32(&m._stopped, 1)
	})
}
