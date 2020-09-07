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

// AuthPacket Client <-> Server
// as part of an extended authentication exchange,
// such as challenge / response authentication.
//
// It is a Protocol Error for the Client or Server to send
// an AUTH packet if the ConnPacket did not contain the
// same Authentication Method
type AuthPacket struct {
	BasePacket
	Code  byte       // the authentication result code
	Props *AuthProps // authentication properties
}

// Type of AuthPacket is CtrlAuth
func (a *AuthPacket) Type() CtrlType {
	return CtrlAuth
}

func (a *AuthPacket) Bytes() []byte {
	if a == nil {
		return nil
	}

	w := new(bytes.Buffer)
	_ = a.WriteTo(w)
	return w.Bytes()
}

func (a *AuthPacket) WriteTo(w BufferedWriter) error {
	if a == nil {
		return ErrEncodeBadPacket
	}

	return a.writeV5(w, CtrlAuth<<4, []byte{a.Code}, a.Props.props(), nil)
}

// AuthProps properties of AuthPacket
type AuthProps struct {
	AuthMethod string
	AuthData   []byte
	Reason     string
	UserProps  UserProps
}

func (a *AuthProps) props() []byte {
	if a == nil {
		return nil
	}

	p := propertySet{}
	p.set(propKeyAuthMethod, a.AuthMethod)
	p.set(propKeyAuthData, a.AuthData)
	p.set(propKeyReasonString, a.Reason)
	p.set(propKeyUserProps, a.UserProps)
	return p.bytes()
}

func (a *AuthProps) setProps(props map[byte][]byte) {
	if a == nil {
		return
	}

	if v, ok := props[propKeyAuthMethod]; ok {
		a.AuthMethod, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyAuthData]; ok {
		a.AuthData, _, _ = getBinaryData(v)
	}

	if v, ok := props[propKeyReasonString]; ok {
		a.Reason, _, _ = getStringData(v)
	}

	if v, ok := props[propKeyUserProps]; ok {
		a.UserProps = getUserProps(v)
	}
}
