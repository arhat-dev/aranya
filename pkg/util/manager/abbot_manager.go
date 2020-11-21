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

package manager

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"arhat.dev/abbot-proto/abbotgopb"
	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/conf"
	"arhat.dev/aranya/pkg/constant"
)

func NewAbbotManager(config conf.VirtualnodeNetworkBackendConfig) *AbbotManager {
	return &AbbotManager{
		provider:  constant.PrefixMeshInterfaceProviderAranya + "_abbot",
		endpoints: make(map[string]abbotgopb.NetworkManagerClient),

		mu: new(sync.RWMutex),
	}
}

type AbbotManager struct {
	provider  string
	endpoints map[string]abbotgopb.NetworkManagerClient

	mu *sync.RWMutex
}

func (m *AbbotManager) AddEndpoint(addr string, port int32) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoint := net.JoinHostPort(addr, strconv.FormatInt(int64(port), 10))

	if _, exists := m.endpoints[endpoint]; exists {
		return fmt.Errorf("endpoint already added")
	}

	cc, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to dial endpoint %q: %w", endpoint, err)
	}

	defer func() {
		if err != nil {
			_ = cc.Close()
		}
	}()

	client := abbotgopb.NewNetworkManagerClient(cc)
	m.endpoints[endpoint] = client

	return nil
}

func (m *AbbotManager) RemoveEndpoint(addr string, port int32) error {
	return nil
}
