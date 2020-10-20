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
	"sync"
)

type WillProps struct {
	// The Server delays publishing the Client’s Will Message until
	// the Will Delay Interval has passed or the Session ends, whichever happens first.
	//
	// If a new Network Connection to this Session is made before the Will Delay Interval has passed,
	// the Server MUST NOT send the Will Message
	WillDelayInterval uint32

	PayloadFormat uint8

	// the lifetime of the Will Message in seconds and is sent as the Publication Expiry
	// Interval when the Server publishes the Will Message.
	MessageExpiryInterval uint32

	// String describing the content of the Will Message
	ContentType string

	// String which is used as the Topic Name for a response message
	ResponseTopic string

	// The Correlation Data is used by the sender of the Request Message to identify
	// which request the Response Message is for when it is received.
	CorrelationData []byte

	UserProps UserProps
}

func (p *WillProps) props() []byte {
	if p == nil {
		return nil
	}

	propSet := propertySet{}
	propSet.set(propKeyWillDelayInterval, p.WillDelayInterval)
	propSet.set(propKeyPayloadFormatIndicator, p.PayloadFormat)
	propSet.set(propKeyMessageExpiryInterval, p.MessageExpiryInterval)
	propSet.set(propKeyContentType, p.ContentType)
	propSet.set(propKeyRespTopic, p.ResponseTopic)
	propSet.set(propKeyCorrelationData, p.CorrelationData)
	propSet.set(propKeyUserProps, p.UserProps)
	return propSet.bytes()
}

// ConnPacket is the first packet sent by Client to Server
// nolint:maligned
type ConnPacket struct {
	BasePacket
	ProtoName string

	// Flags

	CleanSession bool
	IsWill       bool
	WillQos      QosLevel
	WillRetain   bool
	WillProps    *WillProps

	// Properties
	Props *ConnProps

	// Payloads
	Username    string
	Password    string
	ClientID    string
	Keepalive   uint16
	WillTopic   string
	WillMessage []byte
}

// Type ConnPacket's type is CtrlConn
func (c *ConnPacket) Type() CtrlType {
	return CtrlConn
}

func (c *ConnPacket) Bytes() []byte {
	if c == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = c.WriteTo(w)
	return w.Bytes()
}

func (c *ConnPacket) WriteTo(w BufferedWriter) error {
	if c == nil {
		return ErrEncodeBadPacket
	}

	const first = CtrlConn << 4
	varHeader := []byte{0x0, 0x4, 'M', 'Q', 'T', 'T', byte(V311), c.flags(), byte(c.Keepalive >> 8), byte(c.Keepalive)}
	switch c.Version() {
	case V311:
		return c.write(w, first, varHeader, c.payload())
	case V5:
		varHeader[6] = byte(V5)
		return c.writeV5(w, first, varHeader, c.Props.props(), c.payload())
	default:
		return ErrUnsupportedVersion
	}
}

func (c *ConnPacket) clone() *ConnPacket {
	willMessageCopy := make([]byte, len(c.WillMessage))
	_ = copy(willMessageCopy, c.WillMessage)

	return &ConnPacket{
		BasePacket: BasePacket{
			ProtoVersion: c.ProtoVersion,
		},
		ProtoName:    c.ProtoName,
		CleanSession: c.CleanSession,
		IsWill:       c.IsWill,
		WillQos:      c.WillQos,
		WillRetain:   c.WillRetain,
		Username:     c.Username,
		Password:     c.Password,
		ClientID:     c.ClientID,
		Keepalive:    c.Keepalive,
		WillTopic:    c.WillTopic,
		WillMessage:  willMessageCopy,
		Props:        c.Props.clone(),
	}
}

func (c *ConnPacket) flags() byte {
	var flag byte
	if c.ClientID == "" {
		c.CleanSession = true
	}

	if c.CleanSession {
		flag |= 0x02
	}

	if c.IsWill {
		flag |= 0x04
		flag |= c.WillQos << 3

		if c.WillRetain {
			flag |= 0x20
		}
	}

	if c.Password != "" {
		flag |= 0x40
	}

	if c.Username != "" {
		flag |= 0x80
	}

	return flag
}

func (c *ConnPacket) payload() []byte {
	// client id
	result := encodeStringWithLen(c.ClientID)

	if c.IsWill {
		// will properties
		if c.ProtoVersion == V5 {
			if c.WillProps == nil {
				result = append(result, 0)
			} else {
				buf := new(bytes.Buffer)
				willProps := c.WillProps.props()
				_ = writeVarInt(len(willProps), buf)
				result = append(result, buf.Bytes()...)
				result = append(result, willProps...)
			}
		}

		// will topic and message
		result = append(result, encodeStringWithLen(c.WillTopic)...)
		result = append(result, encodeBytesWithLen(c.WillMessage)...)
	}

	if c.Username != "" {
		result = append(result, encodeStringWithLen(c.Username)...)
	}

	if c.Password != "" {
		result = append(result, encodeStringWithLen(c.Password)...)
	}

	return result
}

// ConnProps defines connect packet properties
type ConnProps struct {
	// If the Session Expiry Interval is absent the value 0 is used.
	// If it is set to 0, or is absent, the Session ends when the Network Connection is closed.
	// If the Session Expiry Interval is 0xFFFFFFFF (UINT_MAX), the Session does not expire.
	SessionExpiryInterval uint32

	// The Client uses this value to limit the number of QoS 1 and QoS 2 publications
	// that it is willing to process concurrently.
	//
	// There is no mechanism to limit the QoS 0 publications that the Server might try to send.
	//
	// The value of Receive Maximum applies only to the current Network Connection.
	// If the Receive Maximum value is absent then its value defaults to 65,535
	MaxRecv uint16

	// The Maximum Packet Size the Client is willing to accept
	//
	// If the Maximum Packet Size is not present,
	// no limit on the packet size is imposed beyond the limitations in the protocol as a
	// result of the remaining length encoding and the protocol header sizes
	MaxPacketSize uint32

	// This value indicates the highest value that the Client will accept
	// as a Topic Alias sent by the Server.
	//
	// The Client uses this value to limit the number of Topic Aliases that
	// it is willing to hold on this Connection.
	MaxTopicAlias uint16

	// The Client uses this value to request the Server to return Response
	// Information in the ConnAckPacket
	ReqRespInfo *bool

	// The Client uses this value to indicate whether the Reason String
	// or User Properties are sent in the case of failures.
	ReqProblemInfo *bool

	// User defined Properties
	UserProps UserProps

	// If Authentication Method is absent, extended authentication is not performed.
	//
	// If a Client sets an Authentication Method in the ConnPacket,
	// the Client MUST NOT send any packets other than AuthPacket or DisConn packets
	// until it has received a ConnAck packet
	AuthMethod string

	// The contents of this data are defined by the authentication method.
	AuthData []byte

	mu sync.RWMutex
}

func (c *ConnProps) props() []byte {
	if c == nil {
		return nil
	}

	p := propertySet{}
	p.set(propKeySessionExpiryInterval, c.SessionExpiryInterval)
	p.set(propKeyMaxRecv, c.MaxRecv)
	p.set(propKeyMaxPacketSize, c.MaxPacketSize)
	p.set(propKeyMaxTopicAlias, c.MaxTopicAlias)
	p.set(propKeyReqRespInfo, c.ReqRespInfo)
	p.set(propKeyReqProblemInfo, c.ReqProblemInfo)
	p.set(propKeyUserProps, c.UserProps)
	p.set(propKeyAuthMethod, c.AuthMethod)
	p.set(propKeyAuthData, c.AuthData)
	return p.bytes()
}

func (c *ConnProps) clone() *ConnProps {
	if c == nil {
		return nil
	}
	c.mu.RUnlock()
	defer c.mu.RUnlock()

	authDataCopy := make([]byte, len(c.AuthData))
	_ = copy(authDataCopy, c.AuthData)

	userPropsCopy := make(UserProps)
	for k, v := range c.UserProps {
		buf := make([]string, len(v))
		_ = copy(buf, v)
		userPropsCopy[k] = buf
	}

	return &ConnProps{
		SessionExpiryInterval: c.SessionExpiryInterval,
		MaxRecv:               c.MaxRecv,
		MaxPacketSize:         c.MaxPacketSize,
		MaxTopicAlias:         c.MaxTopicAlias,
		ReqRespInfo:           c.ReqRespInfo,
		ReqProblemInfo:        c.ReqProblemInfo,
		UserProps:             c.UserProps,
		AuthMethod:            c.AuthMethod,
		AuthData:              authDataCopy,
	}
}

func (c *ConnProps) setProps(props map[byte][]byte) {
	if c == nil {
		return
	}

	if v, ok := props[propKeySessionExpiryInterval]; ok {
		c.SessionExpiryInterval = getUint32(v)
	}

	if v, ok := props[propKeyMaxRecv]; ok {
		c.MaxRecv = getUint16(v)
	}

	if v, ok := props[propKeyMaxPacketSize]; ok {
		c.MaxPacketSize = getUint32(v)
	}

	if v, ok := props[propKeyMaxTopicAlias]; ok {
		c.MaxTopicAlias = getUint16(v)
	}

	if v, ok := props[propKeyReqRespInfo]; ok && len(v) == 1 {
		b := v[0] == 1
		c.ReqRespInfo = &b
	}

	if v, ok := props[propKeyReqProblemInfo]; ok && len(v) == 1 {
		b := v[0] == 1
		c.ReqProblemInfo = &b
	}

	if v, ok := props[propKeyUserProps]; ok {
		c.UserProps = getUserProps(v)
	}

	if v, ok := props[propKeyAuthMethod]; ok {
		c.AuthMethod, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthData]; ok {
		c.AuthData, _, _ = getBinaryData(v)
	}
}

// ConnAckPacket is the packet sent by the Server in response to a ConnPacket
// received from a Client.
//
// The first packet sent from the Server to the Client MUST be a ConnAckPacket
type ConnAckPacket struct {
	BasePacket
	Present bool
	Code    byte
	Props   *ConnAckProps
}

// Type ConnAckPacket's type is CtrlConnAck
func (c *ConnAckPacket) Type() CtrlType {
	return CtrlConnAck
}

func (c *ConnAckPacket) Bytes() []byte {
	if c == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = c.WriteTo(w)
	return w.Bytes()
}

func (c *ConnAckPacket) WriteTo(w BufferedWriter) error {
	if c == nil {
		return ErrEncodeBadPacket
	}

	switch c.Version() {
	case V311:
		_, err := w.Write([]byte{CtrlConnAck << 4, 2, boolToByte(c.Present), c.Code})
		return err
	case V5:
		return c.writeV5(w, CtrlConnAck<<4, []byte{boolToByte(c.Present), c.Code}, c.Props.props(), nil)
	default:
		return ErrUnsupportedVersion
	}
}

// ConnAckProps defines connect acknowledge properties
type ConnAckProps struct {
	// The contents of this data are defined by the authentication method.
	AuthData []byte

	// The Client Identifier which was assigned by the Server
	// because a zero length Client Identifier was found in the ConnPacket
	AssignedClientID string

	// Human readable string designed for diagnostics
	Reason string

	// Response Information
	RespInfo string

	// Can be used by the Client to identify another Server to use
	ServerRef string

	// The name of the authentication method
	AuthMethod string

	// Declares whether the Server supports retained messages.
	// true means that retained messages are not supported.
	// false means retained messages are supported
	RetainAvail *bool

	// User defines Properties
	UserProps UserProps

	// Whether the Server supports Wildcard Subscriptions.
	// false means that Wildcard Subscriptions are not supported.
	// true means Wildcard Subscriptions are supported.
	//
	// default is true
	WildcardSubAvail *bool // 40

	// Whether the Server supports Subscription Identifiers.
	// false means that Subscription Identifiers are not supported.
	// true means Subscription Identifiers are supported.
	//
	// default is true
	SubIDAvail *bool

	// Whether the Server supports Shared Subscriptions.
	// false means that Shared Subscriptions are not supported.
	// true means Shared Subscriptions are supported
	//
	// default is true
	SharedSubAvail *bool

	// If the Session Expiry Interval is absent the value in the ConnPacket used.
	// The server uses this property to inform the Client that it is using
	// a value other than that sent by the Client in the ConnAck
	SessionExpiryInterval uint32

	// Maximum Packet Size the Server is willing to accept.
	// If the Maximum Packet Size is not present, there is no limit on the
	// packet size imposed beyond the limitations in the protocol as a
	// result of the remaining length encoding and the protocol header sizes
	MaxPacketSize uint32

	// The Server uses this value to limit the number of QoS 1 and QoS 2 publications
	// that it is willing to process concurrently for the Client.
	//
	// It does not provide a mechanism to limit the QoS 0 publications that
	// the Client might try to send
	MaxRecv uint16

	// This value indicates the highest value that the Server will accept
	// as a Topic Alias sent by the Client.
	//
	// The Server uses this value to limit the number of Topic Aliases
	// that it is willing to hold on this Connection.
	MaxTopicAlias uint16

	// Keep Alive time assigned by the Server
	ServerKeepalive uint16

	MaxQos QosLevel
}

func (c *ConnAckProps) props() []byte {
	if c == nil {
		return nil
	}

	p := propertySet{}
	p.set(propKeyRetainAvail, c.RetainAvail)
	p.set(propKeyWildcardSubAvail, c.WildcardSubAvail)
	p.set(propKeySubIDAvail, c.SubIDAvail)
	p.set(propKeySharedSubAvail, c.SharedSubAvail)
	p.set(propKeySessionExpiryInterval, c.SessionExpiryInterval)
	p.set(propKeyMaxRecv, c.MaxRecv)
	p.set(propKeyMaxQos, c.MaxQos)
	p.set(propKeyMaxPacketSize, c.MaxPacketSize)
	p.set(propKeyAssignedClientID, c.AssignedClientID)
	p.set(propKeyMaxTopicAlias, c.MaxTopicAlias)
	p.set(propKeyReasonString, c.Reason)
	p.set(propKeyUserProps, c.UserProps)
	p.set(propKeyServerKeepalive, c.ServerKeepalive)
	p.set(propKeyRespInfo, c.RespInfo)
	p.set(propKeyServerRef, c.ServerRef)
	p.set(propKeyAuthMethod, c.AuthMethod)
	p.set(propKeyAuthData, c.AuthData)
	return p.bytes()
}

func (c *ConnAckProps) setProps(props map[byte][]byte) {
	if v, ok := props[propKeySessionExpiryInterval]; ok {
		c.SessionExpiryInterval = getUint32(v)
	}

	if v, ok := props[propKeyMaxRecv]; ok {
		c.MaxRecv = getUint16(v)
	}

	if v, ok := props[propKeyMaxQos]; ok && len(v) == 1 {
		c.MaxQos = v[0]
	}

	if v, ok := props[propKeyRetainAvail]; ok && len(v) == 1 {
		b := v[0] == 1
		c.RetainAvail = &b
	}

	if v, ok := props[propKeyMaxPacketSize]; ok {
		c.MaxPacketSize = getUint32(v)
	}

	if v, ok := props[propKeyAssignedClientID]; ok {
		c.AssignedClientID, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyMaxTopicAlias]; ok {
		c.MaxTopicAlias = getUint16(v)
	}

	if v, ok := props[propKeyReasonString]; ok {
		c.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		c.UserProps = getUserProps(v)
	}

	if v, ok := props[propKeyWildcardSubAvail]; ok && len(v) == 1 {
		b := v[0] == 1
		c.WildcardSubAvail = &b
	}

	if v, ok := props[propKeyServerKeepalive]; ok {
		c.ServerKeepalive = getUint16(v)
	}

	if v, ok := props[propKeyRespInfo]; ok {
		c.RespInfo, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyServerRef]; ok {
		c.ServerRef, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthMethod]; ok {
		c.AuthMethod, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthData]; ok {
		c.AuthData, _, _ = getBinaryData(v)
	}
}

type DisConnPacket = DisconnPacket

// DisconnPacket is the final Control Packet sent from the Client to the Server.
// It indicates that the Client is disconnecting cleanly.
type DisconnPacket struct {
	BasePacket
	Code  byte
	Props *DisconnProps
}

// Type of DisconnPacket is CtrlDisConn
func (d *DisconnPacket) Type() CtrlType {
	return CtrlDisConn
}

func (d *DisconnPacket) Bytes() []byte {
	if d == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = d.WriteTo(w)
	return w.Bytes()
}

func (d *DisconnPacket) WriteTo(w BufferedWriter) error {
	if d == nil {
		return ErrEncodeBadPacket
	}

	var (
		err error
	)

	switch d.Version() {
	case V311:
		_, err = w.Write([]byte{CtrlDisConn << 4, 0})
		return err
	case V5:
		return d.writeV5(w, CtrlDisConn<<4, nil, d.Props.props(), nil)
	default:
		return ErrUnsupportedVersion
	}
}

type DisConnProps = DisconnPacket

// DisConnProps properties for DisconnPacket
type DisconnProps struct {
	// Session Expiry Interval in seconds
	// If the Session Expiry Interval is absent, the Session Expiry Interval in the CONNECT packet is used
	//
	// The Session Expiry Interval MUST NOT be sent on a DISCONNECT by the Server
	SessionExpiryInterval uint32

	// Human readable, designed for diagnostics and SHOULD NOT be parsed by the receiver
	Reason string

	// User defines Properties
	UserProps UserProps

	// Used by the Client to identify another Server to use
	ServerRef string
}

func (d *DisconnProps) props() []byte {
	if d == nil {
		return nil
	}

	p := &propertySet{}
	p.set(propKeySessionExpiryInterval, d.SessionExpiryInterval)
	p.set(propKeyReasonString, d.Reason)
	p.set(propKeyUserProps, d.UserProps)
	p.set(propKeyServerRef, d.ServerRef)
	return p.bytes()
}

func (d *DisconnProps) setProps(props map[byte][]byte) {
	if d == nil {
		return
	}

	if v, ok := props[propKeySessionExpiryInterval]; ok {
		d.SessionExpiryInterval = getUint32(v)
	}

	if v, ok := props[propKeyReasonString]; ok {
		d.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		d.UserProps = getUserProps(v)
	}

	if v, ok := props[propKeyServerRef]; ok {
		d.ServerRef, _, _ = getStringData(v)
	}
}
