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
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/gopb"
	"arhat.dev/aranya-proto/gopb/protoconst"
	"arhat.dev/pkg/backoff"
	"arhat.dev/pkg/log"
	"github.com/streadway/amqp"
)

func newAMQPClient(parent *MessageQueueManager) (*amqpClient, error) {
	opts := parent.config.AMQPOpts

	host, portStr, err := net.SplitHostPort(opts.Config.Broker)
	if err != nil {
		return nil, err
	}

	var port int64
	if portStr != "" {
		port, err = strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			return nil, err
		}
	}

	uri := &amqp.URI{
		Scheme:   "amqp",
		Host:     host,
		Port:     int(port),
		Username: string(opts.Username),
		Password: string(opts.Password),
		Vhost:    opts.Config.VHost,
	}

	if opts.TLSConfig != nil {
		uri.Scheme = "amqps"
	}

	var (
		replyTo      string
		pubTopic     string
		subTopic     string
		subWillTopic string
	)

	pubTopic, subTopic, subWillTopic = protoconst.AMQPTopics(opts.Config.TopicNamespace)
	return &amqpClient{
		log:       parent.log,
		url:       uri.String(),
		tlsConfig: opts.TLSConfig,

		onRecvMsg: parent.onRecvMsg,
		rejectAgent: func() {
			parent.Reject(gopb.REJECTION_INTERNAL_SERVER_ERROR, "amqp connection lost")
		},

		sessionStore: new(atomic.Value),
		connBackoff:  backoff.NewStrategy(time.Second, 10*time.Second, 1.2, 0),
		exchange:     opts.Config.Exchange,

		replyTo:      replyTo,
		pubTopic:     pubTopic,
		subTopic:     subTopic,
		subWillTopic: subWillTopic,

		stopSig: parent.ctx.Done(),
	}, nil
}

type amqpSession struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func (s *amqpSession) close() error {
	_ = s.ch.Close()
	return s.conn.Close()
}

type amqpClient struct {
	log         log.Interface
	url         string
	tlsConfig   *tls.Config
	onRecvMsg   func(*gopb.Msg)
	rejectAgent func()

	sessionStore *atomic.Value
	connBackoff  *backoff.Strategy
	exchange     string

	replyTo      string
	pubTopic     string
	subTopic     string
	subWillTopic string

	disconnected int32
	stopSig      <-chan struct{}
}

func (c *amqpClient) getSession() *amqpSession {
	if s := c.sessionStore.Load(); s != nil {
		return s.(*amqpSession)
	}
	return nil
}

func (c *amqpClient) isDisconnected() bool {
	return atomic.LoadInt32(&c.disconnected) == 1
}

func (c *amqpClient) PubTopics() []string {
	return []string{c.pubTopic}
}

func (c *amqpClient) SubTopics() []string {
	return []string{c.subTopic, c.subWillTopic}
}

// Connect to amqp broker with connect packet
func (c *amqpClient) Connect() (err error) {
	var conn *amqp.Connection
	if c.tlsConfig != nil {
		conn, err = amqp.DialTLS(c.url, c.tlsConfig)
	} else {
		conn, err = amqp.Dial(c.url)
	}
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %v", err)
	}

	failedPub := ch.NotifyReturn(make(chan amqp.Return))
	go func() {
		for m := range failedPub {
			cmd := new(gopb.Cmd)
			_ = cmd.Unmarshal(m.Body)
			c.log.I("failed to publish cmd", log.Any("cmd", cmd))
		}
	}()

	err = ch.ExchangeDeclare(c.exchange, amqp.ExchangeTopic, false, true, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %v", err)
	}

	c.sessionStore.Store(&amqpSession{
		conn: conn,
		ch:   ch,
	})

	// start reconnect logic
	connCloseSig := conn.NotifyClose(make(chan *amqp.Error))
	go func() {
		select {
		case e, more := <-connCloseSig:
			if !more {
				// closed by normal operation, check if it's expected to exit
				if c.isDisconnected() {
					return
				}
				e = amqp.ErrClosed
			}

			c.log.I("lost connection to amqp broker, reconnecting", log.Error(e))

			var (
				err   = error(e)
				wait  = c.connBackoff.Next("conn")
				timer = time.NewTimer(wait)
			)
			defer timer.Stop()

			for err != nil {
				select {
				case <-timer.C:
					// backoff finished, reconnect
					err = c.Connect()
					if err != nil {
						c.log.I("failed to connect to amqp broker, retry", log.Error(err))

						wait = c.connBackoff.Next("conn")
						timer.Reset(wait)
						continue
					}

					// reset backoff, since it's meant for network dial
					c.connBackoff.Reset("conn")
					// reconnect succeed, subscribe to the topic
					if err := c.Subscribe(); err != nil {
						wait = c.connBackoff.Next("conn")
						timer.Reset(wait)
						continue
					}

					// reject client to cleanup session
					c.rejectAgent()

					return
				case <-c.stopSig:
					// connectivity manager exited
					return
				}
			}
		case <-c.stopSig:
			return
		}
	}()

	return nil
}

// Disconnect from the amqp broker
func (c *amqpClient) Disconnect() error {
	atomic.StoreInt32(&c.disconnected, 1)

	if s := c.getSession(); s != nil {
		return s.close()
	}

	return nil
}

// Subscribe to c.subTopic, the topic param is not used
func (c *amqpClient) Subscribe() error {

	msgCh, err := c.doSubscribe(c.subTopic)
	if err != nil {
		return err
	}

	willCh, err := c.doSubscribe(c.subWillTopic)
	if err != nil {
		return err
	}

	go func() {
		var recvDeliver amqp.Delivery

		for {
			select {
			case m, more := <-msgCh:
				if !more {
					return
				}
				recvDeliver = m
			case w, more := <-willCh:
				if !more {
					return
				}
				recvDeliver = w
			case <-c.stopSig:
				return
			}

			msg := new(gopb.Msg)
			err := msg.Unmarshal(recvDeliver.Body)
			if err != nil {
				c.log.I("failed to unmarshal msg bytes", log.Error(err), log.Binary("msgBytes", recvDeliver.Body))
				continue
			}

			go c.onRecvMsg(msg)
		}
	}()

	return nil
}

func (c *amqpClient) doSubscribe(topic string) (<-chan amqp.Delivery, error) {
	s := c.getSession()
	if s == nil {
		return nil, fmt.Errorf("no channel available to subscribe [%s]", topic)
	}

	q, err := s.ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue: %v", err)
	}

	err = s.ch.QueueBind(q.Name, topic, c.exchange, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to bind queue: %v", err)
	}

	msgCh, err := s.ch.Consume(q.Name, "aranya", true, true, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to consume messages: %v", err)
	}

	return msgCh, nil
}

// Publish
func (c *amqpClient) Publish(data []byte) error {
	s := c.getSession()
	if s == nil {
		return fmt.Errorf("no channel available to publish [%s]", c.pubTopic)
	}

	err := s.ch.Publish(c.exchange, c.pubTopic, true, true, amqp.Publishing{
		Body: data,
	})
	if err != nil {
		return err
	}

	return nil
}
