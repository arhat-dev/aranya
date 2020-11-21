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
	"math/big"
	"net"
	"sync"

	"github.com/apparentlymart/go-cidr/cidr"
)

const (
	ipv4Bits = 8 * net.IPv4len
	ipv6Bits = 8 * net.IPv6len
)

type normFunc func(ip net.IP) net.IP

func NewBlock(ipCIDR, start, end string) (*Block, error) {
	// nolint:staticcheck
	startIP, ipNet, err := net.ParseCIDR(ipCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid cidr: %w", err)
	}

	ones, bits := ipNet.Mask.Size()
	switch {
	case bits == ipv4Bits && ones == ipv4Bits-1,
		bits == ipv6Bits && ones == ipv6Bits-1:
		// x.x.x.x/31 or ::/127 no host ip available, treat as error
		return nil, fmt.Errorf("invalid prefix length %d: no address available in this range", ones)
	}

	normFunc := func(ip net.IP) net.IP {
		return ip.To4()
	}

	if bits == ipv6Bits {
		normFunc = func(ip net.IP) net.IP {
			return ip.To16()
		}
	}

	if start != "" {
		startIP = net.ParseIP(start)
		if startIP == nil {
			return nil, fmt.Errorf("invalid start ip %q", start)
		}
		if !ipNet.Contains(startIP) {
			return nil, fmt.Errorf("invalid start ip: out of cidr range")
		}
	} else {
		startIP = ipNet.IP
	}

	var endIP net.IP
	if end != "" {
		endIP = net.ParseIP(end)
		if endIP == nil {
			return nil, fmt.Errorf("invalid end ip %q", end)
		}
		if !ipNet.Contains(endIP) {
			return nil, fmt.Errorf("invalid end ip: out of cidr range")
		}
	} else {
		endIP = lastIP(startIP, bits, ones)
	}

	var (
		startInt = new(big.Int).SetBytes(normFunc(startIP))
		endInt   = new(big.Int).SetBytes(normFunc(endIP))
	)

	switch {
	case bits == ipv4Bits && ones == ipv4Bits:
		// x.x.x.x/32 host ip
	case bits == ipv6Bits && ones == ipv6Bits:
		// ::/128 host ip
	default:
		if startIP.Equal(ipNet.IP) {
			startInt = new(big.Int).SetBytes(normFunc(cidr.Inc(startIP)))
		}

		if endIP.Equal(lastIP(ipNet.IP, bits, ones)) {
			endInt = new(big.Int).SetBytes(normFunc(cidr.Dec(endIP)))
		}
	}

	b := &Block{
		Start: startInt,
		End:   endInt,
		Mask:  ipNet.Mask,

		normalize: normFunc,
		bits:      bits,
		next:      startInt,

		allocated: make(map[string]struct{}),

		mu: new(sync.Mutex),
	}

	if b.Start.Cmp(b.End) > 0 {
		return nil, fmt.Errorf("invalid start end pair")
	}

	return b, nil
}

type Block struct {
	Start *big.Int
	End   *big.Int
	Mask  net.IPMask

	normalize normFunc
	bits      int
	next      *big.Int

	allocated map[string]struct{}

	mu *sync.Mutex
}

func (b *Block) Overlapped(another *Block) bool {
	switch {
	case b.End.Cmp(another.Start) < 0,
		b.Start.Cmp(another.End) > 0:
		return false
	default:
		return true
	}
}

func (b *Block) Contains(ip net.IP) bool {
	ipInt := (new(big.Int)).SetBytes(b.normalize(ip))
	return b.Start.Cmp(ipInt) <= 0 && b.End.Cmp(ipInt) >= 0
}

// Allocate one ip address from this block, if `ipReq` is not empty, will allocate one
// ip address not used in sequence
func (b *Block) Allocate(ipReq net.IP) (*net.IPNet, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ipReq != nil {
		// want this ip address
		if !b.Contains(ipReq) {
			return nil, fmt.Errorf("ip request not in this block")
		}

		if _, used := b.allocated[ipReq.String()]; used {
			return nil, fmt.Errorf("requested ip already used")
		}

		// not used
		if ipReq.Equal(intToIP(b.next, b.bits)) {
			_ = b.moveToNextUnused()
		}

		b.allocated[ipReq.String()] = struct{}{}
		return &net.IPNet{IP: ipReq, Mask: b.Mask}, nil
	}

	ret := b.moveToNextUnused()
	if ret == nil {
		return nil, fmt.Errorf("no ip available")
	}
	b.allocated[ret.String()] = struct{}{}

	return &net.IPNet{
		IP:   ret,
		Mask: b.Mask,
	}, nil
}

func (b *Block) moveToNextUnused() (ret net.IP) {
	if b.next == nil {
		return nil
	}

	// always return next unused
	ret = intToIP(b.next, b.bits)
	var (
		nextIP = ret
		used   = true
	)
	for used {
		nextIP = b.normalize(cidr.Inc(nextIP))

		if b.End.Cmp((new(big.Int)).SetBytes(nextIP)) < 0 {
			b.next = nil
			return ret
		}

		_, used = b.allocated[nextIP.String()]
	}

	b.next = new(big.Int).SetBytes(nextIP)

	return ret
}

func (b *Block) PutBack(ip net.IP) bool {
	if !b.Contains(ip) {
		return false
	}

	ip = b.normalize(ip)
	if ip == nil {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	ipStr := ip.String()
	_, used := b.allocated[ipStr]
	if used {
		delete(b.allocated, ipStr)

		ipInt := (new(big.Int)).SetBytes(ip)
		if b.next.Cmp(ipInt) > 0 {
			// next > ip, move to front to reuse this ip address
			b.next = ipInt
		}
		return true
	}

	return false
}

func lastIP(ip net.IP, bits, ones int) net.IP {
	ipInt := big.NewInt(1)
	ipInt.Lsh(ipInt, uint(bits-ones))
	ipInt.Sub(ipInt, big.NewInt(1))
	ipInt.Or(ipInt, new(big.Int).SetBytes(ip))

	return intToIP(ipInt, bits)
}

func intToIP(ipInt *big.Int, bits int) net.IP {
	ipBytes := ipInt.Bytes()
	size := bits / 8
	ret := make([]byte, size)
	// Pack our IP bytes into the end of the return array,
	// since big.Int.Bytes() removes front zero padding.
	for i := 1; i <= len(ipBytes) && i <= size; i++ {
		ret[len(ret)-i] = ipBytes[len(ipBytes)-i]
	}

	return ret
}
