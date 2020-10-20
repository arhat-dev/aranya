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

import "bytes"

// SubscribePacket is sent from the Client to the Server
// to create one or more Subscriptions.
//
// Each Subscription registers a Client's interest in one or more TopicNames.
// The Server sends PublishPackets to the Client in order to forward
// Application Messages that were published to TopicNames that match these Subscriptions.
// The SubscribePacket also specifies (for each Subscription)
// the maximum QoS with which the Server can send Application Messages to the Client
type SubscribePacket struct {
	BasePacket
	PacketID uint16
	Topics   []*Topic
	Props    *SubscribeProps
}

// Type of SubscribePacket is CtrlSubscribe
func (s *SubscribePacket) Type() CtrlType {
	return CtrlSubscribe
}

func (s *SubscribePacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = s.WriteTo(w)
	return w.Bytes()
}

func (s *SubscribePacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	const first = CtrlSubscribe<<4 | 0x02
	varHeader := []byte{byte(s.PacketID >> 8), byte(s.PacketID)}
	switch s.Version() {
	case V311:
		return s.write(w, first, varHeader, s.payload())
	case V5:
		return s.writeV5(w, first, varHeader, s.Props.props(), s.payload())
	default:
		return ErrUnsupportedVersion
	}
}

func (s *SubscribePacket) payload() []byte {
	var result []byte
	if s.Topics != nil {
		for _, t := range s.Topics {
			result = append(result, encodeStringWithLen(t.Name)...)
			result = append(result, t.Qos)
		}
	}
	return result
}

// SubscribeProps properties for SubscribePacket
type SubscribeProps struct {
	// SubID identifier of the subscription
	SubID int
	// UserProps User defined Properties
	UserProps UserProps
}

func (s *SubscribeProps) props() []byte {
	if s == nil {
		return nil
	}

	var result []byte
	if s.SubID != 0 {
		subIDBytes, _ := varIntBytes(s.SubID)
		result = append(result, propKeySubID)
		result = append(result, subIDBytes...)
	}

	if len(s.UserProps) != 0 {
		result = append(result, propKeyUserProps)
		s.UserProps.encodeTo(&result)
	}
	return result
}

func (s *SubscribeProps) setProps(props map[byte][]byte) {
	if s == nil || props == nil {
		return
	}

	if v, ok := props[propKeySubID]; ok {
		id, _ := getRemainLength(bytes.NewReader(v))
		s.SubID = id
	}

	if v, ok := props[propKeyUserProps]; ok {
		s.UserProps = getUserProps(v)
	}
}

// SubAckPacket is sent by the Server to the Client
// to confirm receipt and processing of a SubscribePacket.
//
// SubAckPacket contains a list of return codes,
// that specify the maximum QoS level that was granted in
// each Subscription that was requested by the SubscribePacket.
type SubAckPacket struct {
	BasePacket
	PacketID uint16
	Codes    []byte
	Props    *SubAckProps
}

// Type of SubAckPacket is CtrlSubAck
func (s *SubAckPacket) Type() CtrlType {
	return CtrlSubAck
}

func (s *SubAckPacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = s.WriteTo(w)
	return w.Bytes()
}

func (s *SubAckPacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	const first = CtrlSubAck << 4
	varHeader := []byte{byte(s.PacketID >> 8), byte(s.PacketID)}
	switch s.Version() {
	case V311:
		return s.write(w, first, varHeader, s.payload())
	case V5:
		return s.writeV5(w, first, varHeader, s.Props.props(), s.payload())
	default:
		return ErrUnsupportedVersion
	}
}

func (s *SubAckPacket) payload() []byte {
	return s.Codes
}

// SubAckProps properties for SubAckPacket
type SubAckProps struct {
	// Human readable string designed for diagnostics
	Reason string

	// UserProps User defined Properties
	UserProps UserProps
}

func (p *SubAckProps) props() []byte {
	if p == nil {
		return nil
	}

	propSet := propertySet{}
	if p.Reason != "" {
		propSet.set(propKeyReasonString, p.Reason)
	}

	if p.UserProps != nil {
		propSet.set(propKeyUserProps, p.UserProps)
	}
	return propSet.bytes()
}

func (p *SubAckProps) setProps(props map[byte][]byte) {
	if p == nil || props == nil {
		return
	}

	if v, ok := props[propKeyReasonString]; ok {
		p.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		p.UserProps = getUserProps(v)
	}
}

type UnSubPacket = UnsubPacket

// UnsubPacket is sent by the Client to the Server,
// to unsubscribe from topics.
type UnsubPacket struct {
	BasePacket
	PacketID   uint16
	TopicNames []string
	Props      *UnsubProps
}

// Type of UnsubPacket is CtrlUnSub
func (s *UnsubPacket) Type() CtrlType {
	return CtrlUnSub
}

func (s *UnsubPacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = s.WriteTo(w)
	return w.Bytes()
}

func (s *UnsubPacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	const first = CtrlUnSub<<4 | 0x02
	varHeader := []byte{byte(s.PacketID >> 8), byte(s.PacketID)}
	switch s.Version() {
	case V311:
		return s.write(w, first, varHeader, s.payload())
	case V5:
		return s.writeV5(w, first, varHeader, s.Props.props(), s.payload())
	default:
		return ErrUnsupportedVersion
	}
}

func (s *UnsubPacket) payload() []byte {
	var result []byte
	if s.TopicNames != nil {
		for _, t := range s.TopicNames {
			result = append(result, encodeStringWithLen(t)...)
		}
	}
	return result
}

type UnSubProps = UnsubProps

// UnsubProps properties for UnsubPacket
type UnsubProps struct {
	// UserProps User defined Properties
	UserProps UserProps
}

func (p *UnsubProps) props() []byte {
	if p == nil {
		return nil
	}

	propSet := propertySet{}
	if p.UserProps != nil {
		propSet.set(propKeyUserProps, p.UserProps)
	}
	return propSet.bytes()
}

func (p *UnsubProps) setProps(props map[byte][]byte) {
	if p == nil || props == nil {
		return
	}

	if v, ok := props[propKeyUserProps]; ok {
		p.UserProps = getUserProps(v)
	}
}

type UnSubAckPacket = UnsubAckPacket

// UnsubAckPacket is sent by the Server to the Client to confirm
// receipt of an UnsubPacket
type UnsubAckPacket struct {
	BasePacket
	PacketID uint16
	Props    *UnsubAckProps
}

// Type of UnsubAckPacket is CtrlUnSubAck
func (s *UnsubAckPacket) Type() CtrlType {
	return CtrlUnSubAck
}

func (s *UnsubAckPacket) Bytes() []byte {
	if s == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = s.WriteTo(w)
	return w.Bytes()
}

func (s *UnsubAckPacket) WriteTo(w BufferedWriter) error {
	if s == nil {
		return ErrEncodeBadPacket
	}

	const first = CtrlUnSubAck << 4
	varHeader := []byte{byte(s.PacketID >> 8), byte(s.PacketID)}
	switch s.Version() {
	case V311:
		return s.write(w, first, varHeader, nil)
	case V5:
		return s.writeV5(w, first, varHeader, s.Props.props(), nil)
	default:
		return ErrUnsupportedVersion
	}
}

type UnSubAckProps = UnsubAckProps

// UnsubAckProps properties for UnsubAckPacket
type UnsubAckProps struct {
	// Human readable string designed for diagnostics
	Reason string

	// UserProps User defined Properties
	UserProps UserProps
}

func (p *UnsubAckProps) props() []byte {
	if p == nil {
		return nil
	}

	propSet := propertySet{}
	if p.Reason != "" {
		propSet.set(propKeyReasonString, p.Reason)
	}

	if p.UserProps != nil {
		propSet.set(propKeyUserProps, p.UserProps)
	}
	return propSet.bytes()
}

func (p *UnsubAckProps) setProps(props map[byte][]byte) {
	if p == nil || props == nil {
		return
	}

	if v, ok := props[propKeyReasonString]; ok {
		p.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		p.UserProps = getUserProps(v)
	}
}
