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

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestPodCreated(t *testing.T) {
	kubeClient := createClient()

	tests := []struct {
		ns           string
		expectedPods []string
		podCount     int
	}{
		{
			ns: virtualPodNamespaceDefault,
			expectedPods: []string{
				edgeDeviceNameAlice,
				edgeDeviceNameBob,
			},
			// 1 aranya pod + 2 virtual pods
			podCount: 3,
		},
		{
			ns: virtualPodNamespaceTenant,
			expectedPods: []string{
				edgeDeviceNameFoo,
				edgeDeviceNameBar,
			},
			// 2 virtual pods + 3 abbot pods
			podCount: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.ns, func(t *testing.T) {
			pods, err := kubeClient.CoreV1().Pods(test.ns).List(context.TODO(), metav1.ListOptions{})
			if !assert.NoError(t, err) {
				return
			}

			assert.Len(t, pods.Items, test.podCount)

			expectedPods := sets.NewString(test.expectedPods...)
			var names []string
			for _, po := range pods.Items {
				if !expectedPods.Has(po.Name) {
					// ignore cluster nodes
					continue
				}

				names = append(names, po.Name)
			}

			assert.EqualValues(t, expectedPods.List(), sets.NewString(names...).List())
		})
	}
}
