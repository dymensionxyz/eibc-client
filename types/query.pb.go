// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: dymensionxyz/dymension/rollapp/query.proto

package types

import (
	context "context"
	fmt "fmt"
	io "io"
	math "math"
	math_bits "math/bits"

	_ "github.com/cosmos/gogoproto/gogoproto"
	grpc1 "github.com/cosmos/gogoproto/grpc"
	proto "github.com/cosmos/gogoproto/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	grpc "google.golang.org/grpc"
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

type QueryGetLatestHeightRequest struct {
	RollappId string `protobuf:"bytes,1,opt,name=rollappId,proto3" json:"rollappId,omitempty"`
	Finalized bool   `protobuf:"varint,2,opt,name=finalized,proto3" json:"finalized,omitempty"`
}

func (m *QueryGetLatestHeightRequest) Reset()         { *m = QueryGetLatestHeightRequest{} }
func (m *QueryGetLatestHeightRequest) String() string { return proto.CompactTextString(m) }
func (*QueryGetLatestHeightRequest) ProtoMessage()    {}
func (*QueryGetLatestHeightRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_00a0238fb38306fa, []int{4}
}
func (m *QueryGetLatestHeightRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *QueryGetLatestHeightRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_QueryGetLatestHeightRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *QueryGetLatestHeightRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_QueryGetLatestHeightRequest.Merge(m, src)
}
func (m *QueryGetLatestHeightRequest) XXX_Size() int {
	return m.Size()
}
func (m *QueryGetLatestHeightRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_QueryGetLatestHeightRequest.DiscardUnknown(m)
}

var xxx_messageInfo_QueryGetLatestHeightRequest proto.InternalMessageInfo

func (m *QueryGetLatestHeightRequest) GetRollappId() string {
	if m != nil {
		return m.RollappId
	}
	return ""
}

func (m *QueryGetLatestHeightRequest) GetFinalized() bool {
	if m != nil {
		return m.Finalized
	}
	return false
}

type QueryGetLatestHeightResponse struct {
	Height uint64 `protobuf:"varint,1,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *QueryGetLatestHeightResponse) Reset()         { *m = QueryGetLatestHeightResponse{} }
func (m *QueryGetLatestHeightResponse) String() string { return proto.CompactTextString(m) }
func (*QueryGetLatestHeightResponse) ProtoMessage()    {}
func (*QueryGetLatestHeightResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_00a0238fb38306fa, []int{5}
}
func (m *QueryGetLatestHeightResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *QueryGetLatestHeightResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_QueryGetLatestHeightResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *QueryGetLatestHeightResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_QueryGetLatestHeightResponse.Merge(m, src)
}
func (m *QueryGetLatestHeightResponse) XXX_Size() int {
	return m.Size()
}
func (m *QueryGetLatestHeightResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_QueryGetLatestHeightResponse.DiscardUnknown(m)
}

var xxx_messageInfo_QueryGetLatestHeightResponse proto.InternalMessageInfo

func (m *QueryGetLatestHeightResponse) GetHeight() uint64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func init() {
	proto.RegisterType((*QueryGetLatestHeightRequest)(nil), "dymensionxyz.dymension.rollapp.QueryGetLatestHeightRequest")
	proto.RegisterType((*QueryGetLatestHeightResponse)(nil), "dymensionxyz.dymension.rollapp.QueryGetLatestHeightResponse")
}

func init() {
	proto.RegisterFile("dymensionxyz/dymension/rollapp/query.proto", fileDescriptor_00a0238fb38306fa)
}

var fileDescriptor_00a0238fb38306fa = []byte{
	// 1199 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x58, 0xcf, 0x6f, 0x1b, 0x45,
	0x14, 0xce, 0x38, 0x8e, 0x13, 0xbf, 0x14, 0x35, 0x9a, 0x86, 0x10, 0xdc, 0xe0, 0x26, 0x8b, 0xd4,
	0xba, 0x05, 0x79, 0xe5, 0x04, 0x37, 0x8d, 0x42, 0x4a, 0x1d, 0xa5, 0x09, 0x29, 0xa5, 0x84, 0x0d,
	0x14, 0x01, 0x42, 0xd6, 0xba, 0x3b, 0x71, 0x16, 0xad, 0x77, 0xb7, 0x3b, 0x9b, 0x28, 0x6e, 0x14,
	0x09, 0x21, 0xce, 0x08, 0x89, 0x7b, 0x25, 0xfe, 0x01, 0xae, 0x9c, 0x11, 0x97, 0x08, 0x71, 0xa8,
	0xc4, 0x01, 0x2e, 0x20, 0x94, 0xf0, 0x3f, 0x70, 0x45, 0x9e, 0x79, 0xbb, 0xfe, 0x11, 0xdb, 0xbb,
	0x71, 0x7b, 0xb2, 0x67, 0xf2, 0xde, 0x37, 0xdf, 0x37, 0xf3, 0x7e, 0x39, 0x70, 0xc3, 0xa8, 0xd7,
	0x98, 0xcd, 0x4d, 0xc7, 0x3e, 0xa8, 0x3f, 0x51, 0xc3, 0x85, 0xea, 0x39, 0x96, 0xa5, 0xbb, 0xae,
	0xfa, 0x78, 0x8f, 0x79, 0xf5, 0xbc, 0xeb, 0x39, 0xbe, 0x43, 0xb3, 0xad, 0xb6, 0xf9, 0x70, 0x91,
	0x47, 0xdb, 0xcc, 0x64, 0xd5, 0xa9, 0x3a, 0xc2, 0x54, 0x6d, 0x7c, 0x93, 0x5e, 0x99, 0x99, 0xaa,
	0xe3, 0x54, 0x2d, 0xa6, 0xea, 0xae, 0xa9, 0xea, 0xb6, 0xed, 0xf8, 0xba, 0x6f, 0x3a, 0x36, 0xc7,
	0xbf, 0xde, 0x78, 0xe4, 0xf0, 0x9a, 0xc3, 0xd5, 0x8a, 0xce, 0x99, 0x3c, 0x4c, 0xdd, 0x2f, 0x54,
	0x98, 0xaf, 0x17, 0x54, 0x57, 0xaf, 0x9a, 0xb6, 0x30, 0x46, 0xdb, 0x37, 0x22, 0xb8, 0xba, 0xba,
	0xa7, 0xd7, 0x02, 0xe0, 0x37, 0x23, 0x8c, 0xf1, 0x13, 0xad, 0xd5, 0x08, 0x6b, 0xee, 0xeb, 0x3e,
	0x2b, 0x9b, 0xf6, 0x4e, 0xa0, 0x2a, 0x17, 0xe1, 0xd0, 0x84, 0xbe, 0x15, 0x61, 0x59, 0x65, 0x36,
	0xe3, 0x26, 0x2f, 0x57, 0x3c, 0xd3, 0xa8, 0xb2, 0xb2, 0xa1, 0xfb, 0xba, 0xf4, 0x54, 0x26, 0x81,
	0x7e, 0xd8, 0xb8, 0x91, 0x2d, 0xa1, 0x4b, 0x63, 0x8f, 0xf7, 0x18, 0xf7, 0x95, 0xcf, 0xe1, 0x52,
	0xdb, 0x2e, 0x77, 0x1d, 0x9b, 0x33, 0xba, 0x06, 0x29, 0xa9, 0x7f, 0x9a, 0xcc, 0x92, 0xdc, 0xf8,
	0xfc, 0xd5, 0x7c, 0xff, 0xd7, 0xca, 0x4b, 0xff, 0xd5, 0xe4, 0xf1, 0xdf, 0x57, 0x86, 0x34, 0xf4,
	0x55, 0xb6, 0x61, 0x4a, 0x80, 0x6f, 0x30, 0x5f, 0x93, 0x76, 0x78, 0x2c, 0x9d, 0x81, 0x34, 0x7a,
	0x6e, 0x1a, 0xe2, 0x88, 0xb4, 0xd6, 0xdc, 0xa0, 0x97, 0x21, 0xed, 0xd4, 0x4c, 0xbf, 0xac, 0xbb,
	0x2e, 0x9f, 0x4e, 0xcc, 0x92, 0xdc, 0x98, 0x36, 0xd6, 0xd8, 0x28, 0xb9, 0x2e, 0x57, 0x3e, 0x86,
	0x6c, 0x07, 0xe8, 0x6a, 0xfd, 0xee, 0xe6, 0x56, 0xa1, 0x58, 0x0c, 0xc0, 0xa7, 0x20, 0xc5, 0x4c,
	0xb7, 0x50, 0x2c, 0x0a, 0xe4, 0xa4, 0x86, 0xab, 0xfe, 0xb0, 0x9f, 0xc2, 0xe5, 0x00, 0xf6, 0xbe,
	0xee, 0x33, 0xee, 0xbf, 0xcb, 0xcc, 0xea, 0xae, 0x1f, 0x8f, 0xf0, 0x0c, 0xa4, 0x77, 0x4c, 0x5b,
	0xb7, 0xcc, 0x27, 0xcc, 0x40, 0xe4, 0xe6, 0x86, 0x72, 0x13, 0x66, 0xba, 0x43, 0xe3, 0x65, 0x4f,
	0x41, 0x6a, 0x57, 0xec, 0x04, 0x7c, 0xe5, 0x4a, 0xf9, 0x02, 0xae, 0xb4, 0xfb, 0x6d, 0x37, 0xe2,
	0x66, 0xd3, 0x36, 0xd8, 0xc1, 0x8b, 0xa0, 0x75, 0x00, 0xb3, 0xbd, 0xe1, 0x91, 0xda, 0x47, 0x00,
	0x3c, 0xdc, 0xc5, 0x58, 0xc8, 0x47, 0xc5, 0x02, 0xe2, 0xec, 0x38, 0xc2, 0x0b, 0x63, 0xa2, 0x05,
	0x47, 0xf9, 0x8f, 0xc0, 0x2b, 0x67, 0x02, 0x03, 0x4f, 0xdc, 0x80, 0x51, 0xc4, 0xc1, 0xe3, 0xae,
	0x45, 0x1d, 0x17, 0x44, 0x81, 0x3c, 0x27, 0xf0, 0xa6, 0x0f, 0x60, 0x94, 0xef, 0xd5, 0x6a, 0xba,
	0x57, 0x9f, 0x4e, 0xc5, 0xe3, 0x8d, 0x40, 0xdb, 0xd2, 0x2b, 0xc0, 0x43, 0x10, 0xba, 0x02, 0x49,
	0x11, 0x38, 0xa3, 0xb3, 0xc3, 0xb9, 0xf1, 0xf9, 0xd7, 0xa3, 0xc0, 0x4a, 0xc8, 0x88, 0x68, 0xc2,
	0xed, 0x5e, 0x72, 0x2c, 0x31, 0x91, 0x52, 0x8e, 0x30, 0x23, 0x4a, 0x96, 0xd5, 0x91, 0x11, 0xeb,
	0x00, 0xcd, 0x12, 0x15, 0x66, 0x9d, 0xac, 0x67, 0xf9, 0x46, 0x3d, 0xcb, 0xcb, 0xe2, 0x89, 0xf5,
	0x2c, 0xbf, 0xa5, 0x57, 0x19, 0xfa, 0x6a, 0x2d, 0x9e, 0xfd, 0x83, 0xfc, 0xe7, 0xe0, 0xe2, 0x5b,
	0xcf, 0xc7, 0x8b, 0xff, 0xa4, 0x79, 0xf1, 0xc3, 0x42, 0xe2, 0x62, 0x94, 0xc4, 0x1e, 0x4f, 0xd8,
	0xf9, 0x10, 0x1b, 0x6d, 0xca, 0x12, 0xf8, 0xa8, 0x51, 0xca, 0x24, 0x56, 0xab, 0xb4, 0x7b, 0xc9,
	0x31, 0x32, 0x91, 0x50, 0xbe, 0x21, 0x30, 0x1d, 0x9c, 0x1c, 0x46, 0x5a, 0xbc, 0x7c, 0x98, 0x84,
	0x11, 0x53, 0x04, 0x72, 0x42, 0xe4, 0x99, 0x5c, 0xb4, 0xa4, 0xdf, 0x70, 0x6b, 0xfa, 0xb5, 0x67,
	0x4f, 0xb2, 0x33, 0x7b, 0xbe, 0x84, 0x57, 0xbb, 0xb0, 0xc0, 0xbb, 0x7c, 0x1f, 0xd2, 0x3c, 0xd8,
	0xc4, 0xb7, 0xbc, 0x1e, 0x3b, 0x6b, 0xf0, 0xfe, 0x9a, 0x08, 0x0d, 0xc9, 0xb2, 0x82, 0x68, 0xac,
	0x6a, 0x72, 0x9f, 0x79, 0xcc, 0x58, 0x63, 0xb6, 0x13, 0x56, 0xf1, 0x08, 0xd9, 0xeb, 0x5d, 0x1e,
	0x60, 0x80, 0xd0, 0x52, 0xbe, 0x22, 0xf0, 0x5a, 0x0f, 0x1a, 0xcd, 0x4a, 0x66, 0x88, 0x9d, 0x69,
	0x32, 0x3b, 0x9c, 0x4b, 0x6b, 0xb8, 0x7a, 0x61, 0x21, 0xa0, 0xcc, 0x61, 0x49, 0xfc, 0xa0, 0xc2,
	0x1d, 0x8b, 0xf9, 0x6c, 0x4d, 0xdb, 0x7e, 0xc8, 0xbc, 0xc6, 0x3d, 0x86, 0x1d, 0xed, 0x2e, 0x96,
	0xb5, 0xae, 0x26, 0xc8, 0x73, 0x0e, 0x2e, 0x18, 0x1e, 0x2f, 0xef, 0xe3, 0xbe, 0x60, 0xfb, 0x92,
	0x36, 0x6e, 0x78, 0x3c, 0x30, 0x55, 0xbe, 0x25, 0x30, 0x27, 0x70, 0x1e, 0xea, 0x96, 0x69, 0xe8,
	0x3e, 0xdb, 0x90, 0x9d, 0x75, 0x55, 0x34, 0xd6, 0x78, 0x17, 0xff, 0x1e, 0x24, 0x1b, 0x0d, 0x18,
	0x05, 0x17, 0xa2, 0x22, 0xa0, 0xed, 0x84, 0x35, 0xdd, 0xd7, 0x31, 0x12, 0x04, 0x88, 0x72, 0x1f,
	0x94, 0x7e, 0x7c, 0x50, 0xd9, 0x24, 0x8c, 0xec, 0x37, 0x0c, 0x04, 0x99, 0x31, 0x4d, 0x2e, 0xe8,
	0x04, 0x0c, 0x33, 0xcf, 0x13, 0x3c, 0xd2, 0x5a, 0xe3, 0xeb, 0xfc, 0xd3, 0x8b, 0x30, 0x22, 0xe0,
	0xe8, 0x0f, 0x04, 0x52, 0xb2, 0x7b, 0xd3, 0xf9, 0x58, 0x19, 0xdf, 0x36, 0x40, 0x64, 0x16, 0xce,
	0xe5, 0x23, 0x59, 0x2a, 0xf9, 0xaf, 0x7f, 0xff, 0xf7, 0xfb, 0x44, 0x8e, 0x5e, 0x55, 0x63, 0x0d,
	0x61, 0xf4, 0x27, 0x02, 0xa3, 0x58, 0x65, 0xe8, 0xcd, 0x73, 0x97, 0x25, 0x49, 0x74, 0xd0, 0x72,
	0xa6, 0x2c, 0x0b, 0xb2, 0x45, 0xba, 0xa0, 0xc6, 0x1b, 0x02, 0xd5, 0xc3, 0x30, 0x02, 0x8e, 0xe8,
	0x2f, 0x04, 0x2e, 0x76, 0x8c, 0x29, 0xf4, 0xf6, 0x39, 0x99, 0x74, 0xcc, 0x37, 0x83, 0x2b, 0x59,
	0x14, 0x4a, 0x0a, 0x54, 0x8d, 0x52, 0x22, 0x07, 0x26, 0xf5, 0x50, 0x7e, 0x1e, 0xd1, 0x1f, 0x09,
	0x00, 0x82, 0x95, 0x2c, 0x2b, 0xe6, 0x13, 0x9c, 0xe9, 0x71, 0x31, 0x89, 0x9f, 0xed, 0x4d, 0x8a,
	0x2a, 0x88, 0x5f, 0xa7, 0xd7, 0x62, 0x3e, 0x01, 0xfd, 0x8d, 0xc0, 0x85, 0xd6, 0x59, 0x8b, 0x2e,
	0xc7, 0xbd, 0xb3, 0x2e, 0xc3, 0x5f, 0xe6, 0xed, 0xc1, 0x9c, 0x91, 0x7c, 0x49, 0x90, 0x5f, 0xa6,
	0x4b, 0x51, 0xe4, 0x2d, 0xe1, 0x5d, 0x96, 0xed, 0xa7, 0x2d, 0x8a, 0xfe, 0x22, 0x30, 0xd1, 0x39,
	0xa3, 0xd1, 0x77, 0xce, 0xc7, 0xea, 0xcc, 0xf0, 0x98, 0xb9, 0x33, 0x38, 0x00, 0x4a, 0x5b, 0x17,
	0xd2, 0xee, 0xd0, 0xdb, 0x31, 0xa5, 0x05, 0x3f, 0x7c, 0x0c, 0x76, 0xd0, 0xa6, 0xef, 0x98, 0x40,
	0x3a, 0xec, 0x7f, 0xf4, 0x56, 0x5c, 0x5e, 0x9d, 0xed, 0x3f, 0xb3, 0x34, 0x80, 0xe7, 0x79, 0xa5,
	0x34, 0x7f, 0xbc, 0xb5, 0x4a, 0x50, 0x0f, 0x85, 0xaa, 0x23, 0xfa, 0x2b, 0x81, 0x89, 0xce, 0xfe,
	0x48, 0xe3, 0x05, 0x50, 0x8f, 0xee, 0x9e, 0x59, 0x19, 0xd0, 0x1b, 0x95, 0x2d, 0x09, 0x65, 0x0b,
	0xb4, 0x10, 0x99, 0x3c, 0x21, 0x42, 0x19, 0xfb, 0xf6, 0x1f, 0x04, 0x2e, 0x75, 0xe9, 0xa3, 0x31,
	0x43, 0xaf, 0x77, 0x93, 0x8e, 0x19, 0x7a, 0x7d, 0x5a, 0xb8, 0xb2, 0x22, 0x54, 0x2d, 0xd2, 0x62,
	0x94, 0x2a, 0x07, 0x41, 0xca, 0xad, 0x1d, 0x9f, 0x3e, 0x25, 0xf0, 0x72, 0xd7, 0x4e, 0x4a, 0x4b,
	0xb1, 0xa8, 0xf5, 0x9b, 0x0a, 0x32, 0xab, 0xcf, 0x03, 0x81, 0x43, 0xf4, 0x83, 0xe3, 0x93, 0x2c,
	0x79, 0x76, 0x92, 0x25, 0xff, 0x9c, 0x64, 0xc9, 0x77, 0xa7, 0xd9, 0xa1, 0x67, 0xa7, 0xd9, 0xa1,
	0x3f, 0x4f, 0xb3, 0x43, 0x9f, 0xbd, 0x55, 0x35, 0xfd, 0xdd, 0xbd, 0x4a, 0xfe, 0x91, 0x53, 0xeb,
	0xa5, 0x7d, 0x7f, 0x41, 0x3d, 0x08, 0x2f, 0xc0, 0xaf, 0xbb, 0x8c, 0x57, 0x52, 0xe2, 0xbf, 0x00,
	0x0b, 0xff, 0x07, 0x00, 0x00, 0xff, 0xff, 0xf8, 0x0a, 0xfa, 0xcf, 0xa3, 0x11, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// QueryClient is the client API for Query service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type QueryClient interface {
	// Queries a LatestHeight by rollapp-id.
	LatestHeight(ctx context.Context, in *QueryGetLatestHeightRequest, opts ...grpc.CallOption) (*QueryGetLatestHeightResponse, error)
}

type queryClient struct {
	cc grpc1.ClientConn
}

func NewQueryClient(cc grpc1.ClientConn) QueryClient {
	return &queryClient{cc}
}

func (c *queryClient) LatestHeight(ctx context.Context, in *QueryGetLatestHeightRequest, opts ...grpc.CallOption) (*QueryGetLatestHeightResponse, error) {
	out := new(QueryGetLatestHeightResponse)
	err := c.cc.Invoke(ctx, "/dymensionxyz.dymension.rollapp.Query/LatestHeight", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (m *QueryGetLatestHeightRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *QueryGetLatestHeightRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *QueryGetLatestHeightRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Finalized {
		i--
		if m.Finalized {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x10
	}
	if len(m.RollappId) > 0 {
		i -= len(m.RollappId)
		copy(dAtA[i:], m.RollappId)
		i = encodeVarintQuery(dAtA, i, uint64(len(m.RollappId)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *QueryGetLatestHeightResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *QueryGetLatestHeightResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *QueryGetLatestHeightResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Height != 0 {
		i = encodeVarintQuery(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintQuery(dAtA []byte, offset int, v uint64) int {
	offset -= sovQuery(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}

func (m *QueryGetLatestHeightRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.RollappId)
	if l > 0 {
		n += 1 + l + sovQuery(uint64(l))
	}
	if m.Finalized {
		n += 2
	}
	return n
}

func (m *QueryGetLatestHeightResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Height != 0 {
		n += 1 + sovQuery(uint64(m.Height))
	}
	return n
}

func sovQuery(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozQuery(x uint64) (n int) {
	return sovQuery(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}

func (m *QueryGetLatestHeightRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowQuery
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
			return fmt.Errorf("proto: QueryGetLatestHeightRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: QueryGetLatestHeightRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RollappId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
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
				return ErrInvalidLengthQuery
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthQuery
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.RollappId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Finalized", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
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
			m.Finalized = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipQuery(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthQuery
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
func (m *QueryGetLatestHeightResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowQuery
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
			return fmt.Errorf("proto: QueryGetLatestHeightResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: QueryGetLatestHeightResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Height |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipQuery(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthQuery
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

func skipQuery(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowQuery
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
					return 0, ErrIntOverflowQuery
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
					return 0, ErrIntOverflowQuery
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
				return 0, ErrInvalidLengthQuery
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupQuery
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthQuery
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthQuery        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowQuery          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupQuery = fmt.Errorf("proto: unexpected end of group")
)
