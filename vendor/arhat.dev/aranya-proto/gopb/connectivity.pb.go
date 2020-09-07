// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: connectivity.proto

package gopb

import (
	bytes "bytes"
	context "context"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type CmdType int32

const (
	_INVALID_CMD_TYPE CmdType = 0
	CMD_NODE          CmdType = 1
	CMD_METRICS       CmdType = 2
	CMD_NETWORK       CmdType = 3
	CMD_POD           CmdType = 4
	CMD_POD_OPERATION CmdType = 5
	CMD_DEVICE        CmdType = 6
	CMD_REJECTION     CmdType = 7
	CMD_SESSION       CmdType = 8
	CMD_STORAGE       CmdType = 9
	CMD_CRED          CmdType = 10
)

var CmdType_name = map[int32]string{
	0:  "_INVALID_CMD_TYPE",
	1:  "CMD_NODE",
	2:  "CMD_METRICS",
	3:  "CMD_NETWORK",
	4:  "CMD_POD",
	5:  "CMD_POD_OPERATION",
	6:  "CMD_DEVICE",
	7:  "CMD_REJECTION",
	8:  "CMD_SESSION",
	9:  "CMD_STORAGE",
	10: "CMD_CRED",
}

var CmdType_value = map[string]int32{
	"_INVALID_CMD_TYPE": 0,
	"CMD_NODE":          1,
	"CMD_METRICS":       2,
	"CMD_NETWORK":       3,
	"CMD_POD":           4,
	"CMD_POD_OPERATION": 5,
	"CMD_DEVICE":        6,
	"CMD_REJECTION":     7,
	"CMD_SESSION":       8,
	"CMD_STORAGE":       9,
	"CMD_CRED":          10,
}

func (CmdType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_2872c2021a21e8fe, []int{0}
}

type MsgType int32

const (
	_INVALID_MSG_TYPE MsgType = 0
	MSG_STATE         MsgType = 1
	MSG_NODE          MsgType = 2
	MSG_METRICS       MsgType = 3
	MSG_NETWORK       MsgType = 4
	MSG_DEVICE        MsgType = 5
	MSG_STORAGE       MsgType = 6
	MSG_ERROR         MsgType = 7
	MSG_DATA          MsgType = 8
	MSG_CRED          MsgType = 9
	MSG_POD           MsgType = 10
)

var MsgType_name = map[int32]string{
	0:  "_INVALID_MSG_TYPE",
	1:  "MSG_STATE",
	2:  "MSG_NODE",
	3:  "MSG_METRICS",
	4:  "MSG_NETWORK",
	5:  "MSG_DEVICE",
	6:  "MSG_STORAGE",
	7:  "MSG_ERROR",
	8:  "MSG_DATA",
	9:  "MSG_CRED",
	10: "MSG_POD",
}

var MsgType_value = map[string]int32{
	"_INVALID_MSG_TYPE": 0,
	"MSG_STATE":         1,
	"MSG_NODE":          2,
	"MSG_METRICS":       3,
	"MSG_NETWORK":       4,
	"MSG_DEVICE":        5,
	"MSG_STORAGE":       6,
	"MSG_ERROR":         7,
	"MSG_DATA":          8,
	"MSG_CRED":          9,
	"MSG_POD":           10,
}

func (MsgType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_2872c2021a21e8fe, []int{1}
}

type Cmd struct {
	Kind      CmdType `protobuf:"varint,1,opt,name=kind,proto3,enum=aranya.CmdType" json:"kind,omitempty"`
	SessionId uint64  `protobuf:"varint,2,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Body      []byte  `protobuf:"bytes,11,opt,name=body,proto3" json:"body,omitempty"`
}

func (m *Cmd) Reset()      { *m = Cmd{} }
func (*Cmd) ProtoMessage() {}
func (*Cmd) Descriptor() ([]byte, []int) {
	return fileDescriptor_2872c2021a21e8fe, []int{0}
}
func (m *Cmd) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Cmd) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Cmd.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Cmd) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Cmd.Merge(m, src)
}
func (m *Cmd) XXX_Size() int {
	return m.Size()
}
func (m *Cmd) XXX_DiscardUnknown() {
	xxx_messageInfo_Cmd.DiscardUnknown(m)
}

var xxx_messageInfo_Cmd proto.InternalMessageInfo

func (m *Cmd) GetKind() CmdType {
	if m != nil {
		return m.Kind
	}
	return _INVALID_CMD_TYPE
}

func (m *Cmd) GetSessionId() uint64 {
	if m != nil {
		return m.SessionId
	}
	return 0
}

func (m *Cmd) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

type Msg struct {
	Kind      MsgType `protobuf:"varint,1,opt,name=kind,proto3,enum=aranya.MsgType" json:"kind,omitempty"`
	SessionId uint64  `protobuf:"varint,2,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Completed bool    `protobuf:"varint,3,opt,name=completed,proto3" json:"completed,omitempty"`
	// the id used for online message
	OnlineId string `protobuf:"bytes,4,opt,name=online_id,json=onlineId,proto3" json:"online_id,omitempty"`
	Body     []byte `protobuf:"bytes,11,opt,name=body,proto3" json:"body,omitempty"`
}

func (m *Msg) Reset()      { *m = Msg{} }
func (*Msg) ProtoMessage() {}
func (*Msg) Descriptor() ([]byte, []int) {
	return fileDescriptor_2872c2021a21e8fe, []int{1}
}
func (m *Msg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Msg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Msg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Msg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Msg.Merge(m, src)
}
func (m *Msg) XXX_Size() int {
	return m.Size()
}
func (m *Msg) XXX_DiscardUnknown() {
	xxx_messageInfo_Msg.DiscardUnknown(m)
}

var xxx_messageInfo_Msg proto.InternalMessageInfo

func (m *Msg) GetKind() MsgType {
	if m != nil {
		return m.Kind
	}
	return _INVALID_MSG_TYPE
}

func (m *Msg) GetSessionId() uint64 {
	if m != nil {
		return m.SessionId
	}
	return 0
}

func (m *Msg) GetCompleted() bool {
	if m != nil {
		return m.Completed
	}
	return false
}

func (m *Msg) GetOnlineId() string {
	if m != nil {
		return m.OnlineId
	}
	return ""
}

func (m *Msg) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

func init() {
	proto.RegisterEnum("aranya.CmdType", CmdType_name, CmdType_value)
	proto.RegisterEnum("aranya.MsgType", MsgType_name, MsgType_value)
	proto.RegisterType((*Cmd)(nil), "aranya.Cmd")
	proto.RegisterType((*Msg)(nil), "aranya.Msg")
}

func init() { proto.RegisterFile("connectivity.proto", fileDescriptor_2872c2021a21e8fe) }

var fileDescriptor_2872c2021a21e8fe = []byte{
	// 514 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x93, 0xc1, 0x8a, 0xd3, 0x40,
	0x1c, 0xc6, 0x33, 0xdb, 0x6c, 0x9b, 0xfc, 0xdb, 0xdd, 0x9d, 0x1d, 0x10, 0x8a, 0xab, 0x43, 0x59,
	0x3d, 0x84, 0x05, 0xbb, 0xb2, 0xfa, 0x02, 0x31, 0x19, 0x96, 0xa8, 0x6d, 0xea, 0x24, 0xac, 0x28,
	0x48, 0x69, 0x9b, 0x50, 0x83, 0xdb, 0xa4, 0x34, 0x61, 0x21, 0x37, 0x1f, 0xc1, 0xa3, 0x8f, 0xe0,
	0x23, 0xf8, 0x06, 0x7a, 0xec, 0x71, 0x8f, 0x36, 0xbd, 0x78, 0xdc, 0x47, 0x90, 0x49, 0x33, 0x5a,
	0x41, 0x61, 0x6f, 0xf3, 0xfd, 0x66, 0xf2, 0xf1, 0x7d, 0x7f, 0xfe, 0x01, 0x32, 0x49, 0xe2, 0x38,
	0x9c, 0x64, 0xd1, 0x55, 0x94, 0xe5, 0xdd, 0xf9, 0x22, 0xc9, 0x12, 0x52, 0x1f, 0x2d, 0x46, 0x71,
	0x3e, 0x3a, 0x7e, 0x07, 0x35, 0x6b, 0x16, 0x90, 0x07, 0xa0, 0x7e, 0x88, 0xe2, 0xa0, 0x8d, 0x3a,
	0xc8, 0xd8, 0x3f, 0x3b, 0xe8, 0x6e, 0x6e, 0xbb, 0xd6, 0x2c, 0xf0, 0xf3, 0x79, 0xc8, 0xcb, 0x4b,
	0x72, 0x1f, 0x20, 0x0d, 0xd3, 0x34, 0x4a, 0xe2, 0x61, 0x14, 0xb4, 0x77, 0x3a, 0xc8, 0x50, 0xb9,
	0x5e, 0x11, 0x27, 0x20, 0x04, 0xd4, 0x71, 0x12, 0xe4, 0xed, 0x66, 0x07, 0x19, 0x2d, 0x5e, 0x9e,
	0x8f, 0x3f, 0x23, 0xa8, 0xf5, 0xd2, 0xe9, 0xff, 0xfc, 0x7b, 0xe9, 0xf4, 0xf6, 0xfe, 0xf7, 0x40,
	0x9f, 0x24, 0xb3, 0xf9, 0x65, 0x98, 0x85, 0x41, 0xbb, 0xd6, 0x41, 0x86, 0xc6, 0xff, 0x00, 0x72,
	0x04, 0x7a, 0x12, 0x5f, 0x46, 0x71, 0x28, 0xbe, 0x55, 0x3b, 0xc8, 0xd0, 0xb9, 0xb6, 0x01, 0xff,
	0x8e, 0x76, 0xf2, 0x0d, 0x41, 0xa3, 0xea, 0x47, 0xee, 0xc0, 0xe1, 0xd0, 0xe9, 0x5f, 0x98, 0x2f,
	0x1d, 0x7b, 0x68, 0xf5, 0xec, 0xa1, 0xff, 0x66, 0xc0, 0xb0, 0x42, 0x5a, 0xa0, 0x09, 0xd5, 0x77,
	0x6d, 0x86, 0x11, 0x39, 0x80, 0xa6, 0x50, 0x3d, 0xe6, 0x73, 0xc7, 0xf2, 0xf0, 0x8e, 0x04, 0x7d,
	0xe6, 0xbf, 0x76, 0xf9, 0x0b, 0x5c, 0x23, 0x4d, 0x68, 0x08, 0x30, 0x70, 0x6d, 0xac, 0x0a, 0xcf,
	0x4a, 0x0c, 0xdd, 0x01, 0xe3, 0xa6, 0xef, 0xb8, 0x7d, 0xbc, 0x4b, 0xf6, 0x01, 0x04, 0xb6, 0xd9,
	0x85, 0x63, 0x31, 0x5c, 0x27, 0x87, 0xb0, 0x27, 0x34, 0x67, 0xcf, 0x99, 0x55, 0x3e, 0x69, 0x48,
	0x5f, 0x8f, 0x79, 0x9e, 0x00, 0xda, 0x6f, 0xe0, 0xbb, 0xdc, 0x3c, 0x67, 0x58, 0x97, 0xc1, 0x2c,
	0xce, 0x6c, 0x0c, 0x27, 0x5f, 0x11, 0x34, 0xaa, 0x49, 0xfe, 0xd5, 0xa4, 0xe7, 0x9d, 0xcb, 0x26,
	0x7b, 0xa0, 0x0b, 0xe5, 0xf9, 0xa6, 0x2f, 0xaa, 0xb4, 0x40, 0x13, 0xb2, 0x2c, 0x56, 0xf6, 0x10,
	0x4a, 0x16, 0xab, 0x49, 0x20, 0x8b, 0xa9, 0x22, 0xb4, 0x00, 0x55, 0xe8, 0x5d, 0xf9, 0x40, 0x06,
	0xaa, 0x4b, 0x7f, 0xc6, 0xb9, 0xcb, 0x71, 0x43, 0xfa, 0xdb, 0xa6, 0x6f, 0x62, 0x4d, 0xaa, 0x32,
	0xad, 0x2e, 0x86, 0x24, 0x94, 0x18, 0x12, 0x9c, 0x3d, 0x85, 0x96, 0xb5, 0xb5, 0x9c, 0xe4, 0x21,
	0xa8, 0x5e, 0x1e, 0x4f, 0x48, 0x73, 0x6b, 0x43, 0xee, 0x36, 0xb7, 0xd6, 0xd1, 0x40, 0x8f, 0xd1,
	0xb3, 0x57, 0xcb, 0x15, 0x55, 0xae, 0x57, 0x54, 0xb9, 0x59, 0x51, 0xf4, 0xb1, 0xa0, 0xe8, 0x4b,
	0x41, 0xd1, 0xf7, 0x82, 0xa2, 0x65, 0x41, 0xd1, 0x8f, 0x82, 0xa2, 0x9f, 0x05, 0x55, 0x6e, 0x0a,
	0x8a, 0x3e, 0xad, 0xa9, 0xb2, 0x5c, 0x53, 0xe5, 0x7a, 0x4d, 0x95, 0xb7, 0x47, 0xa3, 0xc5, 0xfb,
	0x51, 0xd6, 0x0d, 0xc2, 0xab, 0xd3, 0x8d, 0xdf, 0xa3, 0xf2, 0x57, 0x38, 0x9d, 0x26, 0xf3, 0xf1,
	0xb8, 0x5e, 0x9e, 0x9f, 0xfc, 0x0a, 0x00, 0x00, 0xff, 0xff, 0xb1, 0x44, 0xef, 0x48, 0x2c, 0x03,
	0x00, 0x00,
}

func (x CmdType) String() string {
	s, ok := CmdType_name[int32(x)]
	if ok {
		return s
	}
	return strconv.Itoa(int(x))
}
func (x MsgType) String() string {
	s, ok := MsgType_name[int32(x)]
	if ok {
		return s
	}
	return strconv.Itoa(int(x))
}
func (this *Cmd) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Cmd)
	if !ok {
		that2, ok := that.(Cmd)
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
	if this.SessionId != that1.SessionId {
		return false
	}
	if !bytes.Equal(this.Body, that1.Body) {
		return false
	}
	return true
}
func (this *Msg) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Msg)
	if !ok {
		that2, ok := that.(Msg)
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
	if this.SessionId != that1.SessionId {
		return false
	}
	if this.Completed != that1.Completed {
		return false
	}
	if this.OnlineId != that1.OnlineId {
		return false
	}
	if !bytes.Equal(this.Body, that1.Body) {
		return false
	}
	return true
}
func (this *Cmd) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 7)
	s = append(s, "&gopb.Cmd{")
	s = append(s, "Kind: "+fmt.Sprintf("%#v", this.Kind)+",\n")
	s = append(s, "SessionId: "+fmt.Sprintf("%#v", this.SessionId)+",\n")
	s = append(s, "Body: "+fmt.Sprintf("%#v", this.Body)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *Msg) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 9)
	s = append(s, "&gopb.Msg{")
	s = append(s, "Kind: "+fmt.Sprintf("%#v", this.Kind)+",\n")
	s = append(s, "SessionId: "+fmt.Sprintf("%#v", this.SessionId)+",\n")
	s = append(s, "Completed: "+fmt.Sprintf("%#v", this.Completed)+",\n")
	s = append(s, "OnlineId: "+fmt.Sprintf("%#v", this.OnlineId)+",\n")
	s = append(s, "Body: "+fmt.Sprintf("%#v", this.Body)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringConnectivity(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ConnectivityClient is the client API for Connectivity service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ConnectivityClient interface {
	Sync(ctx context.Context, opts ...grpc.CallOption) (Connectivity_SyncClient, error)
}

type connectivityClient struct {
	cc *grpc.ClientConn
}

func NewConnectivityClient(cc *grpc.ClientConn) ConnectivityClient {
	return &connectivityClient{cc}
}

func (c *connectivityClient) Sync(ctx context.Context, opts ...grpc.CallOption) (Connectivity_SyncClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Connectivity_serviceDesc.Streams[0], "/aranya.Connectivity/Sync", opts...)
	if err != nil {
		return nil, err
	}
	x := &connectivitySyncClient{stream}
	return x, nil
}

type Connectivity_SyncClient interface {
	Send(*Msg) error
	Recv() (*Cmd, error)
	grpc.ClientStream
}

type connectivitySyncClient struct {
	grpc.ClientStream
}

func (x *connectivitySyncClient) Send(m *Msg) error {
	return x.ClientStream.SendMsg(m)
}

func (x *connectivitySyncClient) Recv() (*Cmd, error) {
	m := new(Cmd)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ConnectivityServer is the server API for Connectivity service.
type ConnectivityServer interface {
	Sync(Connectivity_SyncServer) error
}

// UnimplementedConnectivityServer can be embedded to have forward compatible implementations.
type UnimplementedConnectivityServer struct {
}

func (*UnimplementedConnectivityServer) Sync(srv Connectivity_SyncServer) error {
	return status.Errorf(codes.Unimplemented, "method Sync not implemented")
}

func RegisterConnectivityServer(s *grpc.Server, srv ConnectivityServer) {
	s.RegisterService(&_Connectivity_serviceDesc, srv)
}

func _Connectivity_Sync_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ConnectivityServer).Sync(&connectivitySyncServer{stream})
}

type Connectivity_SyncServer interface {
	Send(*Cmd) error
	Recv() (*Msg, error)
	grpc.ServerStream
}

type connectivitySyncServer struct {
	grpc.ServerStream
}

func (x *connectivitySyncServer) Send(m *Cmd) error {
	return x.ServerStream.SendMsg(m)
}

func (x *connectivitySyncServer) Recv() (*Msg, error) {
	m := new(Msg)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Connectivity_serviceDesc = grpc.ServiceDesc{
	ServiceName: "aranya.Connectivity",
	HandlerType: (*ConnectivityServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Sync",
			Handler:       _Connectivity_Sync_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "connectivity.proto",
}

func (m *Cmd) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Cmd) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Cmd) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Body) > 0 {
		i -= len(m.Body)
		copy(dAtA[i:], m.Body)
		i = encodeVarintConnectivity(dAtA, i, uint64(len(m.Body)))
		i--
		dAtA[i] = 0x5a
	}
	if m.SessionId != 0 {
		i = encodeVarintConnectivity(dAtA, i, uint64(m.SessionId))
		i--
		dAtA[i] = 0x10
	}
	if m.Kind != 0 {
		i = encodeVarintConnectivity(dAtA, i, uint64(m.Kind))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Msg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Msg) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Msg) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Body) > 0 {
		i -= len(m.Body)
		copy(dAtA[i:], m.Body)
		i = encodeVarintConnectivity(dAtA, i, uint64(len(m.Body)))
		i--
		dAtA[i] = 0x5a
	}
	if len(m.OnlineId) > 0 {
		i -= len(m.OnlineId)
		copy(dAtA[i:], m.OnlineId)
		i = encodeVarintConnectivity(dAtA, i, uint64(len(m.OnlineId)))
		i--
		dAtA[i] = 0x22
	}
	if m.Completed {
		i--
		if m.Completed {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x18
	}
	if m.SessionId != 0 {
		i = encodeVarintConnectivity(dAtA, i, uint64(m.SessionId))
		i--
		dAtA[i] = 0x10
	}
	if m.Kind != 0 {
		i = encodeVarintConnectivity(dAtA, i, uint64(m.Kind))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintConnectivity(dAtA []byte, offset int, v uint64) int {
	offset -= sovConnectivity(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Cmd) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Kind != 0 {
		n += 1 + sovConnectivity(uint64(m.Kind))
	}
	if m.SessionId != 0 {
		n += 1 + sovConnectivity(uint64(m.SessionId))
	}
	l = len(m.Body)
	if l > 0 {
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}

func (m *Msg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Kind != 0 {
		n += 1 + sovConnectivity(uint64(m.Kind))
	}
	if m.SessionId != 0 {
		n += 1 + sovConnectivity(uint64(m.SessionId))
	}
	if m.Completed {
		n += 2
	}
	l = len(m.OnlineId)
	if l > 0 {
		n += 1 + l + sovConnectivity(uint64(l))
	}
	l = len(m.Body)
	if l > 0 {
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}

func sovConnectivity(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozConnectivity(x uint64) (n int) {
	return sovConnectivity(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *Cmd) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Cmd{`,
		`Kind:` + fmt.Sprintf("%v", this.Kind) + `,`,
		`SessionId:` + fmt.Sprintf("%v", this.SessionId) + `,`,
		`Body:` + fmt.Sprintf("%v", this.Body) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Msg) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Msg{`,
		`Kind:` + fmt.Sprintf("%v", this.Kind) + `,`,
		`SessionId:` + fmt.Sprintf("%v", this.SessionId) + `,`,
		`Completed:` + fmt.Sprintf("%v", this.Completed) + `,`,
		`OnlineId:` + fmt.Sprintf("%v", this.OnlineId) + `,`,
		`Body:` + fmt.Sprintf("%v", this.Body) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringConnectivity(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *Cmd) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConnectivity
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
			return fmt.Errorf("proto: Cmd: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Cmd: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Kind", wireType)
			}
			m.Kind = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Kind |= CmdType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SessionId", wireType)
			}
			m.SessionId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SessionId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Body", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
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
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Body = append(m.Body[:0], dAtA[iNdEx:postIndex]...)
			if m.Body == nil {
				m.Body = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConnectivity(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConnectivity
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConnectivity
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
func (m *Msg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConnectivity
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
			return fmt.Errorf("proto: Msg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Msg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Kind", wireType)
			}
			m.Kind = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Kind |= MsgType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SessionId", wireType)
			}
			m.SessionId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SessionId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Completed", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Completed = bool(v != 0)
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field OnlineId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
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
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.OnlineId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Body", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
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
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Body = append(m.Body[:0], dAtA[iNdEx:postIndex]...)
			if m.Body == nil {
				m.Body = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConnectivity(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConnectivity
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConnectivity
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
func skipConnectivity(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowConnectivity
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
					return 0, ErrIntOverflowConnectivity
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
					return 0, ErrIntOverflowConnectivity
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
				return 0, ErrInvalidLengthConnectivity
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupConnectivity
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthConnectivity
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthConnectivity        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowConnectivity          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupConnectivity = fmt.Errorf("proto: unexpected end of group")
)
