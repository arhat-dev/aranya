// +build !js

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
	"net"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

func websocketConnect(
	ctx context.Context,
	address string,
	dialTimeout, handShakeTimeout time.Duration,
	headers http.Header,
	tlsConfig *tls.Config,
) (net.Conn, error) {
	netDialer := &net.Dialer{
		Timeout: dialTimeout,
	}

	urlSchema := "ws"
	if tlsConfig != nil {
		urlSchema = "wss"
	}

	// nolint:bodyclose
	conn, _, err := websocket.Dial(ctx, urlSchema+"://"+address, &websocket.DialOptions{
		HTTPHeader:   headers,
		Subprotocols: []string{"mqtt"},
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				DialContext:         netDialer.DialContext,
				TLSHandshakeTimeout: handShakeTimeout,
				TLSClientConfig:     tlsConfig,
				ForceAttemptHTTP2:   true,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// best effort
	emptyAddr := new(net.TCPAddr)

	return &wsConn{
		ctx:     ctx,
		conn:    conn,
		readBuf: new(bytes.Buffer),

		localAddr:  emptyAddr,
		remoteAddr: emptyAddr,
	}, nil
}
