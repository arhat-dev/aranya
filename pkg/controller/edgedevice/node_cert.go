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
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"

	"arhat.dev/pkg/envhelper"
	"arhat.dev/pkg/kubehelper"
	"arhat.dev/pkg/log"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	kubeclient "k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	kubecache "k8s.io/client-go/tools/cache"
	certapi "k8s.io/kubernetes/pkg/apis/certificates"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

type nodeCertController struct {
	certCtx    context.Context
	certLogger log.Interface

	certSecretClient clientcorev1.SecretInterface
	csrClient        *kubehelper.CertificateSigningRequestClient

	certSecretInformer kubecache.SharedIndexInformer

	certAutoApprove bool
}

func (c *nodeCertController) init(
	ctrl *Controller,
	config *conf.Config,
	kubeClient kubeclient.Interface,
	preferredResources []*metav1.APIResourceList,
	thisPodNSInformerFactory informers.SharedInformerFactory,
) error {
	c.certCtx = ctrl.Context()
	c.certLogger = ctrl.Log
	c.certAutoApprove = config.VirtualNode.Node.Cert.AutoApprove
	c.certSecretClient = kubeClient.CoreV1().Secrets(envhelper.ThisPodNS())
	c.csrClient = kubehelper.CreateCertificateSigningRequestClient(preferredResources, kubeClient)

	c.certSecretInformer = thisPodNSInformerFactory.Core().V1().Secrets().Informer()

	ctrl.cacheSyncWaitFuncs = append(ctrl.cacheSyncWaitFuncs, c.certSecretInformer.HasSynced)
	ctrl.listActions = append(ctrl.listActions, func() error {
		_, err := listerscorev1.NewSecretLister(c.certSecretInformer.GetIndexer()).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list secrets in namespace %q: %w", envhelper.ThisPodNS(), err)
		}

		return nil
	})

	return nil
}

// nolint:gocyclo,lll
func (c *nodeCertController) ensureKubeletServerCert(
	name, hostNodeName string,
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
		secretObjName = fmt.Sprintf("kubelet-tls.%s.%s", hostNodeName, name)
		// kubernetes csr name
		csrObjName = fmt.Sprintf("kubelet-tls.%s.%s", hostNodeName, name)
		logger     = c.certLogger.WithFields(log.String("name", csrObjName))
		certReq    = &x509.CertificateRequest{
			Subject: pkix.Name{
				// CommonName is the RBAC role name, which must be `system:node:{nodeName}`
				CommonName: fmt.Sprintf("system:node:%s", name),
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

	nodeCertSecret, err := c.getCertSecretObject(secretObjName)
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

		nodeCertSecret = &corev1.Secret{
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
		nodeCertSecret, err = c.certSecretClient.Create(c.certCtx, nodeCertSecret, metav1.CreateOptions{})
		if err != nil {
			logger.I("failed create csr and private key secret", log.Error(err))
			return nil, err
		}
	} else {
		var ok bool

		privateKeyBytes, ok = nodeCertSecret.Data[corev1.TLSPrivateKeyKey]
		if !ok {
			// TODO: try to fix this (caused by other controller or user)
			return nil, fmt.Errorf("invalid secret, missing private key")
		}

		certBytes, ok = nodeCertSecret.Data[corev1.TLSCertKey]
		if ok && len(certBytes) > 0 {
			needToGetKubeCSR = false
		}

		csrBytes, ok = nodeCertSecret.Data["tls.csr"]
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
		csrReq, err = c.csrClient.Get(c.certCtx, csrObjName, metav1.GetOptions{})
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
			err = c.csrClient.Delete(c.certCtx, csrObjName, *deleteAtOnce)
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
			csrReq, err = c.csrClient.Create(c.certCtx, csrObj, metav1.CreateOptions{})
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

		if needToApproveKubeCSR && c.certAutoApprove {
			csrReq.Status.Conditions = append(csrReq.Status.Conditions, certapi.CertificateSigningRequestCondition{
				Type:    certapi.CertificateApproved,
				Status:  coreapi.ConditionTrue,
				Reason:  "AranyaAutoApprove",
				Message: fmt.Sprintf("This CSR was approved by aranya controller for EdgeDevice %q", name),
			})

			logger.D("approving kubernetes csr automatically")
			csrReq, err = c.csrClient.UpdateApproval(c.certCtx, csrReq, metav1.UpdateOptions{})
			if err != nil {
				logger.I("failed to approve kubernetes csr", log.Error(err))
				return nil, err
			}
			logger.V("approved kubernetes csr automatically")
		}

		certBytes = csrReq.Status.Certificate
		if len(certBytes) == 0 {
			// this could happen since certificate won't be issued immediately (especially when autoApprove disabled)
			// good luck next time :)

			// Shall we watch to reduce retry times? Never!
			// CSR contains the cluster wide credential and requires list permission
			// DO NOT even think about it!
			//
			// and we have efficient backoff mechanism to wait for certificate issued
			return nil, fmt.Errorf("certificate not issued")
		}

		nodeCertSecret.Data[corev1.TLSCertKey] = certBytes
		logger.V("updating secret to include kubelet cert")
		_, err = c.certSecretClient.Update(c.certCtx, nodeCertSecret, metav1.UpdateOptions{})
		if err != nil {
			logger.I("failed to update node secret", log.Error(err))
			return nil, err
		}

		// delete at last to make sure no invalid data exists
		logger.V("deleting kubernetes csr after being approved")
		err = c.csrClient.Delete(c.certCtx, csrObjName, *deleteAtOnce)
		if err != nil && !kubeerrors.IsNotFound(err) {
			logger.I("failed to delete csr object", log.Error(err))
			return nil, err
		}
	}

	cert, err := tls.X509KeyPair(certBytes, privateKeyBytes)
	if err != nil {
		logger.I("failed to load certificate", log.Error(err))

		err2 := c.certSecretClient.Delete(c.certCtx, secretObjName, *deleteAtOnce)
		if err2 != nil {
			logger.I("failed to delete bad certificate", log.Error(err2))
		}

		return nil, err
	}
	return &cert, nil
}

func (c *nodeCertController) getCertSecretObject(name string) (*corev1.Secret, error) {
	obj, found, err := c.certSecretInformer.GetIndexer().GetByKey(envhelper.ThisPodNS() + "/" + name)
	if err != nil || !found {
		secret, err := c.certSecretClient.Get(c.certCtx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return secret, nil
	}

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("invalid non secret object for cert")
	}

	return secret, nil
}
