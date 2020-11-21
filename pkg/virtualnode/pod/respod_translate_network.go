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

package pod

import (
	"math"
	"strings"

	"arhat.dev/abbot-proto/abbotgopb"
	corev1 "k8s.io/api/core/v1"
	kubebandwidth "k8s.io/kubernetes/pkg/util/bandwidth"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
)

func translatePodNetworkOptions(
	pod *corev1.Pod,
	ipv4PodCIDR, ipv6PodCIDR string,
	dnsConfig *aranyaapi.PodDNSConfig,
) *abbotgopb.ContainerNetworkEnsureRequest {
	var capArgs []*abbotgopb.CNICapArgs
	capArgs = append(capArgs, &abbotgopb.CNICapArgs{
		Option: &abbotgopb.CNICapArgs_DnsConfigArg{
			DnsConfigArg: &abbotgopb.CNICapArgs_DNSConfig{
				Servers:  dnsConfig.Servers,
				Searches: dnsConfig.Searches,
				Options:  dnsConfig.Options,
			},
		},
	})

	// ipRange cap arg for cni
	if ipv4PodCIDR != "" {
		capArgs = append(capArgs, &abbotgopb.CNICapArgs{
			Option: &abbotgopb.CNICapArgs_IpRangeArg{
				IpRangeArg: &abbotgopb.CNICapArgs_IPRange{
					Subnet: ipv4PodCIDR,
				},
			},
		})
	}

	if ipv6PodCIDR != "" {
		capArgs = append(capArgs, &abbotgopb.CNICapArgs{
			Option: &abbotgopb.CNICapArgs_IpRangeArg{
				IpRangeArg: &abbotgopb.CNICapArgs_IPRange{
					Subnet: ipv6PodCIDR,
				},
			},
		})
	}

	ingress, egress, _ := kubebandwidth.ExtractPodBandwidthResources(pod.Annotations)
	if ingress != nil || egress != nil {
		var (
			ingressRate, egressRate int32 = math.MaxInt32, math.MaxInt32
		)

		if ingress != nil {
			ingressRate = int32(ingress.Value())
			//bandwidth.IngressBurst = math.MaxUint64 // no limit
		}

		if egress != nil {
			egressRate = int32(egress.Value())
			//bandwidth.EgressBurst = math.MaxUint64 // no limit
		}
		capArgs = append(capArgs, &abbotgopb.CNICapArgs{
			Option: &abbotgopb.CNICapArgs_BandwidthArg{
				BandwidthArg: &abbotgopb.CNICapArgs_Bandwidth{
					IngressRate: ingressRate,
					EgressRate:  egressRate,
					// currently it's unlimited in kubelet
					IngressBurst: math.MaxInt32,
					EgressBurst:  math.MaxInt32,
				},
			},
		})
	}

	for _, ctr := range pod.Spec.Containers {
		for _, p := range ctr.Ports {
			// portMapping cap args for cni
			capArgs = append(capArgs, &abbotgopb.CNICapArgs{
				Option: &abbotgopb.CNICapArgs_PortMapArg{
					PortMapArg: &abbotgopb.CNICapArgs_PortMap{
						ContainerPort: p.ContainerPort,
						HostPort:      p.HostPort,
						Protocol:      strings.ToLower(string(p.Protocol)),
						HostIp:        p.HostIP,
					},
				},
			})
		}
	}

	return &abbotgopb.ContainerNetworkEnsureRequest{
		CniArgs: map[string]string{
			"IgnoreUnknown":              "1",
			"K8S_POD_NAMESPACE":          pod.Namespace,
			"K8S_POD_NAME":               pod.Name,
			"K8S_POD_INFRA_CONTAINER_ID": "",
		},
		CapArgs: capArgs,
	}
}
