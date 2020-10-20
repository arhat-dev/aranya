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

type msgType uint8

const (
	pubMsg msgType = iota
	subMsg
	unSubMsg
	netMsg
	persistMsg
)

type message struct {
	what msgType
	msg  string
	err  error
	obj  interface{}
}

func notifyPubMsg(ch chan<- *message, topic string, err error) {
	ch <- &message{
		what: pubMsg,
		msg:  topic,
		err:  err,
	}
}

func notifySubMsg(ch chan<- *message, p []*Topic, err error) {
	ch <- &message{
		what: subMsg,
		obj:  p,
		err:  err,
	}
}

func notifyUnSubMsg(ch chan<- *message, topics []string, err error) {
	ch <- &message{
		what: unSubMsg,
		err:  err,
		obj:  topics,
	}
}

func notifyNetMsg(ch chan<- *message, server string, err error) {
	ch <- &message{
		what: netMsg,
		msg:  server,
		err:  err,
	}
}

func notifyPersistMsg(ch chan<- *message, packet Packet, err error) {
	if err == nil {
		return
	}

	ch <- &message{
		what: persistMsg,
		err:  err,
		obj:  packet,
	}
}

func (c *AsyncClient) handleMsg() {
	for {
		select {
		case <-c.stopSig:
			return
		case m, more := <-c.msgCh:
			if !more {
				return
			}

			switch m.what {
			case pubMsg:
				if c.pubHandler != nil {
					c.addWorker(func() { c.pubHandler(c, m.msg, m.err) })
				}
			case subMsg:
				if c.subHandler != nil {
					c.addWorker(func() { c.subHandler(c, m.obj.([]*Topic), m.err) })
				}
			case unSubMsg:
				if c.unsubHandler != nil {
					c.addWorker(func() { c.unsubHandler(c, m.obj.([]string), m.err) })
				}
			case netMsg:
				if c.netHandler != nil {
					c.addWorker(func() { c.netHandler(c, m.msg, m.err) })
				}
			case persistMsg:
				if c.persistHandler != nil {
					c.addWorker(func() { c.persistHandler(c, m.obj.(Packet), m.err) })
				}
			}
		}
	}
}
