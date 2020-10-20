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
	"io"
	"net"
	"sync/atomic"
	"time"
)

// clientConn is the wrapper of connection to server
// tend to actual packet send and receive
// nolint:maligned
type clientConn struct {
	protoVersion ProtoVersion      // mqtt protocol version
	parent       Client            // client which created this connection
	name         string            // server addr info
	conn         net.Conn          // connection to server
	connRW       *bufio.ReadWriter // make buffered connection
	logicSendC   chan Packet       // logic send channel
	netRecvC     chan Packet       // received packet from server
	keepaliveC   chan struct{}     // keepalive packet
	parentExit   uint32

	ctx     context.Context    // context for single connection
	exit    context.CancelFunc // terminate this connection if necessary
	stopSig <-chan struct{}
}

func (c *clientConn) parentExiting() bool {
	return atomic.LoadUint32(&c.parentExit) == 1
}

// start mqtt logic
// nolint:gocyclo
func (c *clientConn) logic() {
	defer func() {
		err := c.conn.Close()
		if err != nil {
			notifyNetMsg(c.parent.msgCh, c.name, err)
		} else {
			notifyNetMsg(c.parent.msgCh, c.name, io.EOF)
		}

		c.parent.log.e("NET exit logic for server =", c.name)
	}()

	// start keepalive if required
	if c.parent.options.keepalive > 0 {
		c.parent.addWorker(c.keepalive)
	}

	for {
		select {
		case pkt, more := <-c.netRecvC:
			if !more {
				return
			}

			switch p := pkt.(type) {
			case *SubAckPacket:
				c.parent.log.v("NET received SubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					// nolint:gocritic
					switch originSub := originPkt.(type) {
					case *SubscribePacket:
						N := len(p.Codes)
						for i, v := range originSub.Topics {
							if i < N {
								v.Qos = p.Codes[i]
							}
						}
						c.parent.log.d("NET subscribed topics =", originSub.Topics)
						notifySubMsg(c.parent.msgCh, originSub.Topics, nil)
						c.parent.idGen.free(p.PacketID)

						notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Delete(sendKey(p.PacketID)))
					}
				}
			case *UnsubAckPacket:
				c.parent.log.v("NET received UnSubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					// nolint:gocritic
					switch originUnSub := originPkt.(type) {
					case *UnsubPacket:
						c.parent.log.d("NET unsubscribed topics", originUnSub.TopicNames)
						notifyUnSubMsg(c.parent.msgCh, originUnSub.TopicNames, nil)
						c.parent.idGen.free(p.PacketID)

						notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Delete(sendKey(p.PacketID)))
					}
				}
			case *PublishPacket:
				c.parent.log.v("NET received publish, topic =", p.TopicName, "id =", p.PacketID, "QoS =", p.Qos)
				// received server publish, send to client
				c.parent.recvCh <- p

				// tend to QoS
				switch p.Qos {
				case Qos1:
					c.parent.log.d("NET send PubAck for Publish, id =", p.PacketID)
					c.send(&PubAckPacket{PacketID: p.PacketID})

					notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Store(recvKey(p.PacketID), pkt))
				case Qos2:
					c.parent.log.d("NET send PubRecv for Publish, id =", p.PacketID)
					c.send(&PubRecvPacket{PacketID: p.PacketID})

					notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Store(recvKey(p.PacketID), pkt))
				}
			case *PubAckPacket:
				c.parent.log.v("NET received PubAck, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					// nolint:gocritic
					switch originPub := originPkt.(type) {
					case *PublishPacket:
						if originPub.Qos == Qos1 {
							c.parent.log.d("NET published qos1 packet, topic =", originPub.TopicName)
							notifyPubMsg(c.parent.msgCh, originPub.TopicName, nil)
							c.parent.idGen.free(p.PacketID)

							notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Delete(sendKey(p.PacketID)))
						}
					}
				}
			case *PubRecvPacket:
				c.parent.log.v("NET received PubRec, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					// nolint:gocritic
					switch originPub := originPkt.(type) {
					case *PublishPacket:
						if originPub.Qos == Qos2 {
							c.send(&PubRelPacket{PacketID: p.PacketID})
							c.parent.log.d("NET send PubRel, id =", p.PacketID)
						}
					}
				}
			case *PubRelPacket:
				c.parent.log.v("NET send PubRel, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					// nolint:gocritic
					switch originPub := originPkt.(type) {
					case *PublishPacket:
						if originPub.Qos == Qos2 {
							c.send(&PubCompPacket{PacketID: p.PacketID})
							c.parent.log.d("NET send PubComp, id =", p.PacketID)

							notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Store(recvKey(p.PacketID), pkt))
						}
					}
				}
			case *PubCompPacket:
				c.parent.log.v("NET received PubComp, id =", p.PacketID)

				if originPkt, ok := c.parent.idGen.getExtra(p.PacketID); ok {
					// nolint:gocritic
					switch originPub := originPkt.(type) {
					case *PublishPacket:
						if originPub.Qos == Qos2 {
							c.send(&PubRelPacket{PacketID: p.PacketID})
							c.parent.log.d("NET send PubRel, id =", p.PacketID)
							c.parent.log.d("NET published qos2 packet, topic =", originPub.TopicName)
							notifyPubMsg(c.parent.msgCh, originPub.TopicName, nil)
							c.parent.idGen.free(p.PacketID)

							notifyPersistMsg(c.parent.msgCh, p, c.parent.persist.Delete(sendKey(p.PacketID)))
						}
					}
				}
			default:
				c.parent.log.v("NET received packet, type =", pkt.Type())
			}
		case <-c.stopSig:
			return
		}
	}
}

// keepalive with server
func (c *clientConn) keepalive() {
	c.parent.log.d("NET start keepalive")

	t := time.NewTicker(c.parent.options.keepalive * 3 / 4)
	timeout := time.Duration(float64(c.parent.options.keepalive) * c.parent.options.keepaliveFactor)
	timeoutTimer := time.NewTimer(timeout)

	defer func() {
		t.Stop()
		timeoutTimer.Stop()
		c.parent.log.d("NET stop keepalive for server =", c.name)
	}()

	for {
		select {
		case <-t.C:
			c.send(PingReqPacket)

			select {
			case _, more := <-c.keepaliveC:
				if !more {
					return
				}

				timeoutTimer.Reset(timeout)
			case <-timeoutTimer.C:
				c.parent.log.i("NET keepalive timeout")
				// exit client connection
				c.exit()
				return
			case <-c.stopSig:
				return
			}
		case <-c.stopSig:
			return
		}
	}
}

const (
	flushDelayInterval = 100 * time.Microsecond
)

// handle mqtt logic control packet send
func (c *clientConn) handleSend() {
	c.parent.log.v("NET clientConn.handleSend() for server =", c.name)

	flushSig := time.NewTimer(time.Hour)
	defer func() {
		c.parent.log.e("NET exit clientConn.handleSend() for server =", c.name)
		flushSig.Stop()
	}()

	for {
		select {
		case <-c.stopSig:
			return
		case <-flushSig.C:
			if err := c.connRW.Flush(); err != nil {
				c.parent.log.e("NET flush error", err)
				flushSig.Reset(time.Hour)

				notifyNetMsg(c.parent.msgCh, c.name, err)
				return
			}
		case pkt, more := <-c.parent.sendCh:
			if !more {
				return
			}

			pkt.SetVersion(c.protoVersion)
			if err := pkt.WriteTo(c.connRW); err != nil {
				c.parent.log.e("NET encode error", err)
				return
			}
			flushSig.Reset(flushDelayInterval)

			switch p := pkt.(type) {
			case *PublishPacket:
				if p.Qos == 0 {
					c.parent.log.d("NET published qos0 packet, topic =", p.TopicName)
					notifyPubMsg(c.parent.msgCh, p.TopicName, nil)
				}
			case *DisconnPacket:
				// client exit with disconnect
				if err := c.connRW.Flush(); err != nil {
					c.parent.log.e("NET flush error", err)
					notifyNetMsg(c.parent.msgCh, c.name, err)
				}
				_ = c.conn.Close()

				c.exit()
				return
			}
		case pkt, more := <-c.logicSendC:
			if !more {
				return
			}

			pkt.SetVersion(c.protoVersion)
			if err := pkt.WriteTo(c.connRW); err != nil {
				c.parent.log.e("NET encode error", err)
				return
			}
			flushSig.Reset(flushDelayInterval)

			switch p := pkt.(type) {
			case *PubRelPacket:
				notifyPersistMsg(c.parent.msgCh, pkt,
					c.parent.persist.Store(sendKey(p.PacketID), pkt))
			case *PubAckPacket:
				notifyPersistMsg(c.parent.msgCh, pkt,
					c.parent.persist.Delete(sendKey(p.PacketID)))
			case *PubCompPacket:
				notifyPersistMsg(c.parent.msgCh, pkt,
					c.parent.persist.Delete(sendKey(p.PacketID)))
			case *DisconnPacket:
				// disconnect to server, no more action
				if err := c.connRW.Flush(); err != nil {
					c.parent.log.e("NET flush error", err)
					notifyNetMsg(c.parent.msgCh, c.name, err)
				}
				_ = c.conn.Close()

				c.exit()
				return
			}
		}
	}
}

// handle all message receive
func (c *clientConn) handleNetRecv() {
	c.parent.log.v("NET clientConn.handleNetRecv() for server =", c.name)

	defer func() {
		c.parent.log.v("NET exit clientConn.handleNetRecv() for server =", c.name)
		close(c.netRecvC)
		close(c.keepaliveC)
	}()

	for {
		pkt, err := Decode(c.protoVersion, c.connRW)
		if err != nil {
			c.parent.log.e("NET connection broken, server =", c.name, "err =", err)

			// exit client connection
			notifyNetMsg(c.parent.msgCh, c.name, err)
			c.exit()
			return
		}

		if pkt.Version() != c.protoVersion {
			// protocol version not match, exit
			c.exit()
			return
		}

		if pkt.Type() == CtrlPingResp {
			c.parent.log.d("NET received keepalive message")
			select {
			case c.keepaliveC <- struct{}{}:
			case <-c.stopSig:
			}
		} else {
			select {
			case c.netRecvC <- pkt:
			case <-c.stopSig:
			}
		}
	}
}

// send mqtt logic packet
func (c *clientConn) send(pkt Packet) {
	select {
	case c.logicSendC <- pkt:
	case <-c.stopSig:
	}
}

// send mqtt logic packet before handleSend() is started
func (c *clientConn) sendRaw(pkt Packet) error {
	if err := pkt.WriteTo(c.connRW); err != nil {
		c.parent.log.e("NET encode error", err)
		return err
	}

	if err := c.connRW.Flush(); err != nil {
		c.parent.log.e("NET flush error", err)
		notifyNetMsg(c.parent.msgCh, c.name, err)

		return err
	}

	return nil
}
