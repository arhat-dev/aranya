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
	"sync"

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

	clientSessions *sync.Map

	server   *grpc.Server
	listener net.Listener
}

func NewGRPCManager(parentCtx context.Context, name string, mgrConfig *Options) (*GRPCManager, error) {
	mgr := &GRPCManager{
		baseManager: nil,

		clientSessions: new(sync.Map),

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
		var err error
		mgr.clientSessions.Range(func(key, value interface{}) bool {
			err = multierr.Append(err, value.(rpcpb.EdgeDevice_SyncServer).Send(cmd))
			return true
		})

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
		m.clientSessions.Range(func(key, value interface{}) bool {
			data, _ := (&aranyagopb.RejectCmd{
				Reason:  reason,
				Message: message,
			}).Marshal()

			_ = value.(rpcpb.EdgeDevice_SyncServer).Send(&aranyagopb.Cmd{
				Kind:      aranyagopb.CMD_REJECT,
				Sid:       0,
				Seq:       0,
				Completed: true,
				Payload:   data,
			})

			return true
		})
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
			m.clientSessions.Delete(onlineID)
		}

		return onlineID, false
	})

	rejected := m.Rejected()

	for {
		select {
		case <-rejected:
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
						continue
					}

					// check online id
					onlineID = s.DeviceId
					_, loaded := m.clientSessions.LoadOrStore(onlineID, server)
					if loaded {
						m.Reject(aranyagopb.REJECTION_ALREADY_CONNECTED, "client already connected with same online-id")
						continue
					}

					alreadyOnline = true
				case aranyagopb.STATE_OFFLINE:
					// received offline message, exit loop
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
