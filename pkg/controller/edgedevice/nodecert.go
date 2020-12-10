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
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	certapi "k8s.io/kubernetes/pkg/apis/certificates"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type nodeCertController struct {
	certSecretClient clientcorev1.SecretInterface
	csrClient        *kubehelper.CertificateSigningRequestClient
}

func (c *nodeCertController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	preferredResources []*metav1.APIResourceList,
) error {
	c.certSecretClient = kubeClient.CoreV1().Secrets(envhelper.ThisPodNS())
	c.csrClient = kubehelper.CreateCertificateSigningRequestClient(preferredResources, kubeClient)

	return nil
}

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

// nolint:gocyclo,lll
func (c *Controller) ensureKubeletServerCert(
	hostNodeName, virtualNodeName string,
	certInfo aranyaapi.NodeCertSpec,
	nodeAddresses []corev1.NodeAddress,
) (*tls.Certificate, error) {
	var (
		dnsNames               []string
		ipAddresses            []net.IP
		requiredValidAddresses = make(map[string]struct{})
	)
	for _, nodeAddr := range nodeAddresses {
		switch nodeAddr.Type {
		case corev1.NodeHostName, corev1.NodeInternalDNS, corev1.NodeExternalDNS:
			dnsNames = append(dnsNames, nodeAddr.Address)
		case corev1.NodeInternalIP, corev1.NodeExternalIP:
			ipAddresses = append(ipAddresses, net.ParseIP(nodeAddr.Address))
		}
		requiredValidAddresses[nodeAddr.Address] = struct{}{}
	}

	var (
		// kubernetes secret name
		secretObjName = fmt.Sprintf("kubelet-tls.%s.%s", hostNodeName, virtualNodeName)
		// kubernetes csr name
		csrObjName = fmt.Sprintf("kubelet-tls.%s.%s", hostNodeName, virtualNodeName)
		logger     = c.Log.WithFields(log.String("name", csrObjName))
		certReq    = &x509.CertificateRequest{
			Subject: pkix.Name{
				// CommonName is the RBAC role name, which must be `system:node:{nodeName}`
				CommonName: fmt.Sprintf("system:node:%s", virtualNodeName),
				Country:    []string{certInfo.Country},
				Province:   []string{certInfo.State},
				Locality:   []string{certInfo.Locality},
				// Follow kubernetes guideline, see
				// https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/#kubernetes-signers
				Organization:       []string{"system:nodes"},
				OrganizationalUnit: []string{certInfo.OrgUnit},
			},
			DNSNames:    dnsNames,
			IPAddresses: ipAddresses,
		}

		needToCreateCSR      = false
		needToGetKubeCSR     = true
		needToCreateKubeCSR  = false
		needToApproveKubeCSR = true

		privateKeyBytes []byte
		csrBytes        []byte
		certBytes       []byte
	)

	pkSecret, err := c.certSecretClient.Get(c.Context(), secretObjName, metav1.GetOptions{})
	if err != nil {
		// create secret object if not found, add tls.csr
		if kubeerrors.IsNotFound(err) {
			needToCreateCSR = true
		} else {
			logger.I("failed to get csr and private key secret", log.Error(err))
			return nil, err
		}
	}

	if needToCreateCSR {
		logger.V("generating csr")
		var privateKey *ecdsa.PrivateKey
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			logger.I("failed to generate private key for node cert", log.Error(err))
			return nil, err
		}

		csrBytes, err = generatePEMEncodedCSR(certReq, privateKey)
		if err != nil {
			logger.I("failed to generate csr", log.Error(err))
			return nil, err
		}

		var b []byte
		b, err = x509.MarshalECPrivateKey(privateKey)
		if err != nil {
			logger.I("failed to marshal elliptic curve private key", log.Error(err))
			return nil, err
		}
		privateKeyBytes = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})

		pkSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: envhelper.ThisPodNS(),
				Name:      secretObjName,
				Labels: map[string]string{
					constant.LabelRole: constant.LabelRoleValueCertificate,
				},
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.csr":               csrBytes,
				corev1.TLSPrivateKeyKey: privateKeyBytes,
				corev1.TLSCertKey:       []byte(""),
			},
		}

		logger.D("creating secret with private key and csr")
		pkSecret, err = c.certSecretClient.Create(c.Context(), pkSecret, metav1.CreateOptions{})
		if err != nil {
			logger.I("failed create csr and private key secret", log.Error(err))
			return nil, err
		}
	} else {
		var ok bool

		privateKeyBytes, ok = pkSecret.Data[corev1.TLSPrivateKeyKey]
		if !ok {
			// TODO: try to fix this (caused by other controller or user)
			return nil, fmt.Errorf("invalid secret, missing private key")
		}

		certBytes, ok = pkSecret.Data[corev1.TLSCertKey]
		if ok && len(certBytes) > 0 {
			needToGetKubeCSR = false
		}

		csrBytes, ok = pkSecret.Data["tls.csr"]
		if !ok && needToGetKubeCSR {
			// csr not found, but still need to get kubernetes csr, conflict
			// this could happen when user has created the tls secret with
			// same name in advance, but the
			return nil, fmt.Errorf("invalid secret, missing csr")
		} else if ok {
			// check whether old csr is valid
			// the csr could be invalid if aranya's host node has changed
			// its address, which is possible for nodes with dynamic addresses
			logger.V("validating old csr")

			var oldCSR *x509.CertificateRequest
			oldCSR, err = parsePEMEncodedCSR(csrBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse old csr: %w", err)
			}

			oldValidAddress := make(map[string]struct{})
			for _, ip := range oldCSR.IPAddresses {
				oldValidAddress[ip.String()] = struct{}{}
			}
			for _, dnsName := range oldCSR.DNSNames {
				oldValidAddress[dnsName] = struct{}{}
			}

			for requiredAddress := range requiredValidAddresses {
				if _, ok := oldValidAddress[requiredAddress]; !ok {
					needToGetKubeCSR = true
					needToCreateKubeCSR = true
					break
				}
			}

			if needToCreateKubeCSR {
				logger.D("old csr invalid, possible host node address change, recreating")

				var key crypto.Signer
				key, err = parsePEMEncodedPrivateKey(privateKeyBytes, nil)
				if err != nil {
					logger.I("failed to parse private key", log.Error(err))
					return nil, err
				}

				csrBytes, err = generatePEMEncodedCSR(certReq, key)
				if err != nil {
					logger.I("failed to generate csr", log.Error(err))
					return nil, err
				}
			} else {
				logger.D("old csr valid")
			}
		}
	}

	if needToGetKubeCSR {
		logger.V("fetching existing kubernetes csr")
		var csrReq *certapi.CertificateSigningRequest
		csrReq, err = c.csrClient.Get(c.Context(), csrObjName, metav1.GetOptions{})
		if err != nil {
			if kubeerrors.IsNotFound(err) {
				needToCreateKubeCSR = true
			} else {
				logger.I("failed to get certificate", log.Error(err))
				return nil, err
			}
		}

		if needToCreateKubeCSR {
			logger.D("deleting kubernetes csr to recreate")
			// delete first to make sure no invalid data exists
			err = c.csrClient.Delete(c.Context(), csrObjName, *deleteAtOnce)
			if err != nil && !kubeerrors.IsNotFound(err) {
				logger.I("failed to delete csr object", log.Error(err))
				return nil, err
			}

			csrObj := &certapi.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					// not namespaced
					Name: csrObjName,
				},
				Spec: certapi.CertificateSigningRequestSpec{
					Request:    csrBytes,
					SignerName: certapi.KubeletServingSignerName,
					Groups: []string{
						"system:nodes",
						"system:authenticated",
					},
					Usages: []certapi.KeyUsage{
						certapi.UsageKeyEncipherment,
						certapi.UsageDigitalSignature,
						certapi.UsageServerAuth,
					},
				},
			}

			logger.I("creating new kubernetes csr")
			csrReq, err = c.csrClient.Create(c.Context(), csrObj, metav1.CreateOptions{})
			if err != nil {
				logger.I("failed to crate new kubernetes csr", log.Error(err))
				return nil, err
			}
		}

		for _, condition := range csrReq.Status.Conditions {
			switch condition.Type {
			case certapi.CertificateApproved, certapi.CertificateDenied:
				needToApproveKubeCSR = false
			}

			if !needToApproveKubeCSR {
				break
			}
		}

		if needToApproveKubeCSR && c.vnConfig.Node.Cert.AutoApprove {
			csrReq.Status.Conditions = append(csrReq.Status.Conditions, certapi.CertificateSigningRequestCondition{
				Type:    certapi.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "self approved by aranya controller for EdgeDevice",
			})

			logger.D("approving kubernetes csr automatically")
			csrReq, err = c.csrClient.UpdateApproval(c.Context(), csrReq, metav1.UpdateOptions{})
			if err != nil {
				logger.I("failed to approve kubernetes csr", log.Error(err))
				return nil, err
			}
		}

		certBytes = csrReq.Status.Certificate
		if len(certBytes) == 0 {
			// this could happen since certificate won't be issued immediately (especially when autoApprove disabled)
			// good luck next time :)
			// TODO: should we watch?
			return nil, fmt.Errorf("certificate not issued")
		}

		pkSecret.Data[corev1.TLSCertKey] = certBytes
		logger.V("updating secret to include kubelet cert")
		_, err = c.certSecretClient.Update(c.Context(), pkSecret, metav1.UpdateOptions{})
		if err != nil {
			logger.I("failed to update node secret", log.Error(err))
			return nil, err
		}

		// delete at last to make sure no invalid data exists
		logger.V("deleting kubernetes csr after being approved")
		err = c.csrClient.Delete(c.Context(), csrObjName, *deleteAtOnce)
		if err != nil && !kubeerrors.IsNotFound(err) {
			logger.I("failed to delete csr object", log.Error(err))
			return nil, err
		}
	}

	cert, err := tls.X509KeyPair(certBytes, privateKeyBytes)
	if err != nil {
		logger.I("failed to load certificate", log.Error(err))

		err2 := c.certSecretClient.Delete(c.Context(), secretObjName, *deleteAtOnce)
		if err2 != nil {
			logger.I("failed to delete bad certificate", log.Error(err2))
		}

		return nil, err
	}
	return &cert, nil
}

func (c *Controller) createTLSConfigFromSecret(tlsSecretRef *corev1.LocalObjectReference) (*tls.Config, error) {
	// no tls required
	if tlsSecretRef == nil {
		return nil, nil
	}

	// use system default tls config
	if tlsSecretRef.Name == "" {
		return &tls.Config{}, nil
	}

	tlsSecret, ok := c.getWatchSecretObject(tlsSecretRef.Name)
	if !ok {
		return nil, fmt.Errorf("failed to get secret %q", tlsSecretRef.Name)
	}

	_, insecureSkipVerify := tlsSecret.Data["insecureSkipVerify"]
	caPEM, hasCACert := tlsSecret.Data[constant.TLSCaCertKey]
	certPEM, hasCert := tlsSecret.Data[corev1.TLSCertKey]
	keyPEM, hasKey := tlsSecret.Data[corev1.TLSPrivateKeyKey]
	clientCaPEM, hasClientCACert := tlsSecret.Data["client_ca.crt"]

	tlsConfig := &tls.Config{
		ServerName:         string(tlsSecret.Data["serverName"]),
		InsecureSkipVerify: insecureSkipVerify,
	}

	if hasCACert {
		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(caPEM)
	}

	if hasClientCACert {
		tlsConfig.ClientCAs = x509.NewCertPool()
		tlsConfig.ClientCAs.AppendCertsFromPEM(clientCaPEM)
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	if hasCert {
		if hasKey {
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else {
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(certPEM) {
				return nil, fmt.Errorf("failed to append certificates")
			}
			tlsConfig.RootCAs = certPool
		}
	}

	return tlsConfig, nil
}

func (c *Controller) getUserPassFromSecret(name string) (username, password []byte, err error) {
	secret, ok := c.getWatchSecretObject(name)
	if !ok {
		return nil, nil, fmt.Errorf("failed to get secret %q", name)
	}

	if secret.Type != corev1.SecretTypeBasicAuth {
		return nil, nil, fmt.Errorf("non basic auth secret found by userPassRef")
	}

	return secret.Data[corev1.BasicAuthUsernameKey], secret.Data[corev1.BasicAuthPasswordKey], nil
}
