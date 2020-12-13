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

package kubehelper

import (
	"context"
	"fmt"

	certapiv1 "k8s.io/api/certificates/v1"
	certapiv1b1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/certificates/v1"
	"k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/kubernetes/pkg/apis/certificates"
	certv1 "k8s.io/kubernetes/pkg/apis/certificates/v1"
	certv1b1 "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
)

func CreateCertificateSigningRequestClient(apiResources []*metav1.APIResourceList, kubeClient kubernetes.Interface) *CertificateSigningRequestClient {
	client := &CertificateSigningRequestClient{}

	_ = discovery.FilteredBy(discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
		switch groupVersion + r.Kind {
		case certv1.SchemeGroupVersion.String() + "CertificateSigningRequest":
			client.V1Client = kubeClient.CertificatesV1().CertificateSigningRequests()
		case certv1b1.SchemeGroupVersion.String() + "CertificateSigningRequest":
			client.V1b1Client = kubeClient.CertificatesV1beta1().CertificateSigningRequests()
		}

		return false
	}), apiResources)

	return client
}

type CertificateSigningRequestClient struct {
	V1Client   v1.CertificateSigningRequestInterface
	V1b1Client v1beta1.CertificateSigningRequestInterface
}

func (c *CertificateSigningRequestClient) Create(ctx context.Context, csr *certificates.CertificateSigningRequest, opts metav1.CreateOptions) (*certificates.CertificateSigningRequest, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequest)
	)

	switch {
	case c.V1Client != nil:
		out := new(certapiv1.CertificateSigningRequest)
		// scope is not used
		err = certv1.Convert_certificates_CertificateSigningRequest_To_v1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.Create(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(certapiv1b1.CertificateSigningRequest)
		// scope is not used
		err = certv1b1.Convert_certificates_CertificateSigningRequest_To_v1beta1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.Create(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CertificateSigningRequestClient) Update(ctx context.Context, csr *certificates.CertificateSigningRequest, opts metav1.UpdateOptions) (*certificates.CertificateSigningRequest, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequest)
	)

	switch {
	case c.V1Client != nil:
		out := new(certapiv1.CertificateSigningRequest)
		// scope is not used
		err = certv1.Convert_certificates_CertificateSigningRequest_To_v1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.Update(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(certapiv1b1.CertificateSigningRequest)
		// scope is not used
		err = certv1b1.Convert_certificates_CertificateSigningRequest_To_v1beta1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.Update(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CertificateSigningRequestClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	switch {
	case c.V1Client != nil:
		return c.V1Client.Delete(ctx, name, opts)
	case c.V1b1Client != nil:
		return c.V1b1Client.Delete(ctx, name, opts)
	default:
		return fmt.Errorf("unsupported CertificateSigningRequest api version")
	}
}

func (c *CertificateSigningRequestClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	switch {
	case c.V1Client != nil:
		return c.V1Client.DeleteCollection(ctx, opts, listOpts)
	case c.V1b1Client != nil:
		return c.V1b1Client.DeleteCollection(ctx, opts, listOpts)
	default:
		return fmt.Errorf("unsupported CertificateSigningRequest api version")
	}
}

func (c *CertificateSigningRequestClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*certificates.CertificateSigningRequest, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequest)
	)

	switch {
	case c.V1Client != nil:
		var ret *certapiv1.CertificateSigningRequest
		ret, err = c.V1Client.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *certapiv1b1.CertificateSigningRequest
		ret, err = c.V1b1Client.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CertificateSigningRequestClient) List(ctx context.Context, opts metav1.ListOptions) (*certificates.CertificateSigningRequestList, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequestList)
	)

	switch {
	case c.V1Client != nil:
		var ret *certapiv1.CertificateSigningRequestList
		ret, err = c.V1Client.List(ctx, opts)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequestList_To_certificates_CertificateSigningRequestList(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *certapiv1b1.CertificateSigningRequestList
		ret, err = c.V1b1Client.List(ctx, opts)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequestList_To_certificates_CertificateSigningRequestList(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CertificateSigningRequestClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	switch {
	case c.V1Client != nil:
		return c.V1Client.Watch(ctx, opts)
	case c.V1b1Client != nil:
		return c.V1b1Client.Watch(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}
}

func (c *CertificateSigningRequestClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subResources ...string) (*certificates.CertificateSigningRequest, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequest)
	)

	switch {
	case c.V1Client != nil:
		var ret *certapiv1.CertificateSigningRequest
		ret, err = c.V1Client.Patch(ctx, name, pt, data, opts, subResources...)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *certapiv1b1.CertificateSigningRequest
		ret, err = c.V1b1Client.Patch(ctx, name, pt, data, opts, subResources...)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CertificateSigningRequestClient) UpdateStatus(ctx context.Context, csr *certificates.CertificateSigningRequest, opts metav1.UpdateOptions) (*certificates.CertificateSigningRequest, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequest)
	)

	switch {
	case c.V1Client != nil:
		out := new(certapiv1.CertificateSigningRequest)
		// scope is not used
		err = certv1.Convert_certificates_CertificateSigningRequest_To_v1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.UpdateStatus(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(certapiv1b1.CertificateSigningRequest)
		// scope is not used
		err = certv1b1.Convert_certificates_CertificateSigningRequest_To_v1beta1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.UpdateStatus(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CertificateSigningRequestClient) UpdateApproval(ctx context.Context, csr *certificates.CertificateSigningRequest, opts metav1.UpdateOptions) (*certificates.CertificateSigningRequest, error) {
	var (
		err    error
		result = new(certificates.CertificateSigningRequest)
	)

	switch {
	case c.V1Client != nil:
		out := new(certapiv1.CertificateSigningRequest)
		// scope is not used
		err = certv1.Convert_certificates_CertificateSigningRequest_To_v1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.UpdateApproval(ctx, csr.Name, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1.Convert_v1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(certapiv1b1.CertificateSigningRequest)
		// scope is not used
		err = certv1b1.Convert_certificates_CertificateSigningRequest_To_v1beta1_CertificateSigningRequest(csr, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.UpdateApproval(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = certv1b1.Convert_v1beta1_CertificateSigningRequest_To_certificates_CertificateSigningRequest(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CertificateSigningRequest api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}
