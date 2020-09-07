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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"
)

var (
	ErrNotSupportedVersion = errors.New("mqtt version not supported ")
)

// Option is client option for connection options
type Option func(*AsyncClient, *connectOptions) error

// WithSecureServer use server certificate for verification
// won't apply `WithTLS`, `WithCustomTLS`, `WithTLSReader` options
// when connecting to these servers
//
// Deprecated: use Client.ConnectServer instead (will be removed in v1.0)
func WithSecureServer(servers ...string) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if c == nil {
			return errors.New("global option can not be applied to ConnectServer")
		}

		c.secureServers = append(c.secureServers, servers...)
		return nil
	}
}

// WithServer set client servers
// addresses should be in form of `ip:port` or `domain.name:port`,
// only TCP connection supported for now
//
// Deprecated: use Client.ConnectServer instead (will be removed in v1.0)
func WithServer(servers ...string) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		c.servers = append(c.servers, servers...)
		return nil
	}
}

// WithBuf is the alias of WithBufSize
//
// Deprecated: use WithBufSize instead (will be removed in v1.0)
var WithBuf = WithBufSize

// WithBufSize designate the channel size of send and recv
func WithBufSize(sendBufSize, recvBufSize int) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if sendBufSize < 1 {
			sendBufSize = 1
		}

		if recvBufSize < 1 {
			recvBufSize = 1
		}

		c.sendCh = make(chan Packet, sendBufSize)
		c.recvCh = make(chan *PublishPacket, recvBufSize)

		return nil
	}
}

func WithConnHandleFunc(handler ConnHandleFunc) Option {
	return func(client *AsyncClient, options *connectOptions) error {
		options.connHandler = handler
		return nil
	}
}

func WithPubHandleFunc(handler PubHandleFunc) Option {
	return func(client *AsyncClient, options *connectOptions) error {
		if client.pubHandler == nil {
			client.pubHandler = handler
		}
		return nil
	}
}

func WithSubHandleFunc(handler SubHandleFunc) Option {
	return func(client *AsyncClient, options *connectOptions) error {
		if client.subHandler == nil {
			client.subHandler = handler
		}
		return nil
	}
}

func WithUnsubHandleFunc(handler UnsubHandleFunc) Option {
	return func(client *AsyncClient, options *connectOptions) error {
		if client.unsubHandler == nil {
			client.unsubHandler = handler
		}
		return nil
	}
}

func WithNetHandleFunc(handler NetHandleFunc) Option {
	return func(client *AsyncClient, options *connectOptions) error {
		if client.netHandler == nil {
			client.netHandler = handler
		}
		return nil
	}
}

func WithPersistHandleFunc(handler PersistHandleFunc) Option {
	return func(client *AsyncClient, options *connectOptions) error {
		if client.persistHandler == nil {
			client.persistHandler = handler
		}
		return nil
	}
}

// WithLog will create basic logger for the client
func WithLog(l LogLevel) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		c.log = newLogger(l)
		return nil
	}
}

// WithPersist defines the persist method to be used
func WithPersist(method PersistMethod) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if method != nil {
			c.persist = method
		}
		return nil
	}
}

// WithCleanSession will set clean flag in connect packet
func WithCleanSession(f bool) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.connPacket.CleanSession = f
		return nil
	}
}

// WithIdentity for username and password
func WithIdentity(username, password string) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.connPacket.Username = username
		options.connPacket.Password = password
		return nil
	}
}

// WithKeepalive set the keepalive interval (time in second)
func WithKeepalive(keepalive uint16, factor float64) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if keepalive <= 0 {
			return fmt.Errorf("keepalive provided must be greater than 0")
		}

		options.connPacket.Keepalive = keepalive
		options.keepalive = time.Duration(keepalive) * time.Second
		if factor > 1 {
			options.keepaliveFactor = factor
		} else {
			factor = 1.2
		}
		return nil
	}
}

// WithAutoReconnect set client to auto reconnect to server when connection failed
func WithAutoReconnect(autoReconnect bool) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.autoReconnect = autoReconnect
		return nil
	}
}

// WithBackoffStrategy will set reconnect backoff strategy
// firstDelay is the time to wait before retrying after the first failure
// maxDelay defines the upper bound of backoff delay
// factor is applied to the backoff after each retry.
//
// e.g. FirstDelay = 1s and Factor = 2
// 		then the SecondDelay is 2s, the ThirdDelay is 4s
func WithBackoffStrategy(firstDelay, maxDelay time.Duration, factor float64) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if firstDelay < time.Millisecond {
			firstDelay = time.Millisecond
		}

		if maxDelay < firstDelay {
			maxDelay = firstDelay
		}

		if factor < 1 {
			factor = 1
		}

		options.firstDelay = firstDelay
		options.maxDelay = maxDelay
		options.backOffFactor = factor
		return nil
	}
}

// WithClientID set the client id for connection
func WithClientID(clientID string) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.connPacket.ClientID = clientID
		return nil
	}
}

// WithWill mark this connection as a will teller
func WithWill(topic string, qos QosLevel, retain bool, payload []byte) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.connPacket.IsWill = true
		options.connPacket.WillTopic = topic
		options.connPacket.WillQos = qos
		options.connPacket.WillRetain = retain
		options.connPacket.WillMessage = payload
		return nil
	}
}

// WithTLSReader set tls from client cert, key, ca reader, apply to all servers
// listed in `WithServer` Option
func WithTLSReader(certReader, keyReader, caReader io.Reader, serverNameOverride string, skipVerify bool) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		b, err := ioutil.ReadAll(certReader)
		if err != nil {
			return err
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return err
		}

		// load cert-key pair
		certBytes, err := ioutil.ReadAll(certReader)
		if err != nil {
			return err
		}
		keyBytes, err := ioutil.ReadAll(keyReader)
		if err != nil {
			return err
		}
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return err
		}

		options.tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: skipVerify,
			ClientCAs:          cp,
			ServerName:         serverNameOverride,
		}

		return nil
	}
}

// WithTLS set client tls from cert, key and ca file, apply to all servers
// listed in `WithServer` Option
func WithTLS(certFile, keyFile, caCert, serverNameOverride string, skipVerify bool) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		b, err := ioutil.ReadFile(caCert)
		if err != nil {
			return err
		}

		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(b) {
			return err
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return err
		}

		options.tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: skipVerify,
			ClientCAs:          cp,
			ServerName:         serverNameOverride,
		}
		return nil
	}
}

// WithCustomTLS replaces the TLS options with a custom tls.Config
func WithCustomTLS(config *tls.Config) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if config == nil {
			options.tlsConfig = nil
		} else {
			options.tlsConfig = config.Clone()
		}
		return nil
	}
}

// WithDialTimeout for connection time out (time in second)
func WithDialTimeout(timeout uint16) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.dialTimeout = time.Duration(timeout) * time.Second
		return nil
	}
}

// WithVersion defines the mqtt protocol ProtoVersion in use
func WithVersion(version ProtoVersion, compromise bool) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		switch version {
		case V311, V5:
			options.protoVersion = version
			options.protoCompromise = compromise
			return nil
		}

		return ErrNotSupportedVersion
	}
}

// WithRouter set the router for topic dispatch
func WithRouter(r TopicRouter) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		if r != nil {
			c.router = r
		}
		return nil
	}
}

func WithConnPacket(pkt ConnPacket) Option {
	return func(c *AsyncClient, options *connectOptions) error {
		options.connPacket = &pkt

		if pkt.Keepalive > 0 {
			options.keepalive = time.Duration(pkt.Keepalive) * time.Second
		}
		return nil
	}
}
