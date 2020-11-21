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

package ipam

import (
	"fmt"
	"net"
	"sort"
	"sync"
)

func NewIPAddressManager() *IPAddressManager {
	return &IPAddressManager{
		blocks:     nil,
		blockIndex: 0,

		mu: new(sync.RWMutex),
	}
}

type IPAddressManager struct {
	blocks     []*Block
	blockIndex int

	mu *sync.RWMutex
}

// AddAddressBlock adds one block allocation pool
func (m *IPAddressManager) AddAddressBlock(cidr, start, end string) error {
	b, err := NewBlock(cidr, start, end)
	if err != nil {
		return fmt.Errorf("invalid address block: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// check if any existing block overlapped
	for _, existingBlock := range m.blocks {
		if existingBlock.Overlapped(b) {
			return fmt.Errorf("block overlapped with existing blocks")
		}
	}

	m.blocks = append(m.blocks, b)

	sort.Slice(m.blocks, func(i, j int) bool {
		return m.blocks[i].Start.Cmp(m.blocks[j].Start) < 0
	})

	return nil
}

// Allocate one ip address from blocks, if `ipReq` is nil, will try to allocate a ip address from blocks
// in sequence
func (m *IPAddressManager) Allocate(ipReq net.IP) (*net.IPNet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ipReq == nil {
		ret, err := m.blocks[m.blockIndex].Allocate(nil)
		if err != nil {
			// no ip available from current block, check next if having more ip blocks
			for len(m.blocks) > m.blockIndex+1 {
				m.blockIndex++

				ret, err = m.blocks[m.blockIndex].Allocate(nil)
				if err == nil {
					return ret, nil
				}
			}

			return nil, fmt.Errorf("no ip address available")

		}
		return ret, nil
	}

	// requested a specific ip
	idx := sort.Search(len(m.blocks), func(i int) bool {
		return m.blocks[i].Contains(ipReq)
	})

	if idx < 0 {
		return nil, fmt.Errorf("no block can allocate this ip")
	}

	ret, err := m.blocks[idx].Allocate(ipReq)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate specific ip")
	}

	return ret, nil
}

// PutBack one ip address
func (m *IPAddressManager) PutBack(ip net.IP) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, block := range m.blocks {
		if !block.PutBack(ip) {
			continue
		}

		if i < m.blockIndex {
			m.blockIndex = i
		}

		break
	}
}

// Check if ip is managed
func (m *IPAddressManager) Check(ip string) {
	m.mu.Lock()
}
