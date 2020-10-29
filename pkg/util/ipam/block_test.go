package ipam

import (
	"math/big"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBlock(t *testing.T) {
	tests := []struct {
		name  string
		cidr  string
		start string
		end   string

		expectErr bool

		block *Block
	}{
		{
			name: "IPv4 with no start end",
			cidr: "127.0.0.0/24",

			expectErr: false,

			block: &Block{
				Start: new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
				End:   new(big.Int).SetBytes(net.ParseIP("127.0.0.254").To4()),
				Mask:  net.CIDRMask(24, ipv4Bits),
				bits:  ipv4Bits,
				next:  new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
			},
		},
		{
			name:      "IPv4 with single ip",
			cidr:      "127.0.0.1/32",
			expectErr: false,
			block: &Block{
				Start: new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
				End:   new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
				Mask:  net.CIDRMask(32, ipv4Bits),
				bits:  ipv4Bits,
				next:  new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
			},
		},
		{
			name:      "IPv4 with start at network ip",
			cidr:      "127.0.0.1/24",
			start:     "127.0.0.0",
			end:       "127.0.0.1",
			expectErr: false,
			block: &Block{
				Start: new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
				End:   new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
				Mask:  net.CIDRMask(24, ipv4Bits),
				bits:  ipv4Bits,
				next:  new(big.Int).SetBytes(net.ParseIP("127.0.0.1").To4()),
			},
		},
		{
			name:  "IPv4 with end at network broadcast ip",
			cidr:  "127.0.0.1/24",
			start: "127.0.0.254",
			end:   "127.0.0.255",

			expectErr: false,

			block: &Block{
				Start: new(big.Int).SetBytes(net.ParseIP("127.0.0.254").To4()),
				End:   new(big.Int).SetBytes(net.ParseIP("127.0.0.254").To4()),
				Mask:  net.CIDRMask(24, ipv4Bits),
				bits:  ipv4Bits,
				next:  new(big.Int).SetBytes(net.ParseIP("127.0.0.254").To4()),
			},
		},
		{
			name:      "Bad IPv4 with start after end",
			cidr:      "127.0.0.1/24",
			start:     "127.0.0.253",
			end:       "127.0.0.252",
			expectErr: true,
		},
		{
			name:      "Bad IPv4 with start out of cidr",
			cidr:      "127.0.0.1/24",
			start:     "127.0.1.253",
			expectErr: true,
		},
		{
			name:      "Bad IPv4 with end out of cidr",
			cidr:      "127.0.0.1/24",
			end:       "127.0.1.253",
			expectErr: true,
		},
		{
			name:      "Bad IPv4 with invalid cidr",
			cidr:      "127.0.0.1/31",
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := NewBlock(test.cidr, test.start, test.end)
			if test.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			b.mu = nil
			b.normalize = nil
			assert.EqualValues(t, test.block, b)
		})
	}
}

func TestBlock_Allocate(t *testing.T) {
	tests := []struct {
		name   string
		cidr   string
		start  string
		end    string
		reqIPs []string

		expectErr   []bool
		expectedIPs []string
	}{
		{
			name:        "IPv4 single ip",
			cidr:        "127.0.0.128/32",
			reqIPs:      make([]string, 2),
			expectErr:   []bool{false, true},
			expectedIPs: []string{"127.0.0.128/32"},
		},
		{
			name:        "IPv4 no start end",
			cidr:        "127.0.0.1/30",
			reqIPs:      make([]string, 3),
			expectErr:   []bool{false, false, true},
			expectedIPs: []string{"127.0.0.1/30", "127.0.0.2/30"},
		},
		{
			name:        "IPv4 with start end",
			cidr:        "127.0.0.1/24",
			start:       "127.0.0.1",
			end:         "127.0.0.2",
			reqIPs:      make([]string, 3),
			expectErr:   []bool{false, false, true},
			expectedIPs: []string{"127.0.0.1/24", "127.0.0.2/24"},
		},
		{
			name:        "IPv4 req specific ip then empty",
			cidr:        "127.0.0.1/24",
			reqIPs:      []string{"127.0.0.129", "", "127.0.0.1", "127.0.0.2"},
			expectErr:   []bool{false, false, true, false},
			expectedIPs: []string{"127.0.0.129/24", "127.0.0.1/24", "", "127.0.0.2/24"},
		},
		{
			name:      "Bad IPv4 req invalid ip",
			cidr:      "127.0.0.1/24",
			reqIPs:    []string{"127.0.1.0"},
			expectErr: []bool{true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := NewBlock(test.cidr, test.start, test.end)
			assert.NoError(t, err)

			for i := range test.reqIPs {
				ret, err := b.Allocate(net.ParseIP(test.reqIPs[i]))
				if test.expectErr[i] {
					assert.Error(t, err)
					continue
				}

				assert.NoError(t, err)
				assert.Equal(t, test.expectedIPs[i], ret.String())
			}
		})
	}
}

func TestBlock_PutBack(t *testing.T) {
	const (
		cidr = "127.0.0.1/28"
		// use half of host ips (8)
		usedIPs = (1<<(32-28) - 2) / 2
	)
	tests := []struct {
		name string

		putBackIPs []string

		expectOk        []bool
		expectedNextIPs []string
	}{
		{
			name:            "IPv4 put back unused ip",
			putBackIPs:      []string{"127.0.0.15", "127.0.0.14", "127.0.0.13", "192.168.0.1"},
			expectOk:        []bool{false, false, false, false},
			expectedNextIPs: []string{"127.0.0.8", "127.0.0.8", "127.0.0.8", "127.0.0.8"},
		},
		{
			name:            "IPv4 put back used ip",
			putBackIPs:      []string{"127.0.0.6", "127.0.0.7", "127.0.0.1"},
			expectOk:        []bool{true, true, true},
			expectedNextIPs: []string{"127.0.0.6", "127.0.0.6", "127.0.0.1"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := NewBlock(cidr, "", "")
			assert.NoError(t, err)

			for i := 0; i < usedIPs; i++ {
				_, err = b.Allocate(nil)
				assert.NoError(t, err)
			}

			for i := range test.putBackIPs {
				assert.Equal(t, test.expectOk[i], b.PutBack(net.ParseIP(test.putBackIPs[i])))

				assert.Equal(t, test.expectedNextIPs[i], intToIP(b.next, b.bits).String())
			}
		})
	}
}
