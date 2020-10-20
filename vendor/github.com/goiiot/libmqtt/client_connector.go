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
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

type Connector func(
	ctx context.Context,
	address string,
	timeout time.Duration,
	tlsConfig *tls.Config,
) (net.Conn, error)

func WithTCPConnector(handshakeTimeout time.Duration) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.newConnection = func(
			ctx context.Context,
			address string,
			timeout time.Duration,
			tlsConfig *tls.Config,
		) (conn net.Conn, e error) {
			return tcpConnect(ctx, address, timeout, handshakeTimeout, tlsConfig)
		}
		return nil
	}
}

func WithWebSocketConnector(handshakeTimeout time.Duration, headers http.Header) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.newConnection = func(
			ctx context.Context,
			address string,
			timeout time.Duration,
			tlsConfig *tls.Config,
		) (conn net.Conn, e error) {
			return websocketConnect(ctx, address, timeout, handshakeTimeout, headers, tlsConfig)
		}
		return nil
	}
}

func WithCustomConnector(connector Connector) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.newConnection = connector
		return nil
	}
}

type tlsTimeoutError struct{}

func (tlsTimeoutError) Error() string   { return "tls: timed out" }
func (tlsTimeoutError) Timeout() bool   { return true }
func (tlsTimeoutError) Temporary() bool { return true }

func tcpConnect(
	ctx context.Context,
	address string,
	timeout, handshakeTimeout time.Duration,
	tlsConfig *tls.Config,
) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		// code copied from golang standard library tls.DialWithDialer
		errChannel := make(chan error, 2)

		if handshakeTimeout != 0 {
			time.AfterFunc(handshakeTimeout, func() {
				errChannel <- tlsTimeoutError{}
			})
		}

		colonPos := strings.LastIndex(address, ":")
		if colonPos == -1 {
			colonPos = len(address)
		}
		hostname := address[:colonPos]

		// If no ServerName is set, infer the ServerName
		// from the hostname we're connecting to.
		if tlsConfig.ServerName == "" {
			// Make a copy to avoid polluting argument or default.
			c := tlsConfig.Clone()
			c.ServerName = hostname
			tlsConfig = c
		}

		tlsConn := tls.Client(conn, tlsConfig)
		if timeout == 0 {
			err = tlsConn.Handshake()
		} else {
			go func() {
				errChannel <- tlsConn.Handshake()
			}()

			err = <-errChannel
		}

		if err != nil {
			_ = conn.Close()
			return nil, err
		}

		conn = tlsConn
	}

	return conn, err
}

var errNotSupported = errors.New("operation not supported")

type wsConn struct {
	ctx  context.Context
	conn *websocket.Conn

	localAddr  net.Addr
	remoteAddr net.Addr

	readBuf *bytes.Buffer
}

func (c *wsConn) Read(b []byte) (int, error) {
	_, data, err := c.conn.Read(c.ctx)
	if err != nil {
		return 0, err
	}

	c.readBuf.Write(data)
	return c.readBuf.Read(b)
}

func (c *wsConn) Write(b []byte) (int, error) {
	err := c.conn.Write(c.ctx, websocket.MessageBinary, b)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *wsConn) Close() error {
	return c.conn.Close(websocket.StatusGoingAway, "")
}

func (c *wsConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *wsConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *wsConn) SetDeadline(t time.Time) error {
	return errNotSupported
}

func (c *wsConn) SetReadDeadline(t time.Time) error {
	return errNotSupported
}

func (c *wsConn) SetWriteDeadline(t time.Time) error {
	return errNotSupported
}

// nolint:deadcode,unused
func quicConnector(address string, timeout, handshakeTimeout time.Duration, tlsConfig *tls.Config) (net.Conn, error) {
	// TODO: TBD
	return nil, nil
}
