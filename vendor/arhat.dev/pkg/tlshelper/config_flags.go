// +build !noflaghelper

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

import (
	"github.com/spf13/pflag"
)

func FlagsForTLSConfig(prefix string, config *TLSConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("tls.config", pflag.ExitOnError)

	fs.BoolVar(&config.Enabled, prefix+"enabled", false, "enable tls")
	fs.BoolVar(&config.InsecureSkipVerify, prefix+"insecureSkipVerify", false, "allow insecure tls certs")
	fs.StringVar(&config.CaCert, prefix+"caCert", "", "set path to ca cert")
	fs.StringVar(&config.Cert, prefix+"cert", "", "set path to cert")
	fs.StringVar(&config.Key, prefix+"key", "", "set path to private key file")
	fs.StringVar(&config.ServerName, prefix+"serverName", "", "override server name")
	fs.StringVar(&config.KeyLogFile, prefix+"keyLogFile", "", "set path to a file to write tls session key for debug")
	fs.StringSliceVar(&config.CipherSuites, prefix+"cipherSuites", nil, "set acceptable cipher suites")

	fs.BoolVar(&config.AllowInsecureHashes, prefix+"allowInsecureHashes", false, "allow insecure dtls hash functions")
	fs.StringVar(&config.PreSharedKey.IdentityHint, prefix+"preSharedKey.identityHint", "",
		"set identity hint for pre shared key")
	fs.StringSliceVar(&config.PreSharedKey.ServerHintMapping, prefix+"preSharedKey.serverHintMapping", nil,
		"set server hint to key mapping")

	return fs
}
