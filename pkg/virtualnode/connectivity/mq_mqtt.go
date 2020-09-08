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
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"arhat.dev/aranya-proto/aranyagopb/aranyagoconst"

	"arhat.dev/pkg/log"
	"github.com/goiiot/libmqtt"

	"arhat.dev/aranya-proto/aranyagopb"
)

var errAlreadySubscribing = fmt.Errorf("already subscribing")

func newMQTTClient(parent *MessageQueueManager) (*mqttClient, error) {
	opts := parent.config.MQTTOpts
	options := []libmqtt.Option{
		libmqtt.WithBackoffStrategy(time.Second, 5*time.Second, 1.2),
	}

	switch opts.Config.Version {
	case "5":
		options = append(options, libmqtt.WithVersion(libmqtt.V5, false))
	case "3.1.1", "":
		options = append(options, libmqtt.WithVersion(libmqtt.V311, false))
	default:
		return nil, fmt.Errorf("unsupported mqtt protocol version")
	}

	switch opts.Config.Transport {
	case "websocket":
		options = append(options, libmqtt.WithWebSocketConnector(0, nil))
	case "tcp", "":
		options = append(options, libmqtt.WithTCPConnector(0))
	default:
		return nil, fmt.Errorf("unsupported transport method")
	}

	keepalive := opts.Config.Keepalive
	if keepalive == 0 {
		// default to 60s
		keepalive = 60
	}

	options = append(options, libmqtt.WithConnPacket(libmqtt.ConnPacket{
		CleanSession: true,
		Username:     string(opts.Username),
		Password:     string(opts.Password),
		ClientID:     opts.Config.ClientID,
		Keepalive:    uint16(keepalive),
	}))
	options = append(options, libmqtt.WithKeepalive(uint16(keepalive), 1.2))

	if opts.TLSConfig != nil {
		options = append(options, libmqtt.WithCustomTLS(opts.TLSConfig))
	}

	client, err := libmqtt.NewClient(options...)
	if err != nil {
		return nil, err
	}

	pubTopic, subTopic, subWillTopic := aranyagoconst.MQTTTopics(opts.Config.TopicNamespace)
	return &mqttClient{
		log:    parent.log,
		broker: opts.Config.Broker,
		client: client,

		onRecvMsg: parent.onRecvMsg,
		rejectAgent: func() {
			parent.Reject(aranyagopb.REJECTION_INTERNAL_SERVER_ERROR, "mqtt connectivity lost")
		},

		pubTopic:     pubTopic,
		subTopic:     subTopic,
		subWillTopic: subWillTopic,

		stopSig:   parent.ctx.Done(),
		connErrCh: make(chan error),
		subErrCh:  make(chan error),
	}, nil
}

type mqttClient struct {
	log         log.Interface
	broker      string
	client      libmqtt.Client
	onRecvMsg   func(*aranyagopb.Msg)
	rejectAgent func()

	pubTopic     string
	subTopic     string
	subWillTopic string

	subscribing int32
	started     int32
	closed      int32
	stopSig     <-chan struct{}
	connErrCh   chan error
	subErrCh    chan error
}

func (c *mqttClient) PubTopics() []string {
	return []string{c.pubTopic}
}

func (c *mqttClient) SubTopics() []string {
	return []string{c.subTopic, c.subWillTopic}
}

// Connect to MQTT broker with connect packet
func (c *mqttClient) Connect() error {
	err := c.client.ConnectServer(c.broker,
		libmqtt.WithRouter(libmqtt.NewTextRouter()),
		libmqtt.WithAutoReconnect(true),
		libmqtt.WithConnHandleFunc(c.handleConn),
		libmqtt.WithSubHandleFunc(c.handleSub),
		libmqtt.WithPubHandleFunc(c.handlePub),
		libmqtt.WithNetHandleFunc(c.handleNet),
	)
	if err != nil {
		return err
	}

	select {
	case <-c.stopSig:
		return context.Canceled
	case err, more := <-c.connErrCh:
		if !more {
			return nil
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// Disconnect from the MQTT broker
func (c *mqttClient) Disconnect() error {
	atomic.StoreInt32(&c.closed, 1)
	c.client.Destroy(false)
	return nil
}

// Subscribe to MQTT topic with QoS 1
func (c *mqttClient) Subscribe() error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return ErrManagerClosed
	}

	if !atomic.CompareAndSwapInt32(&c.subscribing, 0, 1) {
		return errAlreadySubscribing
	}

	defer func() {
		atomic.StoreInt32(&c.subscribing, 0)
		atomic.StoreInt32(&c.started, 1)
	}()

	c.client.HandleTopic(c.subTopic, c.handleTopicMsg)
	c.client.HandleTopic(c.subWillTopic, c.handleTopicMsg)

	c.client.Subscribe(
		&libmqtt.Topic{Name: c.subTopic, Qos: libmqtt.Qos1},
		&libmqtt.Topic{Name: c.subWillTopic, Qos: libmqtt.Qos1},
	)

	select {
	case <-c.stopSig:
		return context.Canceled
	case err := <-c.subErrCh:
		if err != nil {
			return fmt.Errorf("failed to subscribe topics: %w", err)
		}
	}

	return nil
}

// Publish data to MQTT topic with QoS 1
func (c *mqttClient) Publish(data []byte) error {
	c.client.Publish(&libmqtt.PublishPacket{TopicName: c.pubTopic, Qos: libmqtt.Qos1, Payload: data})
	return nil
}

func (c *mqttClient) handleNet(client libmqtt.Client, server string, err error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		// we don't care since this client has been destroyed
		return
	}

	if err != nil {
		if atomic.LoadInt32(&c.subscribing) == 1 && atomic.LoadInt32(&c.started) == 0 {
			select {
			case <-c.stopSig:
				return
			case c.subErrCh <- err:
				// we can close subErrCh here since subscribe has failed
				// and no more action will happen in this client
				close(c.subErrCh)
				return
			}
		}

		if atomic.CompareAndSwapInt32(&c.started, 0, 1) {
			select {
			case <-c.stopSig:
				return
			case c.connErrCh <- err:
				close(c.connErrCh)
				return
			}
		}

		c.log.I("network error happened", log.String("server", server), log.Error(err))
	}
}

// nolint:gocritic
func (c *mqttClient) handleConn(client libmqtt.Client, server string, code byte, err error) {
	if atomic.LoadInt32(&c.closed) == 1 {
		// we don't care since this client has been destroyed
		return
	}

	if err != nil {
		if atomic.CompareAndSwapInt32(&c.started, 0, 1) {
			select {
			case <-c.stopSig:
				return
			case c.connErrCh <- err:
				close(c.connErrCh)
				return
			}
		}

		c.log.I("failed to connect to broker", log.Uint8("code", code), log.Error(err))
	} else if code != libmqtt.CodeSuccess {
		if atomic.CompareAndSwapInt32(&c.started, 0, 1) {
			select {
			case <-c.stopSig:
				return
			case c.connErrCh <- fmt.Errorf("rejected by mqtt broker, code: %d", code):
				close(c.connErrCh)
				return
			}
		}

		c.log.I("reconnect rejected by broker", log.Uint8("code", code))
	} else {
		// connection success
		if atomic.LoadInt32(&c.started) == 0 {
			// client still in initial stage
			// close connErrCh to signal connection success
			close(c.connErrCh)
		} else {
			// client has started, connection success means reconnection success
			// so we need to resubscribe topics here
			for {
				if err := c.Subscribe(); err != nil {
					if err == errAlreadySubscribing ||
						errors.Is(err, context.Canceled) ||
						errors.Is(err, context.DeadlineExceeded) ||
						errors.Is(err, ErrManagerClosed) {
						return
					}

					c.log.I("failed to resubscribe to topics after reconnection", log.Error(err))
					time.Sleep(5 * time.Second)
				} else {
					// reject client to cleanup session
					//c.rejectAgent()
					//
					c.log.V("resubscribed to topics after connection lost")
					return
				}
			}
		}
	}
}

func (c *mqttClient) handleSub(client libmqtt.Client, topics []*libmqtt.Topic, err error) {
	if err != nil {
		c.log.I("failed to subscribe", log.Error(err), log.Any("topics", topics))
	} else {
		c.log.D("subscribe succeeded", log.Any("topics", topics))
	}

	select {
	case <-c.stopSig:
		return
	case c.subErrCh <- err:
		return
	}
}

func (c *mqttClient) handlePub(client libmqtt.Client, topic string, err error) {
	if err != nil {
		c.log.I("failed to publish message", log.String("topic", topic), log.Error(err))
	}
}

func (c *mqttClient) handleTopicMsg(client libmqtt.Client, topic string, qos libmqtt.QosLevel, msgBytes []byte) {
	msg := new(aranyagopb.Msg)
	err := msg.Unmarshal(msgBytes)
	if err != nil {
		c.log.I("failed to unmarshal msg", log.Binary("msgBytes", msgBytes), log.Error(err))
		return
	}

	c.onRecvMsg(msg)
}
