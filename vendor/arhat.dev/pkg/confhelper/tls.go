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

package confhelper

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

func FlagsForTLSConfig(prefix string, config *TLSConfig) *pflag.FlagSet {
	fs := pflag.NewFlagSet("tlsConfig", pflag.ExitOnError)

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

var cipherSuites = map[string]uint16{
	"TLS_RSA_WITH_RC4_128_SHA":                tls.TLS_RSA_WITH_RC4_128_SHA,
	"TLS_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"TLS_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":        tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_RC4_128_SHA":          tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":     tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":    tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":  tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

	// TLS 1.3 cipher suites.
	"TLS_AES_128_GCM_SHA256":       tls.TLS_AES_128_GCM_SHA256,
	"TLS_AES_256_GCM_SHA384":       tls.TLS_AES_256_GCM_SHA384,
	"TLS_CHACHA20_POLY1305_SHA256": tls.TLS_CHACHA20_POLY1305_SHA256,

	// TLS_FALLBACK_SCSV isn't a standard cipher suite but an indicator
	// that the client is doing version fallback. See RFC 7507.
	"TLS_FALLBACK_SCSV": tls.TLS_FALLBACK_SCSV,

	//
	//
	// pion/dtls supported cipher suites
	//
	//

	// AES-128-CCM
	"TLS_ECDHE_ECDSA_WITH_AES_128_CCM":   0xc0ac,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CCM_8": 0xc0ae,

	// AES-128-GCM-SHA256
	//"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256": 0xc02b,
	//"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256": 0xc02f,

	// AES-256-CBC-SHA
	//"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA": 0xc00a,
	//"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA": 0xc014,

	"TLS_PSK_WITH_AES_128_CCM":        0xc0a4,
	"TLS_PSK_WITH_AES_128_CCM_8":      0xc0a8,
	"TLS_PSK_WITH_AES_128_GCM_SHA256": 0x00a8,
}

type TLSPreSharedKeyConfig struct {
	// map server hint(s) to pre shared key(s)
	// column separated base64 encoded key value pairs
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

type oneTimeWriter struct {
	file string
}

func (w oneTimeWriter) Write(data []byte) (int, error) {
	f, err := os.OpenFile(w.file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}

	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}

	if err1 := f.Close(); err == nil {
		err = err1
	}

	return n, err
}

func (c TLSConfig) GetTLSConfig(server bool) (_ *tls.Config, err error) {
	if !c.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		ServerName:         c.ServerName,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}

	for _, c := range c.CipherSuites {
		if code, ok := cipherSuites[strings.ToUpper(c)]; ok {
			tlsConfig.CipherSuites = append(tlsConfig.CipherSuites, code)
		} else {
			return nil, fmt.Errorf("unsupported cipher suite: %s", c)
		}
	}

	if c.CaCert != "" || c.CaCertData != "" {
		var caBytes []byte
		if c.CaCert != "" {
			caBytes, err = ioutil.ReadFile(c.CaCert)
			if err != nil {
				return nil, fmt.Errorf("failed to read caCert: %w", err)
			}
		} else {
			caBytes = []byte(c.CaCertData)
		}

		tlsConfig.RootCAs = x509.NewCertPool()
		block, _ := pem.Decode(caBytes)
		if block == nil {
			// not encoded in pem format
			var caCerts []*x509.Certificate
			caCerts, err = x509.ParseCertificates(caBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ca certs: %w", err)
			}
			for i := range caCerts {
				if server {
					tlsConfig.ClientCAs.AddCert(caCerts[i])
				} else {
					tlsConfig.RootCAs.AddCert(caCerts[i])
				}
			}
		} else {
			if server {
				if !tlsConfig.ClientCAs.AppendCertsFromPEM(caBytes) {
					return nil, fmt.Errorf("failed to add pem encoded client ca certs")
				}
			} else {
				if !tlsConfig.RootCAs.AppendCertsFromPEM(caBytes) {
					return nil, fmt.Errorf("failed to add pem encoded ca certs")
				}
			}
		}
	}

	if c.KeyLogFile != "" {
		err = os.Remove(c.KeyLogFile)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to cleanup key log file: %w", err)
		}

		tlsConfig.KeyLogWriter = &oneTimeWriter{file: c.KeyLogFile}
	}

	var certBytes, keyBytes []byte
	if c.Cert != "" {
		certBytes, err = ioutil.ReadFile(c.Cert)
		if err != nil {
			return nil, fmt.Errorf("failed to load cert: %w", err)
		}
	} else if c.CertData != "" {
		certBytes, err = base64.StdEncoding.DecodeString(c.CertData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode cert data (base64): %w", err)
		}
	}

	if c.Key != "" {
		keyBytes, err = ioutil.ReadFile(c.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to load key: %w", err)
		}
	} else {
		keyBytes, err = base64.StdEncoding.DecodeString(c.KeyData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode key data (base64): %w", err)
		}
	}

	if len(keyBytes) != 0 && len(certBytes) != 0 {
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to create x509 pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
