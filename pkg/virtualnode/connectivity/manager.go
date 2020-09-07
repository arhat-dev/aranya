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
	"net"
	"sync"
	"time"

	"arhat.dev/pkg/log"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"

	"arhat.dev/aranya-proto/gopb"
	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

var (
	ErrDeviceNotConnected = errors.New("device is not connected")
	ErrManagerClosed      = errors.New("connectivity manager has been closed")
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

type Options struct {
	UnarySessionTimeout time.Duration

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
	Reject(reason gopb.RejectCmd_Reason, message string)

	// Connected signal
	Connected() <-chan struct{}

	// Disconnected signal
	Disconnected() <-chan struct{}

	// GlobalMessages message with no session attached
	GlobalMessages() <-chan *gopb.Msg

	// PostCmd send a command to remote device with timeout
	// return a channel for messages to be received in the session
	PostCmd(sid uint64, cmd proto.Marshaler) (msgCh <-chan interface{}, realSID uint64, err error)

	// MaxDataSize of this kind connectivity method, used to reduce message overhead
	// when handling date streams for port-forward and command execution
	MaxDataSize() int

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

	maxDataSize      int
	log              log.Interface
	sendCmd          func(*gopb.Cmd) error
	globalMsgChan    chan *gopb.Msg
	connectedDevices map[string]struct{}

	// signals
	connected       chan struct{}
	disconnected    chan struct{}
	rejected        chan struct{}
	alreadyRejected bool

	// status
	stopped bool // we do not use atomic here since all related operation needs to be locked
	mu      *sync.RWMutex
}

func newBaseManager(parentCtx context.Context, name string, config *Options) *baseManager {
	ctx, exit := context.WithCancel(parentCtx)
	disconnected := make(chan struct{})
	close(disconnected)

	sessions := NewSessionManager()
	err := sessions.Start(ctx.Done())
	if err != nil {
		panic(err)
	}

	var maxDataSize int
	switch {
	case config.MQTTOpts != nil, config.AMQPOpts != nil:
		maxDataSize = constant.MaxMQTTDataSize
	case config.AzureIoTHubOpts != nil:
		maxDataSize = constant.MaxAzureIoTHubC2DDataSize
	case config.GCPIoTCoreOpts != nil:
		maxDataSize = constant.MaxGCPIoTCoreC2DDataSize
	case config.GRPCOpts != nil:
		fallthrough
	default:
		maxDataSize = constant.MaxGRPCDataSize
	}

	return &baseManager{
		ctx:      ctx,
		exit:     exit,
		config:   config,
		sessions: sessions,

		maxDataSize:      maxDataSize - gopb.EmptyInputCmdSize,
		log:              log.Log.WithName(fmt.Sprintf("conn.%s", name)),
		sendCmd:          nil,
		globalMsgChan:    make(chan *gopb.Msg, 1),
		connectedDevices: make(map[string]struct{}),

		connected:       make(chan struct{}),
		disconnected:    disconnected,
		rejected:        make(chan struct{}),
		alreadyRejected: false,

		stopped: false,
		mu:      new(sync.RWMutex),
	}
}

func (m *baseManager) MaxDataSize() int {
	return m.maxDataSize
}

func (m *baseManager) GlobalMessages() <-chan *gopb.Msg {
	return m.globalMsgChan
}

// Connected
func (m *baseManager) Connected() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.connected
}

// Disconnected
func (m *baseManager) Disconnected() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disconnected
}

// Rejected
func (m *baseManager) Rejected() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.rejected
}

func (m *baseManager) onReject(reject func()) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.alreadyRejected {
		return
	}
	m.alreadyRejected = true

	reject()

	close(m.rejected)
}

func (m *baseManager) OnConnected(initialize func() (id string)) {
	id := initialize()
	if id == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return
	}

	skipSignal := true
	if len(m.connectedDevices) == 0 {
		skipSignal = false
	}

	m.connectedDevices[id] = struct{}{}

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
}

func (m *baseManager) onRecvMsg(msg *gopb.Msg) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped {
		return
	}

	if dispatched := m.sessions.Dispatch(msg); !dispatched {
		m.globalMsgChan <- msg
	}

	// nolint:gocritic
	switch msg.Kind {
	case gopb.MSG_ERROR:
		if msg.GetError().GetKind() == gopb.ERR_TIMEOUT {
			// close session with best effort
			_, _, _ = m.PostCmd(0, gopb.NewSessionCloseCmd(msg.SessionId))
		}
	}
}

// onDisconnected delete device connection related jobs
func (m *baseManager) OnDisconnected(finalize func() (id string, all bool)) {
	// release device connection, refresh device connection signal
	// and orphaned message channel
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.connectedDevices) == 0 {
		return
	}

	id, all := finalize()
	if !all {
		delete(m.connectedDevices, id)
	} else {
		m.connectedDevices = make(map[string]struct{})
	}

	if len(m.connectedDevices) != 0 {
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
}

func (m *baseManager) PostCmd(sid uint64, cmd proto.Marshaler) (msgCh <-chan interface{}, realSID uint64, err error) {
	m.mu.RLock()
	defer func() {
		if m.log.Enabled(log.LevelVerbose) {
			m.log.V("remaining sessions",
				log.Any("all", m.sessions.Remains()),
				log.Any("timeout", m.sessions.TimeoutRemains()),
			)
		}
		m.mu.RUnlock()
	}()

	if m.stopped {
		return nil, 0, ErrManagerClosed
	}

	var (
		sessionMustExist bool
		expectSeqData    bool
		recordSession    = true
		timeout          = m.config.UnarySessionTimeout
	)

	// session id should not be empty if it's a input or resize command
	switch c := cmd.(type) {
	case *gopb.SessionCmd:
		recordSession = false
		// nolint:gocritic
		switch c.Action {
		case gopb.CLOSE_SESSION:
			defer m.sessions.Delete(sid)
		}
	case *gopb.PodOperationCmd:
		switch c.Action {
		case gopb.PORT_FORWARD_TO_CONTAINER,
			gopb.EXEC_IN_CONTAINER,
			gopb.ATTACH_TO_CONTAINER,
			gopb.RETRIEVE_CONTAINER_LOG:
			// we don't control stream session timeout here
			// it is controlled by pod manager
			expectSeqData = true
			timeout = 0
		case gopb.RESIZE_CONTAINER_TTY,
			gopb.WRITE_TO_CONTAINER:
			timeout = 0
			sessionMustExist = true

			if sid == 0 {
				// session must present, but got empty
				return nil, 0, fmt.Errorf("invalid zero session id")
			}
		}
	}

	if recordSession {
		var realSid uint64
		realSid, msgCh = m.sessions.Add(sid, cmd, timeout, expectSeqData)
		defer func() {
			if err != nil {
				m.sessions.Delete(sid)
			}
		}()

		if sessionMustExist && sid != realSid {
			return nil, 0, fmt.Errorf("existing session id not match")
		}

		sid = realSid
	}

	if err = m.sendCmd(gopb.NewCmd(sid, cmd)); err != nil {
		return nil, 0, err
	}

	return msgCh, realSID, nil
}

func (m *baseManager) onClose(closeManager func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return
	}

	m.exit()
	closeManager()
	m.stopped = true
}
