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
	"errors"
)

var (
	// ErrUnsupportedVersion unsupported mqtt ProtoVersion
	ErrUnsupportedVersion = errors.New("trying encode/decode packet with unsupported MQTT version ")

	// ErrEncodeBadPacket happens when trying to encode none MQTT packet
	ErrEncodeBadPacket = errors.New("trying encode none MQTT packet ")

	// ErrEncodeLargePacket happens when MQTT packet is too large according to MQTT spec
	ErrEncodeLargePacket = errors.New("MQTT packet too large")
)

// Encode MQTT packet to bytes according to protocol ProtoVersion
func Encode(packet Packet, w BufferedWriter) error {
	return packet.WriteTo(w)
}
