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
	"fmt"
	"io"
)

var (
	// TODO: set default
	// nolint:deadcode,unused,varcheck
	propsDefaultBoolValue = map[byte]bool{
		propKeyRetainAvail:      true,
		propKeyWildcardSubAvail: true,
		propKeySubIDAvail:       true,
		propKeySharedSubAvail:   true,
	}
)

type propertySet map[byte][][]byte

func (p propertySet) add(propKey byte, propValue interface{}) {
	var val []byte
	switch v := propValue.(type) {
	case string:
		if propValue.(string) != "" {
			val = encodeStringWithLen(propValue.(string))
		}
	case []byte:
		if len(propValue.([]byte)) > 0 {
			val = encodeBytesWithLen(propValue.([]byte))
		}
	case *bool:
		if *v {
			val = []byte{1}
		} else {
			val = []byte{0}
		}
	case bool:
		if v {
			val = []byte{1}
		} else {
			val = []byte{0}
		}
	case uint8:
		if v != 0 {
			val = []byte{v}
		}
	case uint16:
		if v != 0 {
			val = make([]byte, 2)
			putUint16(val, v)
		}
	case int:
		val, _ = varIntBytes(v)
	case uint32:
		if v != 0 {
			val = make([]byte, 4)
			putUint32(val, v)
		}
	case UserProps:
		v.encodeTo(&val)
	case nil:
		return
	default:
		panic(fmt.Sprintf("unexpected property value type %T", v))
	}

	if v, ok := p[propKey]; ok {
		p[propKey] = append(v, val)
	}
}

func (p propertySet) set(propKey byte, propValue interface{}) {
	p.del(propKey)
	p.add(propKey, propValue)
}

func (p propertySet) del(propKey byte) {
	delete(p, propKey)
}

func (p propertySet) bytes() []byte {
	var ret []byte
	for propKey, propValue := range p {
		for _, v := range propValue {
			ret = append(ret, propKey)
			ret = append(ret, v...)
		}
	}
	return ret
}

// UserProps contains user defined properties
type UserProps map[string][]string

func (u UserProps) Add(key, value string) {
	val, ok := u[key]
	if !ok || val == nil {
		val = make([]string, 0)
	}

	u[key] = append(val, value)
}

func (u UserProps) Get(key string) (string, bool) {
	if val, ok := u[key]; ok && len(val) > 0 {
		return val[0], true
	}

	return "", false
}

func (u UserProps) Set(key string, value string) {
	u[key] = []string{value}
}

func (u UserProps) Del(key string) {
	delete(u, key)
}

func (u UserProps) encodeTo(result *([]byte)) {
	for k, v := range u {
		for _, val := range v {
			// result = append(result, propKeyUserProps)
			*result = append(*result, encodeStringWithLen(k)...)
			*result = append(*result, encodeStringWithLen(val)...)
		}
	}
}

// Packet is MQTT control packet
type Packet interface {
	// Type return the packet type
	Type() CtrlType

	// Bytes presentation of this packet
	Bytes() []byte

	// Write bytes to the buffered writer
	WriteTo(w BufferedWriter) error

	// Version MQTT version of the packet
	Version() ProtoVersion

	SetVersion(version ProtoVersion)
}

// BasePacket for packet encoding and MQTT version note
type BasePacket struct {
	ProtoVersion ProtoVersion
}

func (b *BasePacket) write(w io.Writer, first byte, varHeader, payload []byte) error {
	_, err := w.Write([]byte{first})
	if err != nil {
		return err
	}

	remainingLengthBytes, err := varIntBytes(len(varHeader) + len(payload))
	if err != nil {
		return err
	}

	_, err = w.Write(remainingLengthBytes)
	if err != nil {
		return err
	}

	if varHeader != nil {
		_, err = w.Write(varHeader)
		if err != nil {
			return err
		}
	}

	if payload != nil {
		_, err = w.Write(payload)
	}
	return err
}

func (b *BasePacket) writeV5(w io.Writer, first byte, varHeader, props, payload []byte) error {
	propLen := len(props)
	propsLengthBytes, err := varIntBytes(propLen)
	if err != nil {
		return err
	}

	actualVarHeader := make([]byte, 0, len(varHeader)+len(propsLengthBytes)+propLen)
	actualVarHeader = append(actualVarHeader, varHeader...)
	actualVarHeader = append(actualVarHeader, propsLengthBytes...)
	actualVarHeader = append(actualVarHeader, props...)
	return b.write(w, first, actualVarHeader, payload)
}

func (b *BasePacket) SetVersion(version ProtoVersion) {
	b.ProtoVersion = version
}

// Version is the MQTT version of this packet
func (b *BasePacket) Version() ProtoVersion {
	if b.ProtoVersion != 0 {
		return b.ProtoVersion
	}

	// default version is MQTT 3.1.1
	return V311
}

// Topic for both topic name and topic qos
type Topic struct {
	Name string
	Qos  QosLevel
}

func (t *Topic) String() string {
	return t.Name
}

const (
	maxMsgSize = 268435455
)

// CtrlType is MQTT Control packet type
type CtrlType = byte

const (
	CtrlConn      CtrlType = 1  // Connect
	CtrlConnAck   CtrlType = 2  // connect ack
	CtrlPublish   CtrlType = 3  // publish
	CtrlPubAck    CtrlType = 4  // publish ack
	CtrlPubRecv   CtrlType = 5  // publish received
	CtrlPubRel    CtrlType = 6  // publish release
	CtrlPubComp   CtrlType = 7  // publish complete
	CtrlSubscribe CtrlType = 8  // subscribe
	CtrlSubAck    CtrlType = 9  // subscribe ack
	CtrlUnSub     CtrlType = 10 // unsubscribe
	CtrlUnSubAck  CtrlType = 11 // unsubscribe ack
	CtrlPingReq   CtrlType = 12 // ping request
	CtrlPingResp  CtrlType = 13 // ping response
	CtrlDisConn   CtrlType = 14 // disconnect
	CtrlAuth      CtrlType = 15 // authentication (since MQTT 5)
)

// ProtoVersion MQTT Protocol ProtoVersion
type ProtoVersion byte

const (
	V311 ProtoVersion = 4 // V311 means MQTT 3.1.1
	V5   ProtoVersion = 5 // V5 means MQTT 5
)

// QosLevel is either 0, 1, 2
type QosLevel = byte

const (
	Qos0 QosLevel = 0x00 // Qos0 = 0
	Qos1 QosLevel = 0x01 // Qos1 = 1
	Qos2 QosLevel = 0x02 // Qos2 = 2
)

const (
	SubOkMaxQos0 = 0    // SubOkMaxQos0 QoS 0 is used by server
	SubOkMaxQos1 = 1    // SubOkMaxQos1 QoS 1 is used by server
	SubOkMaxQos2 = 2    // SubOkMaxQos2 QoS 2 is used by server
	SubFail      = 0x80 // SubFail means that subscription is not successful
)

// reason code

const (
	// MQTT 3.1.1 ConnAck code
	CodeUnacceptableVersion   = 1 // Packet: ConnAck
	CodeIdentifierRejected    = 2 // Packet: ConnAck
	CodeServerUnavailable     = 3 // Packet: ConnAck
	CodeBadUsernameOrPassword = 4 // Packet: ConnAck
	CodeUnauthorized          = 5 // Packet: ConnAck
)

const (
	CodeSuccess                             = 0   // Packet: ConnAck, PubAck, PubRecv, PubRel, PubComp, UnSubAck, Auth
	CodeNormalDisconn                       = 0   // Packet: DisConn
	CodeGrantedQos0                         = 0   // Packet: SubAck
	CodeGrantedQos1                         = 1   // Packet: SubAck
	CodeGrantedQos2                         = 2   // Packet: SubAck
	CodeDisconnWithWill                     = 4   // Packet: DisConn
	CodeNoMatchingSubscribers               = 16  // Packet: PubAck, PubRecv
	CodeNoSubscriptionExisted               = 17  // Packet: UnSubAck
	CodeContinueAuth                        = 24  // Packet: Auth
	CodeReAuth                              = 25  // Packet: Auth
	CodeUnspecifiedError                    = 128 // Packet: ConnAck, PubAck, PubRecv, SubAck, UnSubAck, DisConn
	CodeMalformedPacket                     = 129 // Packet: ConnAck, DisConn
	CodeProtoError                          = 130 // Packet: ConnAck, DisConn
	CodeImplementationSpecificError         = 131 // Packet: ConnAck, PubAck, PubRecv, SubAck, UnSubAck, DisConn
	CodeUnsupportedProtoVersion             = 132 // Packet: ConnAck
	CodeClientIDNotValid                    = 133 // Packet: ConnAck
	CodeBadUserPass                         = 134 // Packet: ConnAck
	CodeNotAuthorized                       = 135 // Packet: ConnAck, PubAck, PubRecv, SubAck, UnSubAck, DisConn
	CodeServerUnavail                       = 136 // Packet: ConnAck
	CodeServerBusy                          = 137 // Packet: ConnAck, DisConn
	CodeBanned                              = 138 // Packet: ConnAck
	CodeServerShuttingDown                  = 139 // Packet: DisConn
	CodeBadAuthenticationMethod             = 140 // Packet: ConnAck, DisConn
	CodeKeepaliveTimeout                    = 141 // Packet: DisConn
	CodeSessionTakenOver                    = 142 // Packet: DisConn
	CodeTopicFilterInvalid                  = 143 // Packet: SubAck, UnSubAck, DisConn
	CodeTopicNameInvalid                    = 144 // Packet: ConnAck, PubAck, PubRecv, DisConn
	CodePacketIdentifierInUse               = 145 // Packet: PubAck, PubRecv, PubAck, UnSubAck
	CodePacketIdentifierNotFound            = 146 // Packet: PubRel, PubComp
	CodeReceiveMaxExceeded                  = 147 // Packet: DisConn
	CodeTopicAliasInvalid                   = 148 // Packet: DisConn
	CodePacketTooLarge                      = 149 // Packet: ConnAck, DisConn
	CodeMessageRateTooHigh                  = 150 // Packet: DisConn
	CodeQuotaExceeded                       = 151 // Packet: ConnAck, PubAck, PubRec, SubAck, DisConn
	CodeAdministrativeAction                = 152 // Packet: DisConn
	CodePayloadFormatInvalid                = 153 // Packet: ConnAck, PubAck, PubRecv, DisConn
	CodeRetainNotSupported                  = 154 // Packet: ConnAck, DisConn
	CodeQosNoSupported                      = 155 // Packet: ConnAck, DisConn
	CodeUseAnotherServer                    = 156 // Packet: ConnAck, DisConn
	CodeServerMoved                         = 157 // Packet: ConnAck, DisConn
	CodeSharedSubscriptionNotSupported      = 158 // Packet: SubAck, DisConn
	CodeConnectionRateExceeded              = 159 // Packet: ConnAck, DisConn
	CodeMaxConnectTime                      = 160 // Packet: DisConn
	CodeSubscriptionIdentifiersNotSupported = 161 // Packet: SubAck, DisConn
	CodeWildcardSubscriptionNotSupported    = 162 // Packet: SubAck, DisConn
)

// property identifiers

// nolint:lll
const (
	propKeyPayloadFormatIndicator = 1  // byte, Packet: Will, Publish
	propKeyMessageExpiryInterval  = 2  // Uint (4 bytes), Packet: Will, Publish
	propKeyContentType            = 3  // utf-8, Packet: Will, Publish
	propKeyRespTopic              = 8  // utf-8, Packet: Will, Publish
	propKeyCorrelationData        = 9  // binary data, Packet: Will, Publish
	propKeySubID                  = 11 // uint (variable bytes), Packet: Publish, Subscribe
	propKeySessionExpiryInterval  = 17 // uint (4 bytes), Packet: Connect, ConnAck, DisConn\
	propKeyAssignedClientID       = 18 // utf-8, Packet: ConnAck
	propKeyServerKeepalive        = 19 // uint (2 bytes), Packet: ConnAck
	propKeyAuthMethod             = 21 // utf-8, Packet: Connect, ConnAck, Auth
	propKeyAuthData               = 22 // binary data, Packet: Connect, ConnAck, Auth
	propKeyReqProblemInfo         = 23 // byte, Packet: Connect
	propKeyWillDelayInterval      = 24 // uint (4 bytes), Packet: Will
	propKeyReqRespInfo            = 25 // byte, Packet: Connect
	propKeyRespInfo               = 26 // utf-8, Packet: ConnAck
	propKeyServerRef              = 28 // utf-8, Packet: ConnAck, DisConn
	propKeyReasonString           = 31 // utf-8, Packet: ConnAck, PubAck, PubRecv, PubRel, PubComp, SubAck, UnSubAck, DisConn, Auth
	propKeyMaxRecv                = 33 // uint (2 bytes), Packet: Connect, ConnAck
	propKeyMaxTopicAlias          = 34 // uint (2 bytes), Packet: Connect, ConnAck
	propKeyTopicAlias             = 35 // uint (2 bytes), Packet: Publish
	propKeyMaxQos                 = 36 // byte, Packet: ConnAck
	propKeyRetainAvail            = 37 // byte, Packet: ConnAck
	propKeyUserProps              = 38 // utf-8 string pair, Packet: Connect, ConnAck, Publish, Will, PubAck, PubRecv, PubRel, PubComp, Subscribe, SubAck, UnSub, UnSubAck, DisConn, Auth
	propKeyMaxPacketSize          = 39 // uint (4 bytes), Packet: Connect, ConnAck
	propKeyWildcardSubAvail       = 40 // byte, Packet: ConnAck
	propKeySubIDAvail             = 41 // byte, Packet: ConnAck
	propKeySharedSubAvail         = 42 // byte, Packet: ConnAck
)
