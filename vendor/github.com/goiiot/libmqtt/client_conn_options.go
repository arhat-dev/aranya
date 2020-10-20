/*
 * Copyright Go-IIoT (https://github.com/goiiot)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package libmqtt

import (
	"bufio"
	"context"
	"crypto/tls"
	"math"
	"net"
	"time"
)

// ConnectServer connect to server with connection specific options
// only return errors happened when applying options
func (c *AsyncClient) ConnectServer(server string, connOptions ...Option) error {
	options := c.options.clone()

	for _, setOption := range connOptions {
		if err := setOption(c, &options); err != nil {
			return err
		}
	}

	c.addWorker(func() { options.connect(c, server, options.protoVersion, options.firstDelay) })

	return nil
}

func defaultConnectOptions() connectOptions {
	return connectOptions{
		protoVersion:    V311,
		protoCompromise: false,

		maxDelay:        2 * time.Minute,
		firstDelay:      5 * time.Second,
		backOffFactor:   1.5,
		dialTimeout:     20 * time.Second,
		keepalive:       2 * time.Minute,
		keepaliveFactor: 1.5,
		connPacket:      &ConnPacket{},

		newConnection: func(
			ctx context.Context,
			address string,
			timeout time.Duration,
			tlsConfig *tls.Config,
		) (conn net.Conn, e error) {
			return tcpConnect(ctx, address, timeout, 0, tlsConfig)
		},
	}
}

// connect options when connecting server (for conn packet)
// nolint:maligned
type connectOptions struct {
	connHandler     ConnHandleFunc
	dialTimeout     time.Duration
	protoVersion    ProtoVersion
	protoCompromise bool

	tlsConfig     *tls.Config // tls config with client side cert
	maxDelay      time.Duration
	firstDelay    time.Duration
	backOffFactor float64
	autoReconnect bool

	connPacket      *ConnPacket
	keepalive       time.Duration // used by ConnPacket (time in second)
	keepaliveFactor float64       // used for reasonable amount time to close conn if no ping resp

	newConnection Connector
}

// nolint:gocyclo
func (c connectOptions) connect(
	parent *AsyncClient,
	server string,
	version ProtoVersion,
	reconnectDelay time.Duration,
) {
	var (
		conn net.Conn
		err  error
	)

	parent.log.v("NET connectOptions.connect()")
	defer parent.connectedServers.Delete(server)

	conn, err = c.newConnection(parent.ctx, server, c.dialTimeout, c.tlsConfig)
	if err != nil {
		parent.log.e("CLI connect server failed, err =", err, ", server =", server)
		if c.connHandler != nil {
			parent.addWorker(func() { c.connHandler(parent, server, math.MaxUint8, err) })
		}

		if c.autoReconnect && !parent.isClosing() {
			goto reconnect
		}

		return
	}

	defer func() { _ = conn.Close() }()

	{
		if parent.isClosing() {
			return
		}

		connImpl := &clientConn{
			protoVersion: version,
			parent:       parent,
			name:         server,
			conn:         conn,
			connRW:       bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
			keepaliveC:   make(chan struct{}, 1),
			logicSendC:   make(chan Packet, 10),
			netRecvC:     make(chan Packet, 10),
		}

		parent.connectedServers.Store(server, connImpl)

		connImpl.ctx, connImpl.exit = context.WithCancel(parent.ctx)
		connImpl.stopSig = connImpl.ctx.Done()

		parent.addWorker(connImpl.handleNetRecv)

		connPkt := c.connPacket.clone()
		connPkt.ProtoVersion = version
		if err := connImpl.sendRaw(connPkt); err != nil {
			if c.autoReconnect && !parent.isClosing() {
				goto reconnect
			}
			return
		}

		select {
		case pkt, more := <-connImpl.netRecvC:
			if !more {
				if c.connHandler != nil {
					parent.addWorker(func() { c.connHandler(parent, server, math.MaxUint8, ErrDecodeBadPacket) })
				}
				close(connImpl.logicSendC)

				if c.autoReconnect && !parent.isClosing() {
					goto reconnect
				}
				return
			}

			switch p := pkt.(type) {
			case *ConnAckPacket:
				if p.Code != CodeSuccess {
					close(connImpl.logicSendC)

					if version > V311 && c.protoCompromise && p.Code == CodeUnsupportedProtoVersion {
						parent.addWorker(func() { c.connect(parent, server, version-1, reconnectDelay) })
						return
					}

					if c.connHandler != nil {
						parent.addWorker(func() { c.connHandler(parent, server, p.Code, nil) })
					}

					if c.autoReconnect && !parent.isClosing() {
						goto reconnect
					}
					return
				}
			default:
				close(connImpl.logicSendC)
				if c.connHandler != nil {
					parent.addWorker(func() { c.connHandler(parent, server, math.MaxUint8, ErrDecodeBadPacket) })
				}

				if c.autoReconnect && !parent.isClosing() {
					goto reconnect
				}
				return
			}
		case <-connImpl.stopSig:
			if c.autoReconnect && !parent.isClosing() {
				goto reconnect
			}
			return
		}

		parent.log.i("CLI connected to server =", server)
		if c.connHandler != nil {
			parent.addWorker(func() { c.connHandler(parent, server, CodeSuccess, nil) })
		}

		parent.addWorker(connImpl.handleSend)

		// start mqtt logic
		connImpl.logic()

		if parent.isClosing() || connImpl.parentExiting() {
			return
		}
	}

reconnect:

	reconnectTimer := time.NewTimer(reconnectDelay)
	defer reconnectTimer.Stop()

	select {
	case <-reconnectTimer.C:
		parent.log.e("CLI reconnecting to server =", server, "delay =", reconnectDelay)
		reconnectDelay = time.Duration(float64(reconnectDelay) * c.backOffFactor)
		if reconnectDelay > c.maxDelay {
			reconnectDelay = c.maxDelay
		}

		parent.addWorker(func() { c.connect(parent, server, version, reconnectDelay) })
	case <-parent.stopSig:
		return
	}
}

func (c connectOptions) clone() connectOptions {
	var tlsConfig *tls.Config
	if c.tlsConfig != nil {
		tlsConfig = c.tlsConfig.Clone()
	}
	return connectOptions{
		connHandler:     c.connHandler,
		dialTimeout:     c.dialTimeout,
		protoVersion:    c.protoVersion,
		protoCompromise: c.protoCompromise,
		tlsConfig:       tlsConfig,
		maxDelay:        c.maxDelay,
		firstDelay:      c.firstDelay,
		backOffFactor:   c.backOffFactor,
		autoReconnect:   c.autoReconnect,
		connPacket:      c.connPacket,
		keepalive:       c.keepalive,
		keepaliveFactor: c.keepaliveFactor,
		newConnection:   c.newConnection,
	}
}
