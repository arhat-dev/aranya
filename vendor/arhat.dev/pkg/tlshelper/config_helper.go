// +build !noconfhelper_tls

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
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

var CipherSuites = map[string]uint16{
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

	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,

	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,

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
		if code, ok := CipherSuites[strings.ToUpper(c)]; ok {
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
