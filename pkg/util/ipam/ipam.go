package ipam

import "sync"

func NewIPAddressManager() *IPAddressManager {
	return &IPAddressManager{
		blocks:       make(map[string]interface{}),
		currentBlock: "",

		mu: new(sync.Mutex),
	}
}

type IPAddressManager struct {
	blocks       map[string]interface{}
	currentBlock string

	mu *sync.Mutex
}

func (m *IPAddressManager) AddAddressBlock(cidr, start, end string) error {
	return nil
}

func (m *IPAddressManager) Allocate() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return "", nil
}

func (m *IPAddressManager) Put(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
}
