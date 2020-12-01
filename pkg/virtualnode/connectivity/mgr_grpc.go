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
	"fmt"
	"net"

	"arhat.dev/aranya-proto/aranyagopb"
	"arhat.dev/aranya-proto/aranyagopb/rpcpb"
	"arhat.dev/pkg/log"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"arhat.dev/aranya/pkg/constant"
)

var _ Manager = &GRPCManager{}

type GRPCManager struct {
	*baseManager

	clientSessions map[string]rpcpb.EdgeDevice_SyncServer

	server   *grpc.Server
	listener net.Listener
}

func NewGRPCManager(parentCtx context.Context, name string, mgrConfig *Options) (*GRPCManager, error) {
	mgr := &GRPCManager{
		baseManager: nil,

		clientSessions: make(map[string]rpcpb.EdgeDevice_SyncServer),

		listener: mgrConfig.GRPCOpts.Listener,
		server:   mgrConfig.GRPCOpts.Server,
	}
	var err error
	mgr.baseManager, err = newBaseManager(parentCtx, name, mgrConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create base manager: %w", err)
	}

	rpcpb.RegisterEdgeDeviceServer(mgrConfig.GRPCOpts.Server, mgr)

	mgr.sendCmd = func(cmd *aranyagopb.Cmd) error {
		mgr.mu.RLock()
		defer mgr.mu.RUnlock()

		var err error
		for _, s := range mgr.clientSessions {
			err = multierr.Combine(err, s.Send(cmd))
		}

		return err
	}

	return mgr, nil
}

func (m *GRPCManager) Start() error {
	return m.server.Serve(m.listener)
}

func (m *GRPCManager) Close() {
	m.onClose(func() {
		m.server.Stop()
		_ = m.listener.Close()
	})
}

func (m *GRPCManager) Reject(reason aranyagopb.RejectionReason, message string) {
	m.onReject(func() {
		// best effort
		for _, syncSrv := range m.clientSessions {
			data, _ := (&aranyagopb.RejectCmd{
				Reason:  reason,
				Message: message,
			}).Marshal()

			_ = syncSrv.Send(&aranyagopb.Cmd{
				Kind:      aranyagopb.CMD_REJECT,
				Sid:       0,
				Seq:       0,
				Completed: true,
				Payload:   data,
			})
		}
	})
}

func (m *GRPCManager) Sync(server rpcpb.EdgeDevice_SyncServer) error {
	connCtx, closeConn := context.WithCancel(server.Context())
	defer closeConn()

	allMsgCh := make(chan *aranyagopb.Msg, constant.DefaultConnectivityMsgChannelSize)
	go func() {
		for {
			msg, err := server.Recv()

			if err != nil {
				close(allMsgCh)

				s, _ := status.FromError(err)
				switch s.Code() {
				case codes.Canceled, codes.OK:
				default:
					m.log.I("stream recv failed", log.Error(s.Err()))
				}

				return
			}

			allMsgCh <- msg
		}
	}()

	alreadyOnline := false
	onlineID := ""

	defer m.OnDisconnected(func() (id string, all bool) {
		closeConn()

		if alreadyOnline {
			delete(m.clientSessions, onlineID)
		}

		return onlineID, false
	})

	for {
		select {
		case <-m.rejected:
			// device rejected, return to close this stream
			return nil
		case <-connCtx.Done():
			return nil
		case msg, more := <-allMsgCh:
			if !more {
				return nil
			}

			if msg.Kind == aranyagopb.MSG_STATE {
				s := msg.GetState()
				if s == nil {
					m.Reject(aranyagopb.REJECTION_INVALID_PROTO, "invalid protocol")
					return nil
				}

				switch s.Kind {
				case aranyagopb.STATE_ONLINE:
					if alreadyOnline {
						// online message MUST not be received more than once
						m.Reject(aranyagopb.REJECTION_ALREADY_CONNECTED, "client has sent online message once")
					}

					onlineID = s.DeviceId
					if _, ok := m.clientSessions[onlineID]; ok {
						m.Reject(aranyagopb.REJECTION_ALREADY_CONNECTED, "client already connected with same online-id")
					}

					alreadyOnline = true

					m.mu.Lock()
					m.clientSessions[onlineID] = server
					m.mu.Unlock()
				case aranyagopb.STATE_OFFLINE:
					// received offline message
					return nil
				}
			}

			if !alreadyOnline {
				// discard messages until online message received
				continue
			}

			go m.onRecvMsg(msg)
		}
	}
}
