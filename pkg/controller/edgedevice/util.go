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
	"fmt"
	"net"
	"strconv"
	"strings"

	"arhat.dev/pkg/queue"
	"arhat.dev/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func getNodeLabelsAndAnnotationsToSet(
	nodeLabels, nodeAnnotations map[string]string,
	nodeSystemInfo *corev1.NodeSystemInfo,
	extLabels, extAnnotations map[string]string,
) (setLabels, setAnnotations map[string]string) {
	var (
		arch   = nodeSystemInfo.Architecture
		os     = nodeSystemInfo.OperatingSystem
		goArch = convertToGOARCH(arch)
	)

	for k, v := range map[string]string{
		constant.LabelArch:     arch,
		corev1.LabelArchStable: goArch,
		kubeletapis.LabelArch:  goArch,
		corev1.LabelOSStable:   os,
		kubeletapis.LabelOS:    os,
	} {
		// MUST not override these labels in ext info
		if _, override := extLabels[k]; override {
			extLabels[k] = v
		}

		// MUST set these labels if not present
		if existingVal, ok := nodeLabels[k]; !ok || v != existingVal {
			extLabels[k] = v
		}
	}

	// remove labels already set
	for k, v := range nodeLabels {
		if extLabels[k] == v {
			delete(extLabels, k)
		}
	}

	// remove annotations already set
	for k, v := range nodeAnnotations {
		if extAnnotations[k] == v {
			delete(extAnnotations, k)
		}
	}

	return extLabels, extAnnotations
}

func convertToGOARCH(arch string) string {
	switch {
	case arch == "x86":
		return "386"
	case strings.HasPrefix(arch, "armv"):
		// armv5/armv6/armv7 -> arm
		return "arm"
	case strings.HasPrefix(arch, "mips"):
		// mipshf/mips64hf -> mips/mips64 (runtime.GOARCH)
		return strings.TrimSuffix(arch, "hf")
	}

	return arch
}

func (c *Controller) getEdgeDeviceObject(name string) (*aranyaapi.EdgeDevice, bool) {
	obj, found, err := c.edgeDeviceInformer.GetStore().GetByKey(constant.SysNS() + "/" + name)
	if err != nil || !found {
		ed, err := c.edgeDeviceClient.Get(c.Context(), name, metav1.GetOptions{})
		if err != nil {
			return nil, false
		}

		return ed, true
	}

	ed, ok := obj.(*aranyaapi.EdgeDevice)
	if !ok {
		return nil, false
	}

	return ed, true
}

func newTweakListOptionsFunc(labelReqs labels.Selector) func(options *metav1.ListOptions) {
	reqs, _ := labelReqs.Requirements()
	if len(reqs) == 0 {
		return func(options *metav1.ListOptions) {}
	}

	return func(options *metav1.ListOptions) {
		ls, err := labels.Parse(options.LabelSelector)
		if err != nil || ls == nil {
			ls = labels.NewSelector()
		}

		options.LabelSelector = ls.Add(reqs...).String()
	}
}

func nextActionUpdate(obj interface{}) *reconcile.Result {
	return &reconcile.Result{NextAction: queue.ActionUpdate}
}

func getListenerPort(l net.Listener) (int32, error) {
	if l == nil {
		return 0, fmt.Errorf("nil listener")
	}

	_, port, _ := net.SplitHostPort(l.Addr().String())
	p, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(p), nil
}

func containsAll(real, expected []string) bool {
	wanted := make(map[string]struct{})
	for _, v := range expected {
		wanted[v] = struct{}{}
	}

	for _, v := range real {
		delete(wanted, v)
	}

	return len(wanted) == 0
}

func accessMap(d1 map[string]string, d2 map[string][]byte, key string) ([]byte, bool) {
	if len(d1) > 0 {
		if v, ok := d1[key]; ok {
			return []byte(v), true
		}
	}

	if len(d2) > 0 {
		if v, ok := d2[key]; ok {
			return v, true
		}
	}

	return nil, false
}
