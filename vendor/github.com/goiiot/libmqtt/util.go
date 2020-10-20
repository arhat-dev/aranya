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
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
	"sync/atomic"
)

// BufferedWriter buffered writer, e.g. bufio.Writer, bytes.Buffer
type BufferedWriter interface {
	io.Writer
	io.ByteWriter
}

// BufferedReader buffered reader, e.g. bufio.Reader, bytes.Buffer
type BufferedReader interface {
	io.Reader
	io.ByteReader
}

func boolToByte(flag bool) byte {
	if flag {
		return 1
	}
	return 0
}

func recvKey(packetID uint16) string {
	return fmt.Sprintf("%s%d", "R", packetID)
}

func sendKey(packetID uint16) string {
	return fmt.Sprintf("%s%d", "S", packetID)
}

type idGenerator struct {
	nextID  uint32
	usedIDs map[uint16]interface{}
	mu      *sync.RWMutex
}

func newIDGenerator() *idGenerator {
	return &idGenerator{
		nextID:  0,
		usedIDs: make(map[uint16]interface{}),
		mu:      new(sync.RWMutex),
	}
}

func (g *idGenerator) used(id uint16) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, loaded := g.usedIDs[id]
	return loaded
}

func (g *idGenerator) setUsed(id uint16, data interface{}) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.usedIDs[id] = data
}

func (g *idGenerator) next(extra interface{}) uint16 {
	id := uint16(atomic.AddUint32(&g.nextID, 1))
	if id == 0 {
		atomic.StoreUint32(&g.nextID, 1)
		id = 1
	}

	if used := g.used(id); used {
		for i := 0; used; id = uint16(atomic.AddUint32(&g.nextID, 1)) {
			if i == math.MaxUint16 {
				// id running out, caller should try some time later
				return 0
			}

			used = g.used(id)
			i++
		}
	}

	if id > 0 {
		g.setUsed(id, extra)
	}
	return id
}

func (g *idGenerator) free(id uint16) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.usedIDs, id)
}

func (g *idGenerator) getExtra(id uint16) (interface{}, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	data, ok := g.usedIDs[id]
	return data, ok
}

func putUint16(d []byte, v uint16) {
	binary.BigEndian.PutUint16(d, v)
}

func putUint32(d []byte, v uint32) {
	binary.BigEndian.PutUint32(d, v)
}

func encodeStringWithLen(str string) []byte {
	return encodeBytesWithLen([]byte(str))
}

func encodeBytesWithLen(data []byte) []byte {
	l := len(data)
	result := []byte{byte(l >> 8), byte(l)}
	return append(result, data...)
}

func varIntBytes(n int) ([]byte, error) {
	if n < 0 || n > maxMsgSize {
		return nil, ErrEncodeLargePacket
	}

	if n == 0 {
		return []byte{0}, nil
	}

	var ret []byte
	for n > 0 {
		encodedByte := byte(n % 128)
		n /= 128
		if n > 0 {
			encodedByte |= 128
		}
		ret = append(ret, encodedByte)
	}

	return ret, nil
}

func writeVarInt(n int, w BufferedWriter) error {
	if n < 0 || n > maxMsgSize {
		return ErrEncodeLargePacket
	}

	if n == 0 {
		return w.WriteByte(0)
	}

	for n > 0 {
		encodedByte := byte(n % 128)
		n /= 128
		if n > 0 {
			encodedByte |= 128
		}
		if err := w.WriteByte(encodedByte); err != nil {
			return err
		}
	}

	return nil
}

func getStringData(data []byte) (string, []byte, error) {
	b, next, err := getBinaryData(data)
	if err == nil {
		return string(b), next, nil
	}

	return "", nil, err
}

func getBinaryData(data []byte) ([]byte, []byte, error) {
	dataLen := len(data)
	if dataLen < 2 {
		return nil, nil, ErrDecodeBadPacket
	}

	end := int(getUint16(data)) + 2
	if end > dataLen {
		// out of bounds
		return nil, nil, ErrDecodeBadPacket
	}
	return data[2:end], data[end:], nil
}

func getRemainLength(r io.ByteReader) (length int, byteCount int) {
	var m uint32
	for m < 27 {
		b, err := r.ReadByte()
		if err != nil {
			return 0, 0
		}
		length |= int(b&127) << m
		if (b & 128) == 0 {
			break
		}
		m += 7
	}

	return length, int(m/7 + 1)
}

func getUint16(d []byte) uint16 {
	return binary.BigEndian.Uint16(d)
}

func getUint32(d []byte) uint32 {
	return binary.BigEndian.Uint32(d)
}

// | prop length |
// | prop body.. |
// |   payload   |
// nolint:gocyclo
func getRawProps(data []byte) (props map[byte][]byte, next []byte, err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = ErrDecodeBadPacket
		}
	}()

	propsLen, byteLen := getRemainLength(bytes.NewReader(data))
	propsBytes := data[byteLen : propsLen+byteLen]
	next = data[propsLen+byteLen:]
	props = make(map[byte][]byte)

	for i := 0; i < propsLen; {
		var p []byte
		switch propsBytes[0] {
		case propKeyPayloadFormatIndicator:
			p = propsBytes[1:2]
		case propKeyMessageExpiryInterval:
			p = propsBytes[1:5]
		case propKeyContentType:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyRespTopic:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyCorrelationData:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeySubID:
			_, byteLen := getRemainLength(bytes.NewReader(propsBytes[1:]))
			p = propsBytes[1 : 1+byteLen]
		case propKeySessionExpiryInterval:
			p = propsBytes[1:5]
		case propKeyAssignedClientID:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyServerKeepalive:
			p = propsBytes[1:3]
		case propKeyAuthMethod:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyAuthData:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyReqProblemInfo:
			p = propsBytes[1:2]
		case propKeyWillDelayInterval:
			p = propsBytes[1:5]
		case propKeyReqRespInfo:
			p = propsBytes[1:2]
		case propKeyRespInfo:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyServerRef:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyReasonString:
			p = propsBytes[1 : 3+getUint16(propsBytes[1:3])]
		case propKeyMaxRecv:
			p = propsBytes[1:3]
		case propKeyMaxTopicAlias:
			p = propsBytes[1:3]
		case propKeyTopicAlias:
			p = propsBytes[1:3]
		case propKeyMaxQos:
			p = propsBytes[1:2]
		case propKeyRetainAvail:
			p = propsBytes[1:2]
		case propKeyUserProps:
			keyEnd := 2 + getUint16(propsBytes[1:3])
			valEnd := 2 + keyEnd + getUint16(propsBytes[keyEnd+1:keyEnd+3])
			p = propsBytes[1 : valEnd+1]
		case propKeyMaxPacketSize:
			p = propsBytes[1:5]
		case propKeyWildcardSubAvail:
			p = propsBytes[1:2]
		case propKeySubIDAvail:
			p = propsBytes[1:2]
		case propKeySharedSubAvail:
			p = propsBytes[1:2]
		default:
			err = ErrDecodeBadPacket
			return
		}
		props[propsBytes[0]] = append(props[propsBytes[0]], p...)
		propsBytes = propsBytes[1+len(p):]
		i += 1 + len(p)
	}

	return
}

func getUserProps(data []byte) UserProps {
	props := make(UserProps)
	strKey, next, _ := getStringData(data)
	for ; next != nil; strKey, next, _ = getStringData(next) {
		var val string
		val, next, _ = getStringData(next)
		props[strKey] = append(props[strKey], val)
	}
	return props
}
