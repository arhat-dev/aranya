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

// ConnHandleFunc is the handler which tend to the Connect result
// server is the server address provided by user in client creation call
// code is the ConnResult code
// err is the error happened when connect to server, if a error happened,
// the code value will max byte value (255)
type ConnHandleFunc func(client Client, server string, code byte, err error)

// Deprecated: use ConnHandleFunc instead, will be removed in v1.0
type ConnHandler func(server string, code byte, err error)

// TopicHandleFunc handles topic sub message
// topic is the client user provided topic
// code can be SubOkMaxQos0, SubOkMaxQos1, SubOkMaxQos2, SubFail
type TopicHandleFunc func(client Client, topic string, qos QosLevel, msg []byte)

// Deprecated: use TopicHandleFunc instead, will be removed in v1.0
type TopicHandler func(topic string, qos QosLevel, msg []byte)

// PubHandleFunc handles the error occurred when publish some message
// if err is not nil, that means a error occurred when sending pub msg
type PubHandleFunc func(client Client, topic string, err error)

// Deprecated: use PubHandleFunc instead, will be removed in v1.0
type PubHandler func(topic string, err error)

// SubHandleFunc handles the error occurred when subscribe some topic
// if err is not nil, that means a error occurred when sending sub msg
type SubHandleFunc func(client Client, topics []*Topic, err error)

// Deprecated: use SubHandleFunc instead, will be removed in v1.0
type SubHandler func(topics []*Topic, err error)

// UnsubHandleFunc handles the error occurred when publish some message
type UnsubHandleFunc func(client Client, topics []string, err error)

// Deprecated: use UnsubHandleFunc instead, will be removed in v1.0
type UnSubHandler func(topics []string, err error)

// NetHandleFunc handles the error occurred when net broken
type NetHandleFunc func(client Client, server string, err error)

// Deprecated: use NetHandleFunc instead, will be removed in v1.0
type NetHandler func(server string, err error)

// PersistHandleFunc handles err happened when persist process has trouble
type PersistHandleFunc func(client Client, packet Packet, err error)

// Deprecated: use PersistHandleFunc instead, will be removed in v1.0
type PersistHandler func(err error)

// HandlePub register handler for pub error
// Deprecated: use WithPubHandleFunc instead (will be removed in v1.0)
func (c *AsyncClient) HandlePub(h PubHandler) {
	c.log.d("CLI registered pub handler")
	c.pubHandler = func(client Client, topic string, err error) {
		h(topic, err)
	}
}

// HandleSub register handler for extra sub info
// Deprecated: use WithSubHandleFunc instead (will be removed in v1.0)
func (c *AsyncClient) HandleSub(h SubHandler) {
	c.log.d("CLI registered sub handler")
	c.subHandler = func(client Client, topics []*Topic, err error) {
		h(topics, err)
	}
}

// HandleUnSub register handler for unsubscribe error
// Deprecated: use WithUnsubHandleFunc instead (will be removed in v1.0)
func (c *AsyncClient) HandleUnSub(h UnSubHandler) {
	c.log.d("CLI registered unsubscribe handler")
	c.unsubHandler = func(client Client, topics []string, err error) {
		h(topics, err)
	}
}

// HandleNet register handler for net error
// Deprecated: use WithNetHandleFunc instead (will be removed in v1.0)
func (c *AsyncClient) HandleNet(h NetHandler) {
	c.log.d("CLI registered net handler")
	c.netHandler = func(client Client, server string, err error) {
		h(server, err)
	}
}

// HandlePersist register handler for net error
// Deprecated: use WithPersistHandleFunc instead (will be removed in v1.0)
func (c *AsyncClient) HandlePersist(h PersistHandler) {
	c.log.d("CLI registered persist handler")
	c.persistHandler = func(client Client, packet Packet, err error) {
		h(err)
	}
}
