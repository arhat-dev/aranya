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
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"strings"
)

func generatePEMEncodedCSR(template *x509.CertificateRequest, privateKey interface{}) ([]byte, error) {
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	}), nil
}

func parsePEMEncodedCSR(in []byte) (csr *x509.CertificateRequest, err error) {
	in = bytes.TrimSpace(in)
	p, _ := pem.Decode(in)
	if p != nil {
		if p.Type != "NEW CERTIFICATE REQUEST" && p.Type != "CERTIFICATE REQUEST" {
			return nil, fmt.Errorf("unexpected non csr pem block")
		}

		csr, err = x509.ParseCertificateRequest(p.Bytes)
	} else {
		csr, err = x509.ParseCertificateRequest(in)
	}

	if err != nil {
		return nil, err
	}

	return csr, nil
}

func parsePEMEncodedPrivateKey(keyPEM, password []byte) (key crypto.Signer, err error) {
	p, _ := pem.Decode(keyPEM)
	if p == nil {
		return nil, fmt.Errorf("no private key block found")
	}

	var keyDER []byte
	if procType, ok := p.Headers["Proc-Type"]; ok && strings.Contains(procType, "ENCRYPTED") {
		if len(password) != 0 {
			keyDER, err = x509.DecryptPEMBlock(p, password)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt pem block: %w", err)
			}
		} else {
			return nil, fmt.Errorf("private key password not provided")
		}
	} else {
		keyDER = p.Bytes
	}

	generalKey, err := x509.ParsePKCS8PrivateKey(keyDER)
	if err != nil {
		generalKey, err = x509.ParsePKCS1PrivateKey(keyDER)
		if err != nil {
			generalKey, err = x509.ParseECPrivateKey(keyDER)
			if err != nil {
				// oneAsymmetricKey reflects the ASN.1 structure for storing private keys in
				// https://tools.ietf.org/html/draft-ietf-curdle-pkix-04, excluding the optional
				// fields, which we don't use here.
				//
				// This is identical to pkcs8 in crypto/x509.
				type oneAsymmetricKey struct {
					Version    int
					Algorithm  pkix.AlgorithmIdentifier
					PrivateKey []byte
				}

				// curvePrivateKey is the innter type of the PrivateKey field of
				// oneAsymmetricKey.
				type curvePrivateKey []byte

				var ed25519OID = asn1.ObjectIdentifier{1, 3, 101, 112}

				// ParseEd25519PrivateKey returns the Ed25519 private key encoded by the input.
				parseEd25519PrivateKey := func(der []byte) (crypto.PrivateKey, error) {
					asym := new(oneAsymmetricKey)
					var rest []byte
					rest, err = asn1.Unmarshal(der, asym)
					if err != nil {
						return nil, err
					} else if len(rest) > 0 {
						return nil, fmt.Errorf("oneAsymmetricKey too long")
					}

					// Check that the key type is correct.
					if !asym.Algorithm.Algorithm.Equal(ed25519OID) {
						return nil, fmt.Errorf("incorrect ed25519 object identifier")
					}

					// Unmarshal the inner CurvePrivateKey.
					seed := new(curvePrivateKey)
					rest, err = asn1.Unmarshal(asym.PrivateKey, seed)
					if err != nil {
						return nil, err
					} else if len(rest) > 0 {
						return nil, fmt.Errorf("curvePrivateKey too long")
					}

					return ed25519.NewKeyFromSeed(*seed), nil
				}

				generalKey, err = parseEd25519PrivateKey(keyDER)
				if err != nil {
					// We don't include the actual error into
					// the final error. The reason might be
					// we don't want to leak any info about
					// the private key.
					return nil, fmt.Errorf("failed to parse private key")
				}
			}
		}
	}

	switch t := generalKey.(type) {
	case *rsa.PrivateKey:
		return t, nil
	case *ecdsa.PrivateKey:
		return t, nil
	case ed25519.PrivateKey:
		return t, nil
	}

	// should never reach here
	return nil, fmt.Errorf("failed to parse private key")
}
