// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: cmd_network.proto

package aranyagopb

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
	reflect "reflect"
	strings "strings"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type NetworkUpdatePodNetworkCmd struct {
	CidrIpv4 string `protobuf:"bytes,1,opt,name=cidr_ipv4,json=cidrIpv4,proto3" json:"cidr_ipv4,omitempty"`
	CidrIpv6 string `protobuf:"bytes,2,opt,name=cidr_ipv6,json=cidrIpv6,proto3" json:"cidr_ipv6,omitempty"`
}

func (m *NetworkUpdatePodNetworkCmd) Reset()      { *m = NetworkUpdatePodNetworkCmd{} }
func (*NetworkUpdatePodNetworkCmd) ProtoMessage() {}
func (*NetworkUpdatePodNetworkCmd) Descriptor() ([]byte, []int) {
	return fileDescriptor_c01a4fadacf92e62, []int{0}
}
func (m *NetworkUpdatePodNetworkCmd) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *NetworkUpdatePodNetworkCmd) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_NetworkUpdatePodNetworkCmd.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *NetworkUpdatePodNetworkCmd) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NetworkUpdatePodNetworkCmd.Merge(m, src)
}
func (m *NetworkUpdatePodNetworkCmd) XXX_Size() int {
	return m.Size()
}
func (m *NetworkUpdatePodNetworkCmd) XXX_DiscardUnknown() {
	xxx_messageInfo_NetworkUpdatePodNetworkCmd.DiscardUnknown(m)
}

var xxx_messageInfo_NetworkUpdatePodNetworkCmd proto.InternalMessageInfo

func (m *NetworkUpdatePodNetworkCmd) GetCidrIpv4() string {
	if m != nil {
		return m.CidrIpv4
	}
	return ""
}

func (m *NetworkUpdatePodNetworkCmd) GetCidrIpv6() string {
	if m != nil {
		return m.CidrIpv6
	}
	return ""
}

func init() {
	proto.RegisterType((*NetworkUpdatePodNetworkCmd)(nil), "aranya.NetworkUpdatePodNetworkCmd")
}

func init() { proto.RegisterFile("cmd_network.proto", fileDescriptor_c01a4fadacf92e62) }

var fileDescriptor_c01a4fadacf92e62 = []byte{
	// 192 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x4c, 0xce, 0x4d, 0x89,
	0xcf, 0x4b, 0x2d, 0x29, 0xcf, 0x2f, 0xca, 0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x4b,
	0x2c, 0x4a, 0xcc, 0xab, 0x4c, 0x54, 0x0a, 0xe3, 0x92, 0xf2, 0x83, 0x48, 0x84, 0x16, 0xa4, 0x24,
	0x96, 0xa4, 0x06, 0xe4, 0xa7, 0x40, 0xf9, 0xce, 0xb9, 0x29, 0x42, 0xd2, 0x5c, 0x9c, 0xc9, 0x99,
	0x29, 0x45, 0xf1, 0x99, 0x05, 0x65, 0x26, 0x12, 0x8c, 0x0a, 0x8c, 0x1a, 0x9c, 0x41, 0x1c, 0x20,
	0x01, 0xcf, 0x82, 0x32, 0x13, 0x64, 0x49, 0x33, 0x09, 0x26, 0x14, 0x49, 0x33, 0xa7, 0xf0, 0x0b,
	0x0f, 0xe5, 0x18, 0x6e, 0x3c, 0x94, 0x63, 0xf8, 0xf0, 0x50, 0x8e, 0xb1, 0xe1, 0x91, 0x1c, 0xe3,
	0x8a, 0x47, 0x72, 0x8c, 0x27, 0x1e, 0xc9, 0x31, 0x5e, 0x78, 0x24, 0xc7, 0xf8, 0xe0, 0x91, 0x1c,
	0xe3, 0x8b, 0x47, 0x72, 0x0c, 0x1f, 0x1e, 0xc9, 0x31, 0x4e, 0x78, 0x2c, 0xc7, 0x70, 0xe1, 0xb1,
	0x1c, 0xc3, 0x8d, 0xc7, 0x72, 0x0c, 0x51, 0x8a, 0x89, 0x45, 0x19, 0x89, 0x25, 0x7a, 0x29, 0xa9,
	0x65, 0xfa, 0x10, 0xf7, 0xe9, 0x82, 0x5d, 0x0b, 0xe5, 0xa4, 0xe7, 0x17, 0x24, 0x25, 0xb1, 0x81,
	0x45, 0x8c, 0x01, 0x01, 0x00, 0x00, 0xff, 0xff, 0xdd, 0x15, 0x6d, 0x8b, 0xd4, 0x00, 0x00, 0x00,
}

func (this *NetworkUpdatePodNetworkCmd) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*NetworkUpdatePodNetworkCmd)
	if !ok {
		that2, ok := that.(NetworkUpdatePodNetworkCmd)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.CidrIpv4 != that1.CidrIpv4 {
		return false
	}
	if this.CidrIpv6 != that1.CidrIpv6 {
		return false
	}
	return true
}
func (this *NetworkUpdatePodNetworkCmd) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 6)
	s = append(s, "&aranyagopb.NetworkUpdatePodNetworkCmd{")
	s = append(s, "CidrIpv4: "+fmt.Sprintf("%#v", this.CidrIpv4)+",\n")
	s = append(s, "CidrIpv6: "+fmt.Sprintf("%#v", this.CidrIpv6)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringCmdNetwork(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *NetworkUpdatePodNetworkCmd) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NetworkUpdatePodNetworkCmd) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *NetworkUpdatePodNetworkCmd) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.CidrIpv6) > 0 {
		i -= len(m.CidrIpv6)
		copy(dAtA[i:], m.CidrIpv6)
		i = encodeVarintCmdNetwork(dAtA, i, uint64(len(m.CidrIpv6)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.CidrIpv4) > 0 {
		i -= len(m.CidrIpv4)
		copy(dAtA[i:], m.CidrIpv4)
		i = encodeVarintCmdNetwork(dAtA, i, uint64(len(m.CidrIpv4)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintCmdNetwork(dAtA []byte, offset int, v uint64) int {
	offset -= sovCmdNetwork(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *NetworkUpdatePodNetworkCmd) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.CidrIpv4)
	if l > 0 {
		n += 1 + l + sovCmdNetwork(uint64(l))
	}
	l = len(m.CidrIpv6)
	if l > 0 {
		n += 1 + l + sovCmdNetwork(uint64(l))
	}
	return n
}

func sovCmdNetwork(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozCmdNetwork(x uint64) (n int) {
	return sovCmdNetwork(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *NetworkUpdatePodNetworkCmd) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&NetworkUpdatePodNetworkCmd{`,
		`CidrIpv4:` + fmt.Sprintf("%v", this.CidrIpv4) + `,`,
		`CidrIpv6:` + fmt.Sprintf("%v", this.CidrIpv6) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringCmdNetwork(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *NetworkUpdatePodNetworkCmd) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCmdNetwork
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: NetworkUpdatePodNetworkCmd: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NetworkUpdatePodNetworkCmd: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field CidrIpv4", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCmdNetwork
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthCmdNetwork
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthCmdNetwork
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.CidrIpv4 = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field CidrIpv6", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCmdNetwork
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthCmdNetwork
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthCmdNetwork
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.CidrIpv6 = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCmdNetwork(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthCmdNetwork
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthCmdNetwork
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipCmdNetwork(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowCmdNetwork
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowCmdNetwork
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowCmdNetwork
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthCmdNetwork
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupCmdNetwork
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthCmdNetwork
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthCmdNetwork        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowCmdNetwork          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupCmdNetwork = fmt.Errorf("proto: unexpected end of group")
)