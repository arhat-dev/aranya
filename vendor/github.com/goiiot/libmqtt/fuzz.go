// +build gofuzz

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
)

func Fuzz(data []byte) int {
	pkt, err := Decode(V311, bytes.NewReader(data))
	if err != nil {
		if pkt != nil {
			panic("pkt != nil on error")
		}
		return 0
	}

	pkt, err = Decode(V5, bytes.NewReader(data))
	if err != nil {
		if pkt != nil {
			panic("pkt != nil on error")
		}
		return 0
	}
	return 1
}
