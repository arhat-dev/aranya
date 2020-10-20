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
	"context"
	"crypto/tls"
	"strings"
	"sync"
	"sync/atomic"
)

// Client type for *AsyncClient
type Client = *AsyncClient

// NewClient create a new mqtt client
func NewClient(options ...Option) (Client, error) {
	c := defaultClient()

	for _, setOption := range options {
		err := setOption(c, &c.options)
		if err != nil {
			return nil, err
		}
	}

	c.addWorker(c.handleTopicMsg, c.handleMsg)

	return c, nil
}

// AsyncClient is the async mqtt client implementation
type AsyncClient struct {
	// Deprecated: use ConnectServer instead (will be removed in v1.0)
	servers []string
	// Deprecated: use ConnectServer instead (will be removed in v1.0)
	secureServers []string

	options          connectOptions      // client wide connection options
	msgCh            chan *message       // error channel
	sendCh           chan Packet         // pub channel for sending publish packet to server
	recvCh           chan *PublishPacket // recv channel for server pub receiving
	idGen            *idGenerator        // Packet id generator
	router           TopicRouter         // Topic router
	persist          PersistMethod       // Persist method
	connectedServers *sync.Map
	workers          *sync.WaitGroup // Workers (goroutines)
	log              *logger         // client logger

	// success/error handlers
	pubHandler     PubHandleFunc
	subHandler     SubHandleFunc
	unsubHandler   UnsubHandleFunc
	netHandler     NetHandleFunc
	persistHandler PersistHandleFunc

	ctx     context.Context    // closure of this channel will signal all client worker to stop
	exit    context.CancelFunc // called when client exit
	stopSig <-chan struct{}
}

// create a client with default options
func defaultClient() *AsyncClient {
	ctx, exitFunc := context.WithCancel(context.Background())

	return &AsyncClient{
		servers:       make([]string, 0, 1),
		secureServers: make([]string, 0, 1),

		options: defaultConnectOptions(),
		msgCh:   make(chan *message, 10),
		sendCh:  make(chan Packet, 1),
		recvCh:  make(chan *PublishPacket, 1),
		router:  NewTextRouter(),
		idGen:   newIDGenerator(),
		persist: NonePersist,

		connectedServers: new(sync.Map),
		workers:          new(sync.WaitGroup),

		ctx:     ctx,
		exit:    exitFunc,
		stopSig: ctx.Done(),
	}
}

// Handle register subscription message route
//
// Deprecated: use HandleTopic instead, will be removed in v1.0
func (c *AsyncClient) Handle(topic string, h TopicHandler) {
	if h != nil {
		c.log.v("CLI registered topic handler, topic =", topic)
		c.router.Handle(topic, func(client Client, topic string, qos QosLevel, msg []byte) {
			h(topic, qos, msg)
		})
	}
}

// HandleTopic add a topic routing rule
func (c *AsyncClient) HandleTopic(topic string, h TopicHandleFunc) {
	if h != nil {
		c.log.v("CLI registered topic handler, topic =", topic)
		c.router.Handle(topic, h)
	}
}

// Connect to all designated servers
//
// Deprecated: use Client.ConnectServer instead (will be removed in v1.0)
func (c *AsyncClient) Connect(h ConnHandler) {
	c.log.v("CLI connect to server, handler =", h)

	connHandler := func(client Client, server string, code byte, err error) {
		h(server, code, err)
	}

	for _, s := range c.servers {
		options := c.options.clone()
		options.connHandler = connHandler

		srv := s
		c.addWorker(func() { options.connect(c, srv, c.options.protoVersion, c.options.firstDelay) })
	}

	for _, s := range c.secureServers {
		secureOptions := c.options.clone()
		secureOptions.connHandler = connHandler
		secureOptions.tlsConfig = &tls.Config{
			ServerName: strings.SplitN(s, ":", 1)[0],
		}

		srv := s
		c.addWorker(func() { secureOptions.connect(c, srv, secureOptions.protoVersion, secureOptions.firstDelay) })
	}
}

// Publish message(s) to topic(s), one to one
func (c *AsyncClient) Publish(msg ...*PublishPacket) {
	if c.isClosing() {
		return
	}

	for _, m := range msg {
		if m == nil {
			continue
		}

		p := m
		if p.Qos > Qos2 {
			p.Qos = Qos2
		}

		if p.Qos != Qos0 {
			if p.PacketID == 0 {
				p.PacketID = c.idGen.next(p)
				if err := c.persist.Store(sendKey(p.PacketID), p); err != nil {
					notifyPersistMsg(c.msgCh, p, err)
				}
			}
		}

		select {
		case <-c.stopSig:
			return
		case c.sendCh <- p:
		}
	}
}

// Subscribe topic(s)
func (c *AsyncClient) Subscribe(topics ...*Topic) {
	if c.isClosing() {
		return
	}

	c.log.d("CLI subscribe, topic(s) =", topics)

	s := &SubscribePacket{Topics: topics}
	s.PacketID = c.idGen.next(s)

	select {
	case <-c.stopSig:
		return
	case c.sendCh <- s:
		return
	}
}

// UnSubscribe topic(s)
// Deprecated: use Unsubscribe instead, will be removed in v1.0
func (c *AsyncClient) UnSubscribe(topics ...string) {
	c.Unsubscribe(topics...)
}

// Unsubscribe topic(s)
func (c *AsyncClient) Unsubscribe(topics ...string) {
	if c.isClosing() {
		return
	}

	c.log.d("CLI unsubscribe topic(s) =", topics)

	u := &UnsubPacket{TopicNames: topics}
	u.PacketID = c.idGen.next(u)

	select {
	case <-c.stopSig:
		return
	case c.sendCh <- u:
	}
}

// Wait will wait for all connections to exit
func (c *AsyncClient) Wait() {
	if c.isClosing() {
		return
	}

	c.log.i("CLI wait for all workers")
	c.workers.Wait()
}

// Destroy will disconnect form all server
// If force is true, then close connection without sending a DisconnPacket
func (c *AsyncClient) Destroy(force bool) {
	c.log.d("CLI destroying client with force =", force)
	if force {
		c.exit()
	} else {
		c.connectedServers.Range(func(key, value interface{}) bool {
			c.Disconnect(key.(string), nil)
			return true
		})

		c.exit()
	}
}

// Disconnect from one server
// return true if DisconnPacket will be sent
func (c *AsyncClient) Disconnect(server string, packet *DisconnPacket) bool {
	if packet == nil {
		packet = &DisconnPacket{}
	}

	if val, ok := c.connectedServers.Load(server); ok {
		conn := val.(*clientConn)
		atomic.StoreUint32(&conn.parentExit, 1)
		conn.send(packet)

		select {
		case <-conn.stopSig:
			// wait for conn to exit
		case <-c.stopSig:
			return false
		}

		return true
	}

	return false
}

func (c *AsyncClient) addWorker(workerFunc ...func()) {
	if c.isClosing() {
		return
	}

	for _, f := range workerFunc {
		c.workers.Add(1)
		go func(f func()) {
			defer c.workers.Done()
			f()
		}(f)
	}
}

func (c *AsyncClient) isClosing() bool {
	select {
	case <-c.stopSig:
		return true
	default:
		return false
	}
}

func (c *AsyncClient) handleTopicMsg() {
	for {
		select {
		case <-c.stopSig:
			return
		case pkt, more := <-c.recvCh:
			if !more {
				return
			}

			c.addWorker(func() { c.router.Dispatch(c, pkt) })
		}
	}
}
