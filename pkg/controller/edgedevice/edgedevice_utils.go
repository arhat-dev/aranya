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

package edgedevice

import (
	"crypto/tls"
	"fmt"
	"strings"

	"arhat.dev/aranya-proto/aranyagopb"
	corev1 "k8s.io/api/core/v1"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func (c *Controller) createDeviceConnector(
	spec aranyaapi.PeripheralConnectivitySpec,
) (_ *aranyagopb.Connectivity, _ error) {
	var tlsConfig *aranyagopb.TLSConfig

	if t := spec.TLS; t != nil {
		var cs []uint32

		for _, s := range t.CipherSuites {
			sv, ok := cipherSuites[s]
			if !ok {
				return nil, fmt.Errorf("unsupported cipher suite: %q", s)
			}

			cs = append(cs, uint32(sv))
		}

		min, ok := tlsVersions[strings.ToUpper(t.MinVersion)]
		if !ok {
			min = tls.VersionTLS12
		}

		max, ok := tlsVersions[strings.ToUpper(t.MaxVersion)]
		if !ok {
			max = tls.VersionTLS13
		}

		var (
			certBytes, keyBytes, caBytes []byte
		)
		if certRef := t.CertSecretRef; certRef != nil {
			secret, found := c.getWatchSecretObject(certRef.Name)
			if !found {
				return nil, fmt.Errorf("failed to retrieve connectivity tls cert data from secret %q", secret)
			}

			certBytes, _ = accessMap(secret.StringData, secret.Data, corev1.TLSCertKey)
			keyBytes, _ = accessMap(secret.StringData, secret.Data, corev1.TLSPrivateKeyKey)
			caBytes, _ = accessMap(secret.StringData, secret.Data, constant.TLSCaCertKey)
		}

		tlsConfig = &aranyagopb.TLSConfig{
			ServerName:         t.ServerName,
			InsecureSkipVerify: t.InsecureSkipVerify,
			MinVersion:         uint32(min),
			MaxVersion:         uint32(max),
			CipherSuites:       cs,
			NextProtos:         t.NextProtos,

			Cert:   certBytes,
			Key:    keyBytes,
			CaCert: caBytes,
		}
	}

	return &aranyagopb.Connectivity{
		Method: spec.Method,
		Target: spec.Target,
		Params: spec.Params,
		Tls:    tlsConfig,
	}, nil
}

var tlsVersions = map[string]uint16{
	"TLS10": tls.VersionTLS10,
	"TLS11": tls.VersionTLS11,
	"TLS12": tls.VersionTLS12,
	"TLS13": tls.VersionTLS13,
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
