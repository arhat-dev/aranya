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

package samplecrd

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"

	"arhat.dev/template-kubernetes-controller/pkg/conf"
)

// CheckAPIVersionFallback is just a static library check for the api resource fallback if discovery failed,
// you should only use the result returned by this function when you have failed to discover api resources
// via kubernetes api discovery, and the result only contains the most legacy apis both wanted by the controller
// and supported by the client library
func CheckAPIVersionFallback(kubeClient kubeclient.Interface) []*metav1.APIResourceList {
	var ret []*metav1.APIResourceList

	_ = kubeClient.CoordinationV1beta1().Leases("")
	_ = kubeClient.CoordinationV1().Leases("")

	ret = append(ret, &metav1.APIResourceList{
		GroupVersion: "coordination.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{{
			Name:         "leases",
			SingularName: "",
			Namespaced:   true,
			Kind:         "Lease",
		}},
	})

	_ = kubeClient.StorageV1().CSIDrivers()
	_ = kubeClient.StorageV1beta1().CSIDrivers()

	ret = append(ret, &metav1.APIResourceList{
		GroupVersion: "storage.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{{
			Name:         "csidrivers",
			SingularName: "",
			Namespaced:   false,
			Kind:         "CSIDriver",
		}},
	})

	return ret
}

func NewController(
	appCtx context.Context,
	config *conf.TemplateKubernetesControllerConfig,
	preferredApis []*metav1.APIResourceList,
) (*Controller, error) {
	return &Controller{}, nil
}

type Controller struct {
}

func (c *Controller) Start() error {
	return nil
}
