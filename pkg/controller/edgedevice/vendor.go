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

	corev1 "k8s.io/api/core/v1"

	aranyaapi "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (c *Controller) generateAzureIoTHubOpts(config aranyaapi.AzureIoTHubSpec) (*connectivity.AzureIoTHubOpts, error) {
	var (
		iotSecret, eventSecret *corev1.Secret
		iotRef                 = config.IoTHub.ConnectionStringSecretKeyRef
		eventRef               = config.EventHub.ConnectionStringSecretKeyRef
	)

	switch "" {
	case iotRef.Name, eventRef.Name, iotRef.Key, eventRef.Key:
		return nil, fmt.Errorf("invalid config: iot hub or event hub ref has empty field")
	}

	iotSecret, ok := c.getTenantSecretObject(iotRef.Name)
	if !ok {
		return nil, fmt.Errorf("failed to get referenced iot hub secret %q", iotRef.Name)
	}

	if iotRef.Name == eventRef.Name {
		eventSecret = iotSecret
	} else {
		eventSecret, ok = c.getTenantSecretObject(eventRef.Name)
		if !ok {
			return nil, fmt.Errorf("failed to get referenced event hub secret %q", eventRef.Name)
		}
	}

	opts := &connectivity.AzureIoTHubOpts{
		Config: config,
	}
	if v, ok := accessMap(iotSecret.StringData, iotSecret.Data, iotRef.Key); ok {
		opts.IoTHubConnectionString = string(v)
	}

	if v, ok := accessMap(eventSecret.StringData, eventSecret.Data, eventRef.Key); ok {
		opts.EventHubConnectionString = string(v)
	}

	switch "" {
	case opts.EventHubConnectionString, opts.IoTHubConnectionString:
		return nil, fmt.Errorf("some of referenced connection strings not found")
	}

	return opts, nil
}

func (c *Controller) generateGCPPubSubOpts(config aranyaapi.GCPIoTCoreSpec) (*connectivity.GCPIoTCoreOpts, error) {
	var (
		psSecret, iotSecret *corev1.Secret
		psRef               = config.PubSub.CredentialsSecretKeyRef
		iotRef              = config.CloudIoT.CredentialsSecretKeyRef
	)

	switch "" {
	case psRef.Name, iotRef.Name, psRef.Key, iotRef.Key:
		return nil, fmt.Errorf("invalid config: pubsub or cloud iot ref has empty field")
	}

	psSecret, ok := c.getTenantSecretObject(psRef.Name)
	if !ok {
		return nil, fmt.Errorf("failed to get referenced pub sub secret %q", psRef.Name)
	}

	if psRef.Name == iotRef.Name {
		iotSecret = psSecret
	} else {
		iotSecret, ok = c.getTenantSecretObject(iotRef.Name)
		if !ok {
			return nil, fmt.Errorf("failed to get referenced iot hub secret %q", iotRef.Name)
		}
	}

	opts := &connectivity.GCPIoTCoreOpts{
		Config: config,
	}
	if v, ok := accessMap(psSecret.StringData, psSecret.Data, psRef.Key); ok {
		opts.PubSubCredentialsJSON = v
	}

	if v, ok := accessMap(iotSecret.StringData, iotSecret.Data, iotRef.Key); ok {
		opts.CloudIoTCredentialsJSON = v
	}

	switch "" {
	case string(opts.PubSubCredentialsJSON), string(opts.CloudIoTCredentialsJSON):
		return nil, fmt.Errorf("some of referenced credentials not found")
	}

	return opts, nil
}
