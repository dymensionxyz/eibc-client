// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: dymensionxyz/dymension/eibc/demand_order.proto

package types

import (
	fmt "fmt"
	io "io"
	math "math"
	math_bits "math/bits"

	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
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

type DemandOrder struct {
	// id is a hash of the form generated by GetRollappPacketKey,
	// e.g status/rollappid/packetProofHeight/packetDestinationChannel-PacketSequence which guarantees uniqueness
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// tracking_packet_key is the key of the packet that is being tracked.
	// This key can change depends on the packet status.
	TrackingPacketKey string `protobuf:"bytes,2,opt,name=tracking_packet_key,json=trackingPacketKey,proto3" json:"tracking_packet_key,omitempty"`
	// price is the amount that the fulfiller sends to original eibc transfer recipient
	Price github_com_cosmos_cosmos_sdk_types.Coins `protobuf:"bytes,3,rep,name=price,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"price"`
	// fee is the effective profit made by the fulfiller because they pay price and receive fee + price
	Fee       github_com_cosmos_cosmos_sdk_types.Coins `protobuf:"bytes,4,rep,name=fee,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"fee"`
	Recipient string                                   `protobuf:"bytes,5,opt,name=recipient,proto3" json:"recipient,omitempty"`
	// Deprecated: use DemandOrder.IsFulfilled method instead.
	// Only used for backwards compatibility.
	DeprecatedIsFulfilled bool               `protobuf:"varint,6,opt,name=deprecated_is_fulfilled,json=deprecatedIsFulfilled,proto3" json:"deprecated_is_fulfilled,omitempty"` // Deprecated: Do not use.
	TrackingPacketStatus  Status             `protobuf:"varint,8,opt,name=tracking_packet_status,json=trackingPacketStatus,proto3,enum=dymensionxyz.dymension.common.Status" json:"tracking_packet_status,omitempty"`
	RollappId             string             `protobuf:"bytes,9,opt,name=rollapp_id,json=rollappId,proto3" json:"rollapp_id,omitempty"`
	Type                  RollappPacket_Type `protobuf:"varint,10,opt,name=type,proto3,enum=dymensionxyz.dymension.common.RollappPacket_Type" json:"type,omitempty"`
	// fulfiller_address is the bech32-encoded address of the account which fulfilled the order.
	FulfillerAddress string `protobuf:"bytes,11,opt,name=fulfiller_address,json=fulfillerAddress,proto3" json:"fulfiller_address,omitempty"`
	// creation_height is the height of the block on the hub when order was created.
	CreationHeight uint64 `protobuf:"varint,12,opt,name=creation_height,json=creationHeight,proto3" json:"creation_height,omitempty"`
}

func (m *DemandOrder) Reset()         { *m = DemandOrder{} }
func (m *DemandOrder) String() string { return proto.CompactTextString(m) }
func (*DemandOrder) ProtoMessage()    {}
func (*DemandOrder) Descriptor() ([]byte, []int) {
	return fileDescriptor_2fc99140861fbacd, []int{0}
}
func (m *DemandOrder) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *DemandOrder) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_DemandOrder.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *DemandOrder) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DemandOrder.Merge(m, src)
}
func (m *DemandOrder) XXX_Size() int {
	return m.Size()
}
func (m *DemandOrder) XXX_DiscardUnknown() {
	xxx_messageInfo_DemandOrder.DiscardUnknown(m)
}

var xxx_messageInfo_DemandOrder proto.InternalMessageInfo

func (m *DemandOrder) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *DemandOrder) GetTrackingPacketKey() string {
	if m != nil {
		return m.TrackingPacketKey
	}
	return ""
}

func (m *DemandOrder) GetPrice() github_com_cosmos_cosmos_sdk_types.Coins {
	if m != nil {
		return m.Price
	}
	return nil
}

func (m *DemandOrder) GetFee() github_com_cosmos_cosmos_sdk_types.Coins {
	if m != nil {
		return m.Fee
	}
	return nil
}

func (m *DemandOrder) GetRecipient() string {
	if m != nil {
		return m.Recipient
	}
	return ""
}

// Deprecated: Do not use.
func (m *DemandOrder) GetDeprecatedIsFulfilled() bool {
	if m != nil {
		return m.DeprecatedIsFulfilled
	}
	return false
}

func (m *DemandOrder) GetTrackingPacketStatus() Status {
	if m != nil {
		return m.TrackingPacketStatus
	}
	return Status_PENDING
}

func (m *DemandOrder) GetRollappId() string {
	if m != nil {
		return m.RollappId
	}
	return ""
}

func (m *DemandOrder) GetType() RollappPacket_Type {
	if m != nil {
		return m.Type
	}
	return RollappPacket_ON_RECV
}

func (m *DemandOrder) GetFulfillerAddress() string {
	if m != nil {
		return m.FulfillerAddress
	}
	return ""
}

func (m *DemandOrder) GetCreationHeight() uint64 {
	if m != nil {
		return m.CreationHeight
	}
	return 0
}

func init() {
	proto.RegisterType((*DemandOrder)(nil), "dymensionxyz.dymension.eibc.DemandOrder")
}

func init() {
	proto.RegisterFile("dymensionxyz/dymension/eibc/demand_order.proto", fileDescriptor_2fc99140861fbacd)
}

var fileDescriptor_2fc99140861fbacd = []byte{
	// 512 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x53, 0xcf, 0x6f, 0xd3, 0x30,
	0x14, 0x6e, 0xba, 0x75, 0x5a, 0x5d, 0x54, 0x98, 0x19, 0x60, 0x06, 0x64, 0x15, 0x12, 0x22, 0x02,
	0xe1, 0xd0, 0xee, 0xc6, 0x8d, 0xf2, 0x43, 0x4c, 0x3b, 0x80, 0x02, 0x27, 0x10, 0x8a, 0x52, 0xfb,
	0xb5, 0xb5, 0xda, 0xc4, 0x91, 0xed, 0x4e, 0x0b, 0x47, 0xfe, 0x02, 0xfe, 0x0e, 0xfe, 0x92, 0x1d,
	0x77, 0xe4, 0x04, 0xa8, 0xfd, 0x47, 0x50, 0xec, 0x74, 0x1d, 0x48, 0x85, 0x0b, 0xa7, 0x24, 0xdf,
	0xfb, 0xbe, 0xf7, 0xf9, 0xf3, 0x7b, 0x41, 0x94, 0x17, 0x29, 0x64, 0x5a, 0xc8, 0xec, 0xa4, 0xf8,
	0x14, 0x9e, 0x7f, 0x84, 0x20, 0x06, 0x2c, 0xe4, 0x90, 0x26, 0x19, 0x8f, 0xa5, 0xe2, 0xa0, 0x68,
	0xae, 0xa4, 0x91, 0xf8, 0xd6, 0x45, 0xfe, 0x4a, 0x4c, 0x4b, 0xfe, 0xde, 0xee, 0x48, 0x8e, 0xa4,
	0xe5, 0x85, 0xe5, 0x9b, 0x93, 0xec, 0x3d, 0x58, 0x63, 0xc1, 0x64, 0x9a, 0xca, 0x2c, 0xd4, 0x26,
	0x31, 0x33, 0x5d, 0x71, 0x7b, 0x7f, 0xe7, 0x2a, 0x39, 0x9d, 0x26, 0x79, 0x1e, 0xe7, 0x09, 0x9b,
	0x80, 0xa9, 0x34, 0x3e, 0x93, 0x3a, 0x95, 0x3a, 0x1c, 0x24, 0x1a, 0xc2, 0xe3, 0xee, 0x00, 0x4c,
	0xd2, 0x0d, 0x99, 0x14, 0x99, 0xab, 0xdf, 0xfd, 0xdc, 0x40, 0xad, 0xe7, 0x36, 0xc9, 0xeb, 0x32,
	0x08, 0x6e, 0xa3, 0xba, 0xe0, 0xc4, 0xeb, 0x78, 0x41, 0x33, 0xaa, 0x0b, 0x8e, 0x29, 0xba, 0x6a,
	0x54, 0xc2, 0x26, 0x22, 0x1b, 0x55, 0x8d, 0xe3, 0x09, 0x14, 0xa4, 0x6e, 0x09, 0x3b, 0xcb, 0xd2,
	0x1b, 0x5b, 0x39, 0x82, 0x02, 0x27, 0xa8, 0x91, 0x2b, 0xc1, 0x80, 0x6c, 0x74, 0x36, 0x82, 0x56,
	0xef, 0x26, 0x75, 0xfe, 0xb4, 0xf4, 0xa7, 0x95, 0x3f, 0x7d, 0x26, 0x45, 0xd6, 0x7f, 0x7c, 0xfa,
	0x7d, 0xbf, 0xf6, 0xf5, 0xc7, 0x7e, 0x30, 0x12, 0x66, 0x3c, 0x1b, 0x50, 0x26, 0xd3, 0xb0, 0x3a,
	0xac, 0x7b, 0x3c, 0xd2, 0x7c, 0x12, 0x9a, 0x22, 0x07, 0x6d, 0x05, 0x3a, 0x72, 0x9d, 0xf1, 0x47,
	0xb4, 0x31, 0x04, 0x20, 0x9b, 0xff, 0xdf, 0xa0, 0xec, 0x8b, 0x6f, 0xa3, 0xa6, 0x02, 0x26, 0x72,
	0x01, 0x99, 0x21, 0x0d, 0x9b, 0x73, 0x05, 0xe0, 0x27, 0xe8, 0x06, 0x87, 0x5c, 0x01, 0x4b, 0x0c,
	0xf0, 0x58, 0xe8, 0x78, 0x38, 0x9b, 0x0e, 0xc5, 0x74, 0x0a, 0x9c, 0x6c, 0x75, 0xbc, 0x60, 0xbb,
	0x5f, 0x27, 0x5e, 0x74, 0x6d, 0x45, 0x39, 0xd4, 0x2f, 0x97, 0x04, 0xfc, 0x01, 0x5d, 0xff, 0xf3,
	0x2e, 0xdd, 0x7c, 0xc9, 0x76, 0xc7, 0x0b, 0xda, 0xbd, 0x7b, 0x74, 0xcd, 0xfe, 0xb8, 0x01, 0xd3,
	0xb7, 0x96, 0x1c, 0xed, 0xfe, 0x7e, 0xeb, 0x0e, 0xc5, 0x77, 0x10, 0x5a, 0x2e, 0x80, 0xe0, 0xa4,
	0x59, 0x9d, 0xdb, 0x21, 0x87, 0x1c, 0xbf, 0x40, 0x9b, 0x65, 0x52, 0x82, 0xac, 0x53, 0xf7, 0x1f,
	0x4e, 0x91, 0xd3, 0x39, 0x03, 0xfa, 0xae, 0xc8, 0x21, 0xb2, 0x72, 0xfc, 0x10, 0xed, 0x2c, 0x03,
	0xab, 0x38, 0xe1, 0x5c, 0x81, 0xd6, 0xa4, 0x65, 0xcd, 0xae, 0x9c, 0x17, 0x9e, 0x3a, 0x1c, 0xdf,
	0x47, 0x97, 0x99, 0x82, 0xc4, 0x08, 0x99, 0xc5, 0x63, 0x10, 0xa3, 0xb1, 0x21, 0x97, 0x3a, 0x5e,
	0xb0, 0x19, 0xb5, 0x97, 0xf0, 0x2b, 0x8b, 0xf6, 0x8f, 0x4e, 0xe7, 0xbe, 0x77, 0x36, 0xf7, 0xbd,
	0x9f, 0x73, 0xdf, 0xfb, 0xb2, 0xf0, 0x6b, 0x67, 0x0b, 0xbf, 0xf6, 0x6d, 0xe1, 0xd7, 0xde, 0x77,
	0x2f, 0xcc, 0x6e, 0xcd, 0xf6, 0x1f, 0x1f, 0x84, 0x27, 0xee, 0x8f, 0xb4, 0xa3, 0x1c, 0x6c, 0xd9,
	0xc5, 0x3e, 0xf8, 0x15, 0x00, 0x00, 0xff, 0xff, 0x6c, 0x42, 0xcd, 0xcc, 0xbd, 0x03, 0x00, 0x00,
}

func (m *DemandOrder) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *DemandOrder) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *DemandOrder) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.CreationHeight != 0 {
		i = encodeVarintDemandOrder(dAtA, i, uint64(m.CreationHeight))
		i--
		dAtA[i] = 0x60
	}
	if len(m.FulfillerAddress) > 0 {
		i -= len(m.FulfillerAddress)
		copy(dAtA[i:], m.FulfillerAddress)
		i = encodeVarintDemandOrder(dAtA, i, uint64(len(m.FulfillerAddress)))
		i--
		dAtA[i] = 0x5a
	}
	if m.Type != 0 {
		i = encodeVarintDemandOrder(dAtA, i, uint64(m.Type))
		i--
		dAtA[i] = 0x50
	}
	if len(m.RollappId) > 0 {
		i -= len(m.RollappId)
		copy(dAtA[i:], m.RollappId)
		i = encodeVarintDemandOrder(dAtA, i, uint64(len(m.RollappId)))
		i--
		dAtA[i] = 0x4a
	}
	if m.TrackingPacketStatus != 0 {
		i = encodeVarintDemandOrder(dAtA, i, uint64(m.TrackingPacketStatus))
		i--
		dAtA[i] = 0x40
	}
	if m.DeprecatedIsFulfilled {
		i--
		if m.DeprecatedIsFulfilled {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x30
	}
	if len(m.Recipient) > 0 {
		i -= len(m.Recipient)
		copy(dAtA[i:], m.Recipient)
		i = encodeVarintDemandOrder(dAtA, i, uint64(len(m.Recipient)))
		i--
		dAtA[i] = 0x2a
	}
	if len(m.Fee) > 0 {
		for iNdEx := len(m.Fee) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Fee[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintDemandOrder(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x22
		}
	}
	if len(m.Price) > 0 {
		for iNdEx := len(m.Price) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Price[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintDemandOrder(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x1a
		}
	}
	if len(m.TrackingPacketKey) > 0 {
		i -= len(m.TrackingPacketKey)
		copy(dAtA[i:], m.TrackingPacketKey)
		i = encodeVarintDemandOrder(dAtA, i, uint64(len(m.TrackingPacketKey)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Id) > 0 {
		i -= len(m.Id)
		copy(dAtA[i:], m.Id)
		i = encodeVarintDemandOrder(dAtA, i, uint64(len(m.Id)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintDemandOrder(dAtA []byte, offset int, v uint64) int {
	offset -= sovDemandOrder(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *DemandOrder) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Id)
	if l > 0 {
		n += 1 + l + sovDemandOrder(uint64(l))
	}
	l = len(m.TrackingPacketKey)
	if l > 0 {
		n += 1 + l + sovDemandOrder(uint64(l))
	}
	if len(m.Price) > 0 {
		for _, e := range m.Price {
			l = e.Size()
			n += 1 + l + sovDemandOrder(uint64(l))
		}
	}
	if len(m.Fee) > 0 {
		for _, e := range m.Fee {
			l = e.Size()
			n += 1 + l + sovDemandOrder(uint64(l))
		}
	}
	l = len(m.Recipient)
	if l > 0 {
		n += 1 + l + sovDemandOrder(uint64(l))
	}
	if m.DeprecatedIsFulfilled {
		n += 2
	}
	if m.TrackingPacketStatus != 0 {
		n += 1 + sovDemandOrder(uint64(m.TrackingPacketStatus))
	}
	l = len(m.RollappId)
	if l > 0 {
		n += 1 + l + sovDemandOrder(uint64(l))
	}
	if m.Type != 0 {
		n += 1 + sovDemandOrder(uint64(m.Type))
	}
	l = len(m.FulfillerAddress)
	if l > 0 {
		n += 1 + l + sovDemandOrder(uint64(l))
	}
	if m.CreationHeight != 0 {
		n += 1 + sovDemandOrder(uint64(m.CreationHeight))
	}
	return n
}

func sovDemandOrder(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozDemandOrder(x uint64) (n int) {
	return sovDemandOrder(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *DemandOrder) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDemandOrder
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
			return fmt.Errorf("proto: DemandOrder: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: DemandOrder: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Id", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Id = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TrackingPacketKey", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TrackingPacketKey = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Price", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Price = append(m.Price, types.Coin{})
			if err := m.Price[len(m.Price)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Fee", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Fee = append(m.Fee, types.Coin{})
			if err := m.Fee[len(m.Fee)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Recipient", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Recipient = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field DeprecatedIsFulfilled", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
			m.DeprecatedIsFulfilled = bool(v != 0)
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TrackingPacketStatus", wireType)
			}
			m.TrackingPacketStatus = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TrackingPacketStatus |= Status(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RollappId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.RollappId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 10:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= RollappPacket_Type(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field FulfillerAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
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
				return ErrInvalidLengthDemandOrder
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthDemandOrder
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.FulfillerAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 12:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field CreationHeight", wireType)
			}
			m.CreationHeight = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDemandOrder
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.CreationHeight |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipDemandOrder(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDemandOrder
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
func skipDemandOrder(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowDemandOrder
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
					return 0, ErrIntOverflowDemandOrder
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
					return 0, ErrIntOverflowDemandOrder
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
				return 0, ErrInvalidLengthDemandOrder
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupDemandOrder
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthDemandOrder
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthDemandOrder        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowDemandOrder          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupDemandOrder = fmt.Errorf("proto: unexpected end of group")
)
