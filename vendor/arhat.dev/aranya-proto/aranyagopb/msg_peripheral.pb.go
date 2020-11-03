// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: msg_peripheral.proto

// +build !noperipheral

package aranyagopb

import (
	bytes "bytes"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
	reflect "reflect"
	strconv "strconv"
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

type PeripheralState int32

const (
	PERIPHERAL_STATE_UNKNOWN   PeripheralState = 0
	PERIPHERAL_STATE_CREATED   PeripheralState = 1
	PERIPHERAL_STATE_CONNECTED PeripheralState = 2
	PERIPHERAL_STATE_ERRORED   PeripheralState = 3
	PERIPHERAL_STATE_REMOVED   PeripheralState = 4
)

var PeripheralState_name = map[int32]string{
	0: "PERIPHERAL_STATE_UNKNOWN",
	1: "PERIPHERAL_STATE_CREATED",
	2: "PERIPHERAL_STATE_CONNECTED",
	3: "PERIPHERAL_STATE_ERRORED",
	4: "PERIPHERAL_STATE_REMOVED",
}

var PeripheralState_value = map[string]int32{
	"PERIPHERAL_STATE_UNKNOWN":   0,
	"PERIPHERAL_STATE_CREATED":   1,
	"PERIPHERAL_STATE_CONNECTED": 2,
	"PERIPHERAL_STATE_ERRORED":   3,
	"PERIPHERAL_STATE_REMOVED":   4,
}

func (PeripheralState) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_287a3c44189a38fa, []int{0}
}

type PeripheralStatusMsg struct {
	Kind  PeripheralType  `protobuf:"varint,1,opt,name=kind,proto3,enum=aranya.PeripheralType" json:"kind,omitempty"`
	Name  string          `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	State PeripheralState `protobuf:"varint,3,opt,name=state,proto3,enum=aranya.PeripheralState" json:"state,omitempty"`
	// Human readable description for this state
	Message string `protobuf:"bytes,4,opt,name=message,proto3" json:"message,omitempty"`
}

func (m *PeripheralStatusMsg) Reset()      { *m = PeripheralStatusMsg{} }
func (*PeripheralStatusMsg) ProtoMessage() {}
func (*PeripheralStatusMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_287a3c44189a38fa, []int{0}
}
func (m *PeripheralStatusMsg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PeripheralStatusMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PeripheralStatusMsg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PeripheralStatusMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeripheralStatusMsg.Merge(m, src)
}
func (m *PeripheralStatusMsg) XXX_Size() int {
	return m.Size()
}
func (m *PeripheralStatusMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_PeripheralStatusMsg.DiscardUnknown(m)
}

var xxx_messageInfo_PeripheralStatusMsg proto.InternalMessageInfo

func (m *PeripheralStatusMsg) GetKind() PeripheralType {
	if m != nil {
		return m.Kind
	}
	return _INVALID_PERIPHERAL_TYPE
}

func (m *PeripheralStatusMsg) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *PeripheralStatusMsg) GetState() PeripheralState {
	if m != nil {
		return m.State
	}
	return PERIPHERAL_STATE_UNKNOWN
}

func (m *PeripheralStatusMsg) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

type PeripheralStatusListMsg struct {
	Peripherals []*PeripheralStatusMsg `protobuf:"bytes,1,rep,name=peripherals,proto3" json:"peripherals,omitempty"`
}

func (m *PeripheralStatusListMsg) Reset()      { *m = PeripheralStatusListMsg{} }
func (*PeripheralStatusListMsg) ProtoMessage() {}
func (*PeripheralStatusListMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_287a3c44189a38fa, []int{1}
}
func (m *PeripheralStatusListMsg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PeripheralStatusListMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PeripheralStatusListMsg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PeripheralStatusListMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeripheralStatusListMsg.Merge(m, src)
}
func (m *PeripheralStatusListMsg) XXX_Size() int {
	return m.Size()
}
func (m *PeripheralStatusListMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_PeripheralStatusListMsg.DiscardUnknown(m)
}

var xxx_messageInfo_PeripheralStatusListMsg proto.InternalMessageInfo

func (m *PeripheralStatusListMsg) GetPeripherals() []*PeripheralStatusMsg {
	if m != nil {
		return m.Peripherals
	}
	return nil
}

type PeripheralOperationResultMsg struct {
	Data [][]byte `protobuf:"bytes,1,rep,name=data,proto3" json:"data,omitempty"`
}

func (m *PeripheralOperationResultMsg) Reset()      { *m = PeripheralOperationResultMsg{} }
func (*PeripheralOperationResultMsg) ProtoMessage() {}
func (*PeripheralOperationResultMsg) Descriptor() ([]byte, []int) {
	return fileDescriptor_287a3c44189a38fa, []int{2}
}
func (m *PeripheralOperationResultMsg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PeripheralOperationResultMsg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PeripheralOperationResultMsg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PeripheralOperationResultMsg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeripheralOperationResultMsg.Merge(m, src)
}
func (m *PeripheralOperationResultMsg) XXX_Size() int {
	return m.Size()
}
func (m *PeripheralOperationResultMsg) XXX_DiscardUnknown() {
	xxx_messageInfo_PeripheralOperationResultMsg.DiscardUnknown(m)
}

var xxx_messageInfo_PeripheralOperationResultMsg proto.InternalMessageInfo

func (m *PeripheralOperationResultMsg) GetData() [][]byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func init() {
	proto.RegisterEnum("aranya.PeripheralState", PeripheralState_name, PeripheralState_value)
	proto.RegisterType((*PeripheralStatusMsg)(nil), "aranya.PeripheralStatusMsg")
	proto.RegisterType((*PeripheralStatusListMsg)(nil), "aranya.PeripheralStatusListMsg")
	proto.RegisterType((*PeripheralOperationResultMsg)(nil), "aranya.PeripheralOperationResultMsg")
}

func init() { proto.RegisterFile("msg_peripheral.proto", fileDescriptor_287a3c44189a38fa) }

var fileDescriptor_287a3c44189a38fa = []byte{
	// 395 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x92, 0xc1, 0x6e, 0xda, 0x40,
	0x10, 0x86, 0xbd, 0xe0, 0x52, 0x75, 0xa9, 0x5a, 0x6b, 0x5b, 0x15, 0x8b, 0xa2, 0x15, 0xe5, 0x84,
	0x90, 0x70, 0x25, 0x7a, 0xee, 0x81, 0xc2, 0x4a, 0xad, 0x0a, 0x36, 0x5a, 0xdc, 0x52, 0xf5, 0x82,
	0x16, 0xb1, 0x32, 0x56, 0xc0, 0xb6, 0xbc, 0x4b, 0x24, 0x6e, 0x79, 0x84, 0x3c, 0x41, 0xce, 0xc9,
	0x9b, 0xe4, 0xc8, 0x91, 0x63, 0x30, 0x97, 0x1c, 0x79, 0x84, 0x88, 0x25, 0x09, 0x11, 0xf8, 0x36,
	0x33, 0xff, 0xef, 0x6f, 0xe6, 0xb7, 0x16, 0x7e, 0x9c, 0x09, 0x6f, 0x18, 0xf1, 0xd8, 0x8f, 0x26,
	0x3c, 0x66, 0x53, 0x2b, 0x8a, 0x43, 0x19, 0xa2, 0x1c, 0x8b, 0x59, 0xb0, 0x60, 0x45, 0xe3, 0x58,
	0xa9, 0x5c, 0x01, 0xf8, 0xa1, 0xf7, 0x3c, 0xec, 0x4b, 0x26, 0xe7, 0xa2, 0x2b, 0x3c, 0x54, 0x83,
	0xfa, 0x99, 0x1f, 0x8c, 0x4d, 0x50, 0x06, 0xd5, 0x77, 0x8d, 0x4f, 0xd6, 0x1e, 0x60, 0x1d, 0xac,
	0xee, 0x22, 0xe2, 0x54, 0x79, 0x10, 0x82, 0x7a, 0xc0, 0x66, 0xdc, 0xcc, 0x94, 0x41, 0xf5, 0x0d,
	0x55, 0x35, 0xaa, 0xc3, 0x57, 0x42, 0x32, 0xc9, 0xcd, 0xac, 0x02, 0x14, 0x4e, 0x01, 0xbb, 0x5d,
	0x9c, 0xee, 0x5d, 0xc8, 0x84, 0xaf, 0x67, 0x5c, 0x08, 0xe6, 0x71, 0x53, 0x57, 0x94, 0xa7, 0xb6,
	0xf2, 0x0f, 0x16, 0x8e, 0xef, 0xeb, 0xf8, 0x42, 0xee, 0x6e, 0xfc, 0x0e, 0xf3, 0x87, 0x3c, 0xc2,
	0x04, 0xe5, 0x6c, 0x35, 0xdf, 0xf8, 0x9c, 0xbe, 0x49, 0xa5, 0xa2, 0x2f, 0xfd, 0x95, 0x06, 0x2c,
	0x1d, 0x3c, 0x4e, 0xc4, 0x63, 0x26, 0xfd, 0x30, 0xa0, 0x5c, 0xcc, 0xa7, 0x0a, 0x8f, 0xa0, 0x3e,
	0x66, 0x92, 0x29, 0xee, 0x5b, 0xaa, 0xea, 0xda, 0x0d, 0x80, 0xef, 0x8f, 0x22, 0xa0, 0x12, 0x34,
	0x7b, 0x84, 0xfe, 0xea, 0xfd, 0x24, 0xb4, 0xd9, 0x19, 0xf6, 0xdd, 0xa6, 0x4b, 0x86, 0x7f, 0xec,
	0xdf, 0xb6, 0x33, 0xb0, 0x0d, 0x2d, 0x55, 0x6d, 0x51, 0xd2, 0x74, 0x49, 0xdb, 0x00, 0x08, 0xc3,
	0xe2, 0xa9, 0xea, 0xd8, 0x36, 0x69, 0xed, 0xf4, 0x4c, 0xea, 0xd7, 0x84, 0x52, 0x87, 0x92, 0xb6,
	0x91, 0x4d, 0x55, 0x29, 0xe9, 0x3a, 0x7f, 0x49, 0xdb, 0xd0, 0x7f, 0x0c, 0x96, 0x6b, 0xac, 0xad,
	0xd6, 0x58, 0xdb, 0xae, 0x31, 0xb8, 0x48, 0x30, 0xb8, 0x4e, 0x30, 0xb8, 0x4d, 0x30, 0x58, 0x26,
	0x18, 0xdc, 0x25, 0x18, 0xdc, 0x27, 0x58, 0xdb, 0x26, 0x18, 0x5c, 0x6e, 0xb0, 0xb6, 0xdc, 0x60,
	0x6d, 0xb5, 0xc1, 0xda, 0xff, 0x2f, 0x2c, 0x9e, 0x30, 0x69, 0x8d, 0xf9, 0xf9, 0xd7, 0xfd, 0x8f,
	0xac, 0xab, 0x87, 0xf2, 0xd8, 0x78, 0x61, 0x34, 0x1a, 0xe5, 0xd4, 0xe4, 0xdb, 0x43, 0x00, 0x00,
	0x00, 0xff, 0xff, 0x8c, 0xf2, 0x2f, 0xf2, 0x6c, 0x02, 0x00, 0x00,
}

func (x PeripheralState) String() string {
	s, ok := PeripheralState_name[int32(x)]
	if ok {
		return s
	}
	return strconv.Itoa(int(x))
}
func (this *PeripheralStatusMsg) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PeripheralStatusMsg)
	if !ok {
		that2, ok := that.(PeripheralStatusMsg)
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
	if this.Kind != that1.Kind {
		return false
	}
	if this.Name != that1.Name {
		return false
	}
	if this.State != that1.State {
		return false
	}
	if this.Message != that1.Message {
		return false
	}
	return true
}
func (this *PeripheralStatusListMsg) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PeripheralStatusListMsg)
	if !ok {
		that2, ok := that.(PeripheralStatusListMsg)
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
	if len(this.Peripherals) != len(that1.Peripherals) {
		return false
	}
	for i := range this.Peripherals {
		if !this.Peripherals[i].Equal(that1.Peripherals[i]) {
			return false
		}
	}
	return true
}
func (this *PeripheralOperationResultMsg) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PeripheralOperationResultMsg)
	if !ok {
		that2, ok := that.(PeripheralOperationResultMsg)
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
	if len(this.Data) != len(that1.Data) {
		return false
	}
	for i := range this.Data {
		if !bytes.Equal(this.Data[i], that1.Data[i]) {
			return false
		}
	}
	return true
}
func (this *PeripheralStatusMsg) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 8)
	s = append(s, "&aranyagopb.PeripheralStatusMsg{")
	s = append(s, "Kind: "+fmt.Sprintf("%#v", this.Kind)+",\n")
	s = append(s, "Name: "+fmt.Sprintf("%#v", this.Name)+",\n")
	s = append(s, "State: "+fmt.Sprintf("%#v", this.State)+",\n")
	s = append(s, "Message: "+fmt.Sprintf("%#v", this.Message)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *PeripheralStatusListMsg) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&aranyagopb.PeripheralStatusListMsg{")
	if this.Peripherals != nil {
		s = append(s, "Peripherals: "+fmt.Sprintf("%#v", this.Peripherals)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *PeripheralOperationResultMsg) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&aranyagopb.PeripheralOperationResultMsg{")
	s = append(s, "Data: "+fmt.Sprintf("%#v", this.Data)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringMsgPeripheral(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *PeripheralStatusMsg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PeripheralStatusMsg) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PeripheralStatusMsg) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Message) > 0 {
		i -= len(m.Message)
		copy(dAtA[i:], m.Message)
		i = encodeVarintMsgPeripheral(dAtA, i, uint64(len(m.Message)))
		i--
		dAtA[i] = 0x22
	}
	if m.State != 0 {
		i = encodeVarintMsgPeripheral(dAtA, i, uint64(m.State))
		i--
		dAtA[i] = 0x18
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dAtA[i:], m.Name)
		i = encodeVarintMsgPeripheral(dAtA, i, uint64(len(m.Name)))
		i--
		dAtA[i] = 0x12
	}
	if m.Kind != 0 {
		i = encodeVarintMsgPeripheral(dAtA, i, uint64(m.Kind))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *PeripheralStatusListMsg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PeripheralStatusListMsg) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PeripheralStatusListMsg) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Peripherals) > 0 {
		for iNdEx := len(m.Peripherals) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Peripherals[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintMsgPeripheral(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *PeripheralOperationResultMsg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PeripheralOperationResultMsg) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PeripheralOperationResultMsg) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Data) > 0 {
		for iNdEx := len(m.Data) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Data[iNdEx])
			copy(dAtA[i:], m.Data[iNdEx])
			i = encodeVarintMsgPeripheral(dAtA, i, uint64(len(m.Data[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintMsgPeripheral(dAtA []byte, offset int, v uint64) int {
	offset -= sovMsgPeripheral(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *PeripheralStatusMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Kind != 0 {
		n += 1 + sovMsgPeripheral(uint64(m.Kind))
	}
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + sovMsgPeripheral(uint64(l))
	}
	if m.State != 0 {
		n += 1 + sovMsgPeripheral(uint64(m.State))
	}
	l = len(m.Message)
	if l > 0 {
		n += 1 + l + sovMsgPeripheral(uint64(l))
	}
	return n
}

func (m *PeripheralStatusListMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Peripherals) > 0 {
		for _, e := range m.Peripherals {
			l = e.Size()
			n += 1 + l + sovMsgPeripheral(uint64(l))
		}
	}
	return n
}

func (m *PeripheralOperationResultMsg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Data) > 0 {
		for _, b := range m.Data {
			l = len(b)
			n += 1 + l + sovMsgPeripheral(uint64(l))
		}
	}
	return n
}

func sovMsgPeripheral(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozMsgPeripheral(x uint64) (n int) {
	return sovMsgPeripheral(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *PeripheralStatusMsg) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&PeripheralStatusMsg{`,
		`Kind:` + fmt.Sprintf("%v", this.Kind) + `,`,
		`Name:` + fmt.Sprintf("%v", this.Name) + `,`,
		`State:` + fmt.Sprintf("%v", this.State) + `,`,
		`Message:` + fmt.Sprintf("%v", this.Message) + `,`,
		`}`,
	}, "")
	return s
}
func (this *PeripheralStatusListMsg) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForPeripherals := "[]*PeripheralStatusMsg{"
	for _, f := range this.Peripherals {
		repeatedStringForPeripherals += strings.Replace(f.String(), "PeripheralStatusMsg", "PeripheralStatusMsg", 1) + ","
	}
	repeatedStringForPeripherals += "}"
	s := strings.Join([]string{`&PeripheralStatusListMsg{`,
		`Peripherals:` + repeatedStringForPeripherals + `,`,
		`}`,
	}, "")
	return s
}
func (this *PeripheralOperationResultMsg) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&PeripheralOperationResultMsg{`,
		`Data:` + fmt.Sprintf("%v", this.Data) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringMsgPeripheral(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *PeripheralStatusMsg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMsgPeripheral
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
			return fmt.Errorf("proto: PeripheralStatusMsg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PeripheralStatusMsg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Kind", wireType)
			}
			m.Kind = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgPeripheral
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Kind |= PeripheralType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgPeripheral
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
				return ErrInvalidLengthMsgPeripheral
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field State", wireType)
			}
			m.State = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgPeripheral
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.State |= PeripheralState(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Message", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgPeripheral
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
				return ErrInvalidLengthMsgPeripheral
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Message = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMsgPeripheral(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthMsgPeripheral
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
func (m *PeripheralStatusListMsg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMsgPeripheral
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
			return fmt.Errorf("proto: PeripheralStatusListMsg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PeripheralStatusListMsg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Peripherals", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgPeripheral
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Peripherals = append(m.Peripherals, &PeripheralStatusMsg{})
			if err := m.Peripherals[len(m.Peripherals)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMsgPeripheral(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthMsgPeripheral
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
func (m *PeripheralOperationResultMsg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMsgPeripheral
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
			return fmt.Errorf("proto: PeripheralOperationResultMsg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PeripheralOperationResultMsg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgPeripheral
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = append(m.Data, make([]byte, postIndex-iNdEx))
			copy(m.Data[len(m.Data)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMsgPeripheral(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMsgPeripheral
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthMsgPeripheral
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
func skipMsgPeripheral(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMsgPeripheral
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
					return 0, ErrIntOverflowMsgPeripheral
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
					return 0, ErrIntOverflowMsgPeripheral
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
				return 0, ErrInvalidLengthMsgPeripheral
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupMsgPeripheral
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthMsgPeripheral
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthMsgPeripheral        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMsgPeripheral          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupMsgPeripheral = fmt.Errorf("proto: unexpected end of group")
)