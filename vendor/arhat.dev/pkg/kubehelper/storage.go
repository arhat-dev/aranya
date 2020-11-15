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

	storageapiv1 "k8s.io/api/storage/v1"
	storageapiv1b1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/storage/v1"
	"k8s.io/client-go/kubernetes/typed/storage/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/storage"
	storagev1 "k8s.io/kubernetes/pkg/apis/storage/v1"
	storagev1b1 "k8s.io/kubernetes/pkg/apis/storage/v1beta1"
)

func CreateCSIDriverLister(indexer cache.Indexer) *CSIDriverLister {
	return &CSIDriverLister{indexer: indexer}
}

type CSIDriverLister struct {
	indexer cache.Indexer
}

// List lists all CSIDrivers in the indexer.
func (l *CSIDriverLister) List(selector labels.Selector) ([]*storage.CSIDriver, error) {
	var (
		err   error
		errIn error
		ret   []*storage.CSIDriver
	)

	err = cache.ListAll(l.indexer, selector, func(m interface{}) {
		if errIn != nil {
			return
		}

		out := new(storage.CSIDriver)

		switch t := m.(type) {
		case *storageapiv1.CSIDriver:
			errIn = storagev1.Convert_v1_CSIDriver_To_storage_CSIDriver(t, out, conversion.Scope(nil))
		case *storageapiv1b1.CSIDriver:
			errIn = storagev1b1.Convert_v1beta1_CSIDriver_To_storage_CSIDriver(t, out, conversion.Scope(nil))
		}

		if errIn != nil {
			return
		}

		ret = append(ret, out)
	})

	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Get retrieves the CSIDriver from the index for a given name.
func (l *CSIDriverLister) Get(name string) (*storage.CSIDriver, error) {
	obj, exists, err := l.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}

	if !exists {
		list := l.indexer.List()
		if len(list) == 0 {
			return nil, errors.NewNotFound(storageapiv1.Resource("csidriver"), name)
		}

		switch list[0].(type) {
		case *storageapiv1.CSIDriver:
			return nil, errors.NewNotFound(storageapiv1.Resource("csidriver"), name)
		case *storageapiv1b1.CSIDriver:
			return nil, errors.NewNotFound(storageapiv1b1.Resource("csidriver"), name)
		}
	}

	out := new(storage.CSIDriver)
	switch t := obj.(type) {
	case *storageapiv1.CSIDriver:
		err = storagev1.Convert_v1_CSIDriver_To_storage_CSIDriver(t, out, conversion.Scope(nil))
	case *storageapiv1b1.CSIDriver:
		err = storagev1b1.Convert_v1beta1_CSIDriver_To_storage_CSIDriver(t, out, conversion.Scope(nil))
	}
	if err != nil {
		return nil, err
	}

	return out, nil
}

func CreateCSIDriverClient(apiResources []*metav1.APIResourceList, kubeClient kubernetes.Interface) *CSIDriverClient {
	client := &CSIDriverClient{}

	_ = discovery.FilteredBy(discovery.ResourcePredicateFunc(func(groupVersion string, r *metav1.APIResource) bool {
		switch groupVersion + r.Kind {
		case storagev1.SchemeGroupVersion.String() + "CSIDriver":
			client.V1Client = kubeClient.StorageV1().CSIDrivers()
		case storagev1b1.SchemeGroupVersion.String() + "CSIDriver":
			client.V1b1Client = kubeClient.StorageV1beta1().CSIDrivers()
		}

		return false
	}), apiResources)

	return client
}

type CSIDriverClient struct {
	V1Client   v1.CSIDriverInterface
	V1b1Client v1beta1.CSIDriverInterface
}

func (c *CSIDriverClient) Create(ctx context.Context, csiDriver *storage.CSIDriver, opts metav1.CreateOptions) (*storage.CSIDriver, error) {
	var (
		err    error
		result = new(storage.CSIDriver)
	)

	switch {
	case c.V1Client != nil:
		out := new(storageapiv1.CSIDriver)
		// scope is not used
		err = storagev1.Convert_storage_CSIDriver_To_v1_CSIDriver(csiDriver, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.Create(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1.Convert_v1_CSIDriver_To_storage_CSIDriver(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(storageapiv1b1.CSIDriver)
		// scope is not used
		err = storagev1b1.Convert_storage_CSIDriver_To_v1beta1_CSIDriver(csiDriver, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.Create(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1b1.Convert_v1beta1_CSIDriver_To_storage_CSIDriver(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CSIDriver api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CSIDriverClient) Update(ctx context.Context, csiDriver *storage.CSIDriver, opts metav1.UpdateOptions) (*storage.CSIDriver, error) {
	var (
		err    error
		result = new(storage.CSIDriver)
	)

	switch {
	case c.V1Client != nil:
		out := new(storageapiv1.CSIDriver)
		// scope is not used
		err = storagev1.Convert_storage_CSIDriver_To_v1_CSIDriver(csiDriver, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1Client.Update(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1.Convert_v1_CSIDriver_To_storage_CSIDriver(out, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		out := new(storageapiv1b1.CSIDriver)
		// scope is not used
		err = storagev1b1.Convert_storage_CSIDriver_To_v1beta1_CSIDriver(csiDriver, out, conversion.Scope(nil))
		if err != nil {
			return nil, err
		}

		out, err = c.V1b1Client.Update(ctx, out, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1b1.Convert_v1beta1_CSIDriver_To_storage_CSIDriver(out, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CSIDriver api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CSIDriverClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	switch {
	case c.V1Client != nil:
		return c.V1Client.Delete(ctx, name, opts)
	case c.V1b1Client != nil:
		return c.V1b1Client.Delete(ctx, name, opts)
	default:
		return fmt.Errorf("unsupported CSIDriver api version")
	}
}

func (c *CSIDriverClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	switch {
	case c.V1Client != nil:
		return c.V1Client.DeleteCollection(ctx, opts, listOpts)
	case c.V1b1Client != nil:
		return c.V1b1Client.DeleteCollection(ctx, opts, listOpts)
	default:
		return fmt.Errorf("unsupported CSIDriver api version")
	}
}

func (c *CSIDriverClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*storage.CSIDriver, error) {
	var (
		err    error
		result = new(storage.CSIDriver)
	)

	switch {
	case c.V1Client != nil:
		var ret *storageapiv1.CSIDriver
		ret, err = c.V1Client.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1.Convert_v1_CSIDriver_To_storage_CSIDriver(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *storageapiv1b1.CSIDriver
		ret, err = c.V1b1Client.Get(ctx, name, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1b1.Convert_v1beta1_CSIDriver_To_storage_CSIDriver(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CSIDriver api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CSIDriverClient) List(ctx context.Context, opts metav1.ListOptions) (*storage.CSIDriverList, error) {
	var (
		err    error
		result = new(storage.CSIDriverList)
	)

	switch {
	case c.V1Client != nil:
		var ret *storageapiv1.CSIDriverList
		ret, err = c.V1Client.List(ctx, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1.Convert_v1_CSIDriverList_To_storage_CSIDriverList(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *storageapiv1b1.CSIDriverList
		ret, err = c.V1b1Client.List(ctx, opts)
		if err != nil {
			return nil, err
		}

		err = storagev1b1.Convert_v1beta1_CSIDriverList_To_storage_CSIDriverList(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CSIDriver api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *CSIDriverClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	switch {
	case c.V1Client != nil:
		return c.V1Client.Watch(ctx, opts)
	case c.V1b1Client != nil:
		return c.V1b1Client.Watch(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported CSIDriver api version")
	}
}

func (c *CSIDriverClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subResources ...string) (*storage.CSIDriver, error) {
	var (
		err    error
		result = new(storage.CSIDriver)
	)

	switch {
	case c.V1Client != nil:
		var ret *storageapiv1.CSIDriver
		ret, err = c.V1Client.Patch(ctx, name, pt, data, opts, subResources...)
		if err != nil {
			return nil, err
		}

		err = storagev1.Convert_v1_CSIDriver_To_storage_CSIDriver(ret, result, conversion.Scope(nil))
	case c.V1b1Client != nil:
		var ret *storageapiv1b1.CSIDriver
		ret, err = c.V1b1Client.Patch(ctx, name, pt, data, opts, subResources...)
		if err != nil {
			return nil, err
		}

		err = storagev1b1.Convert_v1beta1_CSIDriver_To_storage_CSIDriver(ret, result, conversion.Scope(nil))
	default:
		return nil, fmt.Errorf("unsupported CSIDriver api version")
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}
