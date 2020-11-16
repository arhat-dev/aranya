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

package tlshelper

type TLSPreSharedKeyConfig struct {
	// map server hint(s) to pre shared key(s)
	// colon separated base64 encoded key value pairs
	ServerHintMapping []string `json:"serverHintMapping" yaml:"serverHintMapping"`
	// the client hint provided to server, base64 encoded value
	IdentityHint string `json:"identityHint" yaml:"identityHint"`
}

// nolint:maligned
type TLSConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	CaCert string `json:"caCert" yaml:"caCert"`
	Cert   string `json:"cert" yaml:"cert"`
	Key    string `json:"key" yaml:"key"`

	CaCertData string `json:"caCertData" yaml:"caCertData"`
	CertData   string `json:"certData" yaml:"certData"`
	KeyData    string `json:"keyData" yaml:"keyData"`

	ServerName         string `json:"serverName" yaml:"serverName"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify" yaml:"insecureSkipVerify"`
	// write tls session shared key to this file
	KeyLogFile   string   `json:"keyLogFile" yaml:"keyLogFile"`
	CipherSuites []string `json:"cipherSuites" yaml:"cipherSuites"`

	// options for dtls
	AllowInsecureHashes bool `json:"allowInsecureHashes" yaml:"allowInsecureHashes"`

	PreSharedKey TLSPreSharedKeyConfig `json:"preSharedKey" yaml:"preSharedKey"`
}
