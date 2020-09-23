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

package convert

import (
	"net"

	corev1 "k8s.io/api/core/v1"
)

func GetPodCIDRWithIPVersion(podCIDR string, podCIDRs []string) (ipv4, ipv6 string) {
	for _, cidr := range append([]string{podCIDR}, podCIDRs...) {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		if ip.To4() == nil {
			ipv4 = cidr
		} else {
			ipv6 = cidr
		}
	}

	return
}

func GetPodIPs(ipv4, ipv6 string) (podCIDR string, podCIDRs []corev1.PodIP) {
	if ipv4 != "" {
		podCIDR = ipv4
		podCIDRs = append(podCIDRs, corev1.PodIP{IP: ipv4})
	}

	if ipv6 != "" {
		if podCIDR == "" {
			podCIDR = ipv6
		}
		podCIDRs = append(podCIDRs, corev1.PodIP{IP: ipv6})
	}

	return
}
