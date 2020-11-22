/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hashhelper

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
)

func Sha256SumHex(data []byte) string {
	h := sha256.New()
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func Sha512SumHex(data []byte) string {
	h := sha512.New()
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func MD5SumHex(data []byte) string {
	h := md5.New()
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
