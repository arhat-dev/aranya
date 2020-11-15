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

	codapiv1 "k8s.io/api/coordination/v1"
	codapiv1b1 "k8s.io/api/coordination/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
	"k8s.io/kubernetes/pkg/apis/coordination"
	codv1 "k8s.io/kubernetes/pkg/apis/coordination/v1"
	codv1b1 "k8s.io/kubernetes/pkg/apis/coordination/v1beta1"
)

func CreateLeaseClient(apiResources []*metav1.APIResourceList, kubeClient kubernetes.Interface, namespace string) *LeaseClient {
	client := &LeaseClient{}

	_ = discovery.FilteredBy(discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
		switch groupVersion + r.Kind {
		case codv1.SchemeGroupVersion.String() + "Lease":
			client.V1Client = kubeClient.CoordinationV1().Leases(namespace)
		case codv1b1.SchemeGroupVersion.String() + "Lease":
			client.V1b1Client = kubeClient.CoordinationV1beta1().Leases(namespace)
		}

		return false
	}), apiResources)

	return client
}

type LeaseClient struct {
	V1Client   v1.LeaseInterface
	V1b1Client v1beta1.LeaseInterface
}

func (c *LeaseClient) Create(ctx context.Context, lease *coordination.Lease, opts metav1.CreateOptions) (*coordination.Lease, error) {
	var (
		err    error
		result = new(coordination.Lease)
	)

	switch {
	case c.V1Client != nil:
		out := new(codapiv1.Lease)
		// scope is not used
		err = codv1.Convert_coordination_Lease_To_v1_Lease(lease, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.Create(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = codv1.Convert_v1_Lease_To_coordination_Lease(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(codapiv1b1.Lease)
		// scope is not used
		err = codv1b1.Convert_coordination_Lease_To_v1beta1_Lease(lease, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.Create(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = codv1b1.Convert_v1beta1_Lease_To_coordination_Lease(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported Lease api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *LeaseClient) Update(ctx context.Context, lease *coordination.Lease, opts metav1.UpdateOptions) (*coordination.Lease, error) {
	var (
		err    error
		result = new(coordination.Lease)
	)

	switch {
	case c.V1Client != nil:
		out := new(codapiv1.Lease)
		// scope is not used
		err = codv1.Convert_coordination_Lease_To_v1_Lease(lease, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.Update(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = codv1.Convert_v1_Lease_To_coordination_Lease(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(codapiv1b1.Lease)
		// scope is not used
		err = codv1b1.Convert_coordination_Lease_To_v1beta1_Lease(lease, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.Update(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = codv1b1.Convert_v1beta1_Lease_To_coordination_Lease(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported Lease api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *LeaseClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	switch {
	case c.V1Client != nil:
		return c.V1Client.Delete(ctx, name, opts)
	case c.V1b1Client != nil:
		return c.V1b1Client.Delete(ctx, name, opts)
	default:
		return fmt.Errorf("unsupported Lease api version")
	}
}

func (c *LeaseClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	switch {
	case c.V1Client != nil:
		return c.V1Client.DeleteCollection(ctx, opts, listOpts)
	case c.V1b1Client != nil:
		return c.V1b1Client.DeleteCollection(ctx, opts, listOpts)
	default:
		return fmt.Errorf("unsupported Lease api version")
	}
}

func (c *LeaseClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*coordination.Lease, error) {
	var (
		err    error
		result = new(coordination.Lease)
	)

	switch {
	case c.V1Client != nil:
		var ret *codapiv1.Lease
		ret, err = c.V1Client.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}

		err = codv1.Convert_v1_Lease_To_coordination_Lease(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *codapiv1b1.Lease
		ret, err = c.V1b1Client.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}

		err = codv1b1.Convert_v1beta1_Lease_To_coordination_Lease(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported Lease api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *LeaseClient) List(ctx context.Context, opts metav1.ListOptions) (*coordination.LeaseList, error) {
	var (
		err    error
		result = new(coordination.LeaseList)
	)

	switch {
	case c.V1Client != nil:
		var ret *codapiv1.LeaseList
		ret, err = c.V1Client.List(ctx, opts)
		if err != nil {
			return nil, err
		}

		err = codv1.Convert_v1_LeaseList_To_coordination_LeaseList(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *codapiv1b1.LeaseList
		ret, err = c.V1b1Client.List(ctx, opts)
		if err != nil {
			return nil, err
		}

		err = codv1b1.Convert_v1beta1_LeaseList_To_coordination_LeaseList(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported Lease api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *LeaseClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	switch {
	case c.V1Client != nil:
		return c.V1Client.Watch(ctx, opts)
	case c.V1b1Client != nil:
		return c.V1b1Client.Watch(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported Lease api version")
	}
}

func (c *LeaseClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subResources ...string) (*coordination.Lease, error) {
	var (
		err    error
		result = new(coordination.Lease)
	)

	switch {
	case c.V1Client != nil:
		var ret *codapiv1.Lease
		ret, err = c.V1Client.Patch(ctx, name, pt, data, opts, subResources...)
		if err != nil {
			return nil, err
		}

		err = codv1.Convert_v1_Lease_To_coordination_Lease(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *codapiv1b1.Lease
		ret, err = c.V1b1Client.Patch(ctx, name, pt, data, opts, subResources...)
		if err != nil {
			return nil, err
		}

		err = codv1b1.Convert_v1beta1_Lease_To_coordination_Lease(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported Lease api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}
