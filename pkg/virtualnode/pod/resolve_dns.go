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
	"fmt"
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func (m *Manager) resolveDNSSettings(pod *corev1.Pod) (_ *aranyaapi.PodDNSConfig, err error) {
	defaults := new(aranyaapi.PodDNSConfig)

	bareDefaults := m.options.Config.DNS
	clusterDomain := bareDefaults.ClusterDomain
	searches := append([]string{
		constant.TenantNS() + ".svc." + clusterDomain,
		"svc." + clusterDomain,
		clusterDomain,
	}, bareDefaults.Searches...)

	defaults.Searches = omitDuplicates(searches)
	defaults.Servers = omitDuplicates(bareDefaults.Servers)

	result := new(aranyaapi.PodDNSConfig)

	// use custom dns
	if pod.Spec.DNSPolicy == corev1.DNSNone {
		config := pod.Spec.DNSConfig
		if config == nil {
			return nil, fmt.Errorf("unexpected nil dnsConfig for dnsPolicy none")
		}

		result.Servers = omitDuplicates(config.Nameservers)
		result.Searches = omitDuplicates(config.Searches)
		result.Options = mergeDNSOptions(result.Options, config.Options)
		return result, nil
	}

	// use host dns
	if pod.Spec.HostNetwork && pod.Spec.DNSPolicy != corev1.DNSClusterFirstWithHostNet {
		return result, nil
	}

	// use cluster dns
	// lookup cluster dns servers with best effort
	ips, _ := net.LookupIP("kube-dns.kube-system")
	for _, ip := range ips {
		result.Servers = append(result.Servers, ip.String())
	}
	result.Servers = omitDuplicates(result.Servers)

	result.Searches = defaults.Searches
	if config := pod.Spec.DNSConfig; config != nil {
		result.Options = mergeDNSOptions(defaults.Options, config.Options)
	}

	return result, nil
}

func omitDuplicates(strs []string) []string {
	uniqueStrs := make(map[string]bool)

	var ret []string
	for _, str := range strs {
		if !uniqueStrs[str] {
			ret = append(ret, str)
			uniqueStrs[str] = true
		}
	}
	return ret
}

// mergeDNSOptions merges DNS options. If duplicated, entries given by PodDNSConfigOption will
// overwrite the existing ones.
func mergeDNSOptions(existingDNSConfigOptions []string, dnsConfigOptions []corev1.PodDNSConfigOption) []string {
	optionsMap := make(map[string]string)
	for _, op := range existingDNSConfigOptions {
		if index := strings.Index(op, ":"); index != -1 {
			optionsMap[op[:index]] = op[index+1:]
		} else {
			optionsMap[op] = ""
		}
	}
	for _, op := range dnsConfigOptions {
		if op.Value != nil {
			optionsMap[op.Name] = *op.Value
		} else {
			optionsMap[op.Name] = ""
		}
	}
	// Reconvert DNS options into a string array.
	var options []string
	for opName, opValue := range optionsMap {
		op := opName
		if opValue != "" {
			op = op + ":" + opValue
		}
		options = append(options, op)
	}
	return options
}
