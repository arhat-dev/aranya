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
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	testLabel1 = "e2e.aranya.arhat.dev/label-1"
	testLabel2 = "e2e.aranya.arhat.dev/label-2"

	testAnnotation1 = "e2e.aranya.arhat.dev/annotation-1"
	testAnnotation2 = "e2e.aranya.arhat.dev/annotation-2"

	taintKey1 = "e2e.aranya.arhat.dev/taint-1"
	taintKey2 = "e2e.aranya.arhat.dev/taint-2"
)

// node ext info related
const (
	arhatStartCount = 5

	extInfoSetString    = "e2e.aranya.arhat.dev/ext-info-set-string"
	extInfoAppendString = "e2e.aranya.arhat.dev/ext-info-append-string"

	extInfoSet1   = "e2e.aranya.arhat.dev/ext-info-set-1"
	extInfoAdd1   = "e2e.aranya.arhat.dev/ext-info-add-1"
	extInfoMinus1 = "e2e.aranya.arhat.dev/ext-info-minus-1"

	extInfoSet1_5   = "e2e.aranya.arhat.dev/ext-info-set-1-5"
	extInfoAdd1_5   = "e2e.aranya.arhat.dev/ext-info-add-1-5"
	extInfoMinus1_5 = "e2e.aranya.arhat.dev/ext-info-minus-1-5"
)

// field hook related
const (
	fieldHookExpressionValue = "e2e.aranya.arhat.dev/field-hook-expression-value"
	fieldHookReferenceValue  = "e2e.aranya.arhat.dev/field-hook-reference-value"
	fieldHookSetValue        = "e2e.aranya.arhat.dev/field-hook-set-value"
)

func TestNodeCreated(t *testing.T) {
	kubeClient, _ := createClient()

	expectedNodes := sets.NewString(
		edgeDeviceNameAlice,
		edgeDeviceNameBob,
		edgeDeviceNameFoo,
		edgeDeviceNameBar,
	)

	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if !assert.NoError(t, err) {
		return
	}

	var names []string
	for _, n := range nodes.Items {
		if !expectedNodes.Has(n.Name) {
			// ignore cluster nodes
			continue
		}

		assert.Equal(t, corev1.NodeRunning, n.Status.Phase)
		names = append(names, n.Name)
	}

	assert.EqualValues(t, expectedNodes.List(), sets.NewString(names...).List())
}

func TestNodeSpec(t *testing.T) {
	kubeClient, ver := createClient()

	aranyaDefault, err := getAranyaLeaderPod(kubeClient, aranyaNamespaceDefault)
	if !assert.NoError(t, err) {
		return
	}

	aranyaFull, err := getAranyaLeaderPod(kubeClient, aranyaNamespaceFull)
	if !assert.NoError(t, err) {
		return
	}

	nodeClient := kubeClient.CoreV1().Nodes()

	nodeDefault, err := nodeClient.Get(context.TODO(), aranyaDefault.Spec.NodeName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}

	nodeFull, err := nodeClient.Get(context.TODO(), aranyaFull.Spec.NodeName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}

	expectedNodeConditions := []corev1.NodeCondition{
		{
			Type:   corev1.NodeDiskPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeReady,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   corev1.NodeMemoryPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodePIDPressure,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   corev1.NodeNetworkUnavailable,
			Status: corev1.ConditionFalse,
		},
	}

	sort.Slice(expectedNodeConditions, func(i, j int) bool {
		return expectedNodeConditions[i].Type < expectedNodeConditions[j].Type
	})

	expectedCapacity := corev1.ResourceList{
		corev1.ResourceCPU:              resource.Quantity{},
		corev1.ResourceMemory:           resource.Quantity{},
		corev1.ResourceEphemeralStorage: resource.Quantity{},
		corev1.ResourcePods:             resource.Quantity{},
	}

	expectedAllocatable := corev1.ResourceList{
		corev1.ResourceCPU:              resource.Quantity{},
		corev1.ResourceMemory:           resource.Quantity{},
		corev1.ResourceEphemeralStorage: resource.Quantity{},
		corev1.ResourcePods:             resource.Quantity{},
	}

	extInfoValues := map[string]string{
		extInfoSetString:    "set-string",
		extInfoAppendString: strings.Repeat("str", arhatStartCount),

		extInfoSet1: "1",
		extInfoAdd1: strconv.FormatInt(arhatStartCount, 10),

		extInfoSet1_5: "1.5",
		extInfoAdd1_5: strconv.FormatFloat(1.5*arhatStartCount, 'f', -1, 64),

		// we can not set negative numbers as value to labels
	}

	commonLabels := map[string]string{
		"arhat.dev/arch":          "amd64",
		"arhat.dev/role":          "Node",
		"beta.kubernetes.io/arch": "amd64",
		"beta.kubernetes.io/os":   "linux",
		"kubernetes.io/arch":      "amd64",
		"kubernetes.io/os":        "linux",
	}

	commonAnnotations := map[string]string{
		// cannot set negative number as labels value
		extInfoMinus1:   strconv.FormatInt(-arhatStartCount, 10),
		extInfoMinus1_5: strconv.FormatFloat(-1.5*arhatStartCount, 'f', -1, 64),
	}

	for k, v := range extInfoValues {
		commonLabels[k] = v
		commonAnnotations[k] = v
	}

	tests := []struct {
		name string

		labels      map[string]string
		annotations map[string]string
		spec        corev1.NodeSpec
		status      corev1.NodeStatus
	}{
		{
			name: edgeDeviceNameAlice,
			labels: map[string]string{
				// override should not work on these labels
				"kubernetes.io/hostname": nodeDefault.Name,
				"arhat.dev/name":         edgeDeviceNameAlice,
				"arhat.dev/namespace":    edgeDeviceNamespaceDefault,

				// can override kubernetes.io/role
				"kubernetes.io/role": "valid-override",

				// custom labels
				testLabel1: "1",
				testLabel2: "2",

				// labels['e2e.aranya.arhat.dev/annotation-1'] == 1
				fieldHookExpressionValue: "true",
				// labels['kubernetes.io/role']
				fieldHookReferenceValue: "valid-override",
				fieldHookSetValue:       "set-label-value",
			},
			annotations: map[string]string{
				testAnnotation1: "1",

				// labels['e2e.aranya.arhat.dev/ext-info-add-1'] == arhatStartCount
				fieldHookExpressionValue: "true",
				// from .spec.providerID
				fieldHookReferenceValue: "aranya://" + edgeDeviceNamespaceDefault + "/" + edgeDeviceNameAlice,
				fieldHookSetValue:       "set-annotation-value",
			},
			spec: corev1.NodeSpec{
				PodCIDR:       "",
				PodCIDRs:      []string{},
				ProviderID:    "aranya://" + edgeDeviceNamespaceDefault + "/" + edgeDeviceNameAlice,
				Unschedulable: false,
				Taints: []corev1.Taint{
					{
						Key:    "arhat.dev/namespace",
						Value:  edgeDeviceNamespaceDefault,
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key:    taintKey1,
						Value:  "1",
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key:    taintKey2,
						Value:  "2",
						Effect: corev1.TaintEffectNoExecute,
					},
				},
				ConfigSource: nil,
			},
			status: corev1.NodeStatus{
				Capacity:    nil,
				Allocatable: nil,
				Phase:       corev1.NodeRunning,
				Conditions:  nil,
				Addresses:   nodeDefault.Status.Addresses,
				DaemonEndpoints: corev1.NodeDaemonEndpoints{
					KubeletEndpoint: corev1.DaemonEndpoint{
						Port: 0,
					},
				},
				NodeInfo: corev1.NodeSystemInfo{
					MachineID:     "",
					SystemUUID:    "",
					BootID:        "",
					KernelVersion: "",
					OSImage:       "",

					ContainerRuntimeVersion: "",
					KubeletVersion:          "",
					KubeProxyVersion:        "",
					OperatingSystem:         "linux",
					Architecture:            "amd64",
				},
				Images:          nil,
				VolumesInUse:    nil,
				VolumesAttached: nil,
				Config:          nil,
			},
		},
		{
			name: edgeDeviceNameBob,
			labels: map[string]string{
				// no override and should present
				"kubernetes.io/hostname": nodeDefault.Name,
				"arhat.dev/name":         edgeDeviceNameBob,
				"arhat.dev/namespace":    edgeDeviceNamespaceDefault,

				// can override kubernetes.io/role
				"kubernetes.io/role": "valid-override",

				// custom labels
				testLabel1: "1",
				testLabel2: "2",

				// labels['e2e.aranya.arhat.dev/annotation-2'] == 2
				fieldHookExpressionValue: "true",
				// labels['e2e.aranya.arhat.dev/ext-info-add-1']
				fieldHookReferenceValue: extInfoValues[extInfoAdd1],
				fieldHookSetValue:       "set-label-value",
			},
			annotations: map[string]string{
				testAnnotation2: "2",

				// labels['e2e.aranya.arhat.dev/ext-info-minus-1'] == -arhatStartCount
				fieldHookExpressionValue: "true",
				// labels['e2e.aranya.arhat.dev/ext-info-minus-1-5']
				fieldHookReferenceValue: commonAnnotations[extInfoMinus1_5],
				fieldHookSetValue:       "set-annotation-value",
			},
			spec: corev1.NodeSpec{
				PodCIDR:       "",
				PodCIDRs:      []string{},
				ProviderID:    "aranya://" + edgeDeviceNamespaceDefault + "/" + edgeDeviceNameBob,
				Unschedulable: false,
				Taints: []corev1.Taint{
					{
						Key:    "arhat.dev/namespace",
						Value:  edgeDeviceNamespaceDefault,
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				ConfigSource: nil,
			},
			status: corev1.NodeStatus{
				Capacity:    nil,
				Allocatable: nil,
				Phase:       corev1.NodeRunning,
				Conditions:  nil,
				Addresses:   nodeDefault.Status.Addresses,
				DaemonEndpoints: corev1.NodeDaemonEndpoints{
					KubeletEndpoint: corev1.DaemonEndpoint{
						Port: 0,
					},
				},
				NodeInfo: corev1.NodeSystemInfo{
					MachineID:     "",
					SystemUUID:    "",
					BootID:        "",
					KernelVersion: "",
					OSImage:       "",

					ContainerRuntimeVersion: "",
					KubeletVersion:          "",
					KubeProxyVersion:        "",
					OperatingSystem:         "linux",
					Architecture:            "amd64",
				},
				Images:          nil,
				VolumesInUse:    nil,
				VolumesAttached: nil,
				Config:          nil,
			},
		},
		{
			name: edgeDeviceNameFoo,
			labels: map[string]string{
				// no override and should present
				"kubernetes.io/hostname": nodeFull.Name,
				"arhat.dev/name":         edgeDeviceNameFoo,
				"arhat.dev/namespace":    edgeDeviceNamespaceSys,

				// no override to kubernetes.io/role and the value should be the namespace
				"kubernetes.io/role": edgeDeviceNamespaceSys,

				// custom labels
				testLabel1: "1",
				testLabel2: "2",
			},
			annotations: map[string]string{
				testAnnotation1: "1",
				testAnnotation2: "2",
			},
			spec: corev1.NodeSpec{
				PodCIDR:       "",
				PodCIDRs:      []string{},
				ProviderID:    "aranya://" + edgeDeviceNamespaceSys + "/" + edgeDeviceNameFoo,
				Unschedulable: false,
				Taints: []corev1.Taint{
					{
						Key:    "arhat.dev/namespace",
						Value:  edgeDeviceNamespaceSys,
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				ConfigSource: nil,
			},
			status: corev1.NodeStatus{
				Capacity:    nil,
				Allocatable: nil,
				Phase:       corev1.NodeRunning,
				Conditions:  nil,
				Addresses:   nodeFull.Status.Addresses,
				DaemonEndpoints: corev1.NodeDaemonEndpoints{
					KubeletEndpoint: corev1.DaemonEndpoint{
						Port: 0,
					},
				},
				NodeInfo: corev1.NodeSystemInfo{
					MachineID:     "",
					SystemUUID:    "",
					BootID:        "",
					KernelVersion: "",
					OSImage:       "",

					ContainerRuntimeVersion: "",
					KubeletVersion:          "",
					KubeProxyVersion:        "",
					OperatingSystem:         "linux",
					Architecture:            "amd64",
				},
				Images:          nil,
				VolumesInUse:    nil,
				VolumesAttached: nil,
				Config:          nil,
			},
		},
		{
			name: edgeDeviceNameBar,
			labels: map[string]string{
				// no override and should present
				"kubernetes.io/hostname": nodeFull.Name,
				"arhat.dev/name":         edgeDeviceNameBar,
				"arhat.dev/namespace":    edgeDeviceNamespaceSys,

				// no override to kubernetes.io/role and the value should be the namespace
				"kubernetes.io/role": edgeDeviceNamespaceSys,

				// no custom labels
				// testLabel1: "1",
				// testLabel2: "2",
			},
			annotations: map[string]string{
				// no custom annotations
				// testAnnotation1: "1",
				// testAnnotation2: "2",
			},
			spec: corev1.NodeSpec{
				PodCIDR:       "",
				PodCIDRs:      []string{},
				ProviderID:    "aranya://" + edgeDeviceNamespaceSys + "/" + edgeDeviceNameBar,
				Unschedulable: false,
				Taints: []corev1.Taint{
					{
						Key:    "arhat.dev/namespace",
						Value:  edgeDeviceNamespaceSys,
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				ConfigSource: nil,
			},
			status: corev1.NodeStatus{
				Capacity:    nil,
				Allocatable: nil,
				Phase:       corev1.NodeRunning,
				Conditions:  nil,
				Addresses:   nodeFull.Status.Addresses,
				DaemonEndpoints: corev1.NodeDaemonEndpoints{
					KubeletEndpoint: corev1.DaemonEndpoint{
						Port: 0,
					},
				},
				NodeInfo: corev1.NodeSystemInfo{
					MachineID:     "",
					SystemUUID:    "",
					BootID:        "",
					KernelVersion: "",
					OSImage:       "",

					ContainerRuntimeVersion: "",
					KubeletVersion:          "",
					KubeProxyVersion:        "",
					OperatingSystem:         "linux",
					Architecture:            "amd64",
				},
				Images:          nil,
				VolumesInUse:    nil,
				VolumesAttached: nil,
				Config:          nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node, err := nodeClient.Get(context.TODO(), test.name, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}

			// check node metadata
			{
				for k, v := range commonLabels {
					test.labels[k] = v
				}
				assert.EqualValues(t, test.labels, node.Labels)

				for k, v := range commonAnnotations {
					test.annotations[k] = v
				}
				// remove kubernetes managed annotations
				delete(node.Annotations, "node.alpha.kubernetes.io/ttl")
				assert.EqualValues(t, test.annotations, node.Annotations)
			}

			// check node spec
			{
				assert.NotEmpty(t, node.Spec.PodCIDR)

				if ver.LT(v1_16) {
					// there is no podCIDRs before Kubernetes v1.16
					assert.Empty(t, node.Spec.PodCIDRs)
				} else {
					assert.NotEmpty(t, node.Spec.PodCIDRs)
				}

				sort.Slice(node.Spec.Taints, func(i, j int) bool {
					return node.Spec.Taints[i].Key < node.Spec.Taints[j].Key
				})
				sort.Slice(test.spec.Taints, func(i, j int) bool {
					return test.spec.Taints[i].Key < test.spec.Taints[j].Key
				})

				node.Spec.PodCIDR = ""
				node.Spec.PodCIDRs = []string{}
				node.Spec.ConfigSource = nil

				assert.EqualValues(t, test.spec, node.Spec)
			}

			// check node status
			{
				// check node conditions
				actualConditions := node.Status.Conditions
				for i, cond := range actualConditions {
					// assert.NotEmpty(t, cond.Message)
					// assert.NotEmpty(t, cond.Reason)

					assert.False(t, cond.LastHeartbeatTime.Time.IsZero())
					assert.False(t, cond.LastTransitionTime.Time.IsZero())

					actualConditions[i] = corev1.NodeCondition{
						Type:   cond.Type,
						Status: cond.Status,
					}
				}
				sort.Slice(actualConditions, func(i, j int) bool {
					return actualConditions[i].Type < actualConditions[j].Type
				})

				assert.EqualValues(t, expectedNodeConditions, actualConditions)

				node.Status.Conditions = nil
			}

			{
				// check resouces
				actualCapacity := node.Status.Capacity
				for name := range actualCapacity {
					actualCapacity[name] = resource.Quantity{}
				}

				actualAllocatable := node.Status.Allocatable
				for name := range actualAllocatable {
					actualAllocatable[name] = resource.Quantity{}
				}

				assert.EqualValues(t, expectedCapacity, actualCapacity)
				assert.EqualValues(t, expectedAllocatable, actualAllocatable)

				node.Status.Capacity = nil
				node.Status.Allocatable = nil
			}
			{
				// check node system info
				assert.NotEmpty(t, node.Status.NodeInfo.MachineID)
				assert.NotEmpty(t, node.Status.NodeInfo.SystemUUID)
				assert.NotEmpty(t, node.Status.NodeInfo.BootID)
				assert.NotEmpty(t, node.Status.NodeInfo.KernelVersion)
				assert.NotEmpty(t, node.Status.NodeInfo.OSImage)

				assert.Empty(t, node.Status.NodeInfo.ContainerRuntimeVersion)
				assert.Empty(t, node.Status.NodeInfo.KubeletVersion)
				assert.Empty(t, node.Status.NodeInfo.KubeProxyVersion)

				assert.EqualValues(t, node.Status.NodeInfo.OperatingSystem, "linux")
				assert.EqualValues(t, node.Status.NodeInfo.Architecture, "amd64")

				node.Status.NodeInfo = corev1.NodeSystemInfo{
					OperatingSystem: "linux",
					Architecture:    "amd64",
				}
			}

			{
				// TODO: check volumes

				// sort.Slice(test.status.VolumesInUse, func(i, j int) bool {
				// 	return test.status.VolumesInUse[i] < test.status.VolumesInUse[j]
				// })
				// sort.Slice(node.Status.VolumesInUse, func(i, j int) bool {
				// 	return node.Status.VolumesInUse[i] < node.Status.VolumesInUse[j]
				// })

				assert.EqualValues(t, test.status.VolumesInUse, node.Status.VolumesInUse)

				// sort.Slice(test.status.VolumesAttached, func(i, j int) bool {
				// 	return test.status.VolumesAttached[i].Name < test.status.VolumesAttached[j].Name
				// })
				// sort.Slice(node.Status.VolumesAttached, func(i, j int) bool {
				// 	return node.Status.VolumesAttached[i].Name < node.Status.VolumesAttached[j].Name
				// })

				assert.EqualValues(t, test.status.VolumesAttached, node.Status.VolumesAttached)
			}

			{
				// TODO: check images
				// sort.Slice(test.status.Images, func(i, j int) bool {
				// 	return test.status.Images[i].Names[0] < test.status.Images[j].Names[0]
				// })

				assert.EqualValues(t, test.status.Images, node.Status.Images)
			}

			// other node status
			assert.Greater(t, int64(node.Status.DaemonEndpoints.KubeletEndpoint.Port), int64(1024))
			node.Status.DaemonEndpoints.KubeletEndpoint.Port = 0

			assert.EqualValues(t, test.status, node.Status)
		})
	}
}
