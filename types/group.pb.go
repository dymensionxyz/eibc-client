package types

import (
	"context"
	"fmt"
	"io"
	"math"

	sdkerrors "cosmossdk.io/errors"
	_ "github.com/cosmos/cosmos-proto"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/authz/codec"
	"github.com/cosmos/cosmos-sdk/x/group/errors"
	_ "github.com/cosmos/gogoproto/gogoproto"
	"github.com/cosmos/gogoproto/proto"
	"google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = proto.Marshal
	_ = fmt.Errorf
	_ = math.Inf
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// Exec defines modes of execution of a proposal on creation or on new vote.
type Exec int32

const (
	// An empty value means that there should be a separate
	// MsgExec request for the proposal to execute.
	Exec_EXEC_UNSPECIFIED Exec = 0
	// Try to execute the proposal immediately.
	// If the proposal is not allowed per the DecisionPolicy,
	// the proposal will still be open and could
	// be executed at a later point.
	Exec_EXEC_TRY Exec = 1
)

var Exec_name = map[int32]string{
	0: "EXEC_UNSPECIFIED",
	1: "EXEC_TRY",
}

var Exec_value = map[string]int32{
	"EXEC_UNSPECIFIED": 0,
	"EXEC_TRY":         1,
}

func (x Exec) String() string {
	return proto.EnumName(Exec_name, int32(x))
}

func (Exec) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_6b8d3d629f136420, []int{0}
}

var _ sdk.Msg = &MsgSubmitProposal{}

// NewMsgSubmitProposal creates a new MsgSubmitProposal.
func NewMsgSubmitProposal(address string, proposers []string, msgs []sdk.Msg, metadata string, exec Exec, title, summary string) (*MsgSubmitProposal, error) {
	m := &MsgSubmitProposal{
		GroupPolicyAddress: address,
		Proposers:          proposers,
		Metadata:           metadata,
		Exec:               exec,
		Title:              title,
		Summary:            summary,
	}
	err := m.SetMsgs(msgs)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// MsgSubmitProposal is the Msg/SubmitProposal request type.
type MsgSubmitProposal struct {
	// group_policy_address is the account address of group policy.
	GroupPolicyAddress string `protobuf:"bytes,1,opt,name=group_policy_address,json=groupPolicyAddress,proto3" json:"group_policy_address,omitempty"`
	// proposers are the account addresses of the proposers.
	// Proposers signatures will be counted as yes votes.
	Proposers []string `protobuf:"bytes,2,rep,name=proposers,proto3" json:"proposers,omitempty"`
	// metadata is any arbitrary metadata attached to the proposal.
	Metadata string `protobuf:"bytes,3,opt,name=metadata,proto3" json:"metadata,omitempty"`
	// messages is a list of `sdk.Msg`s that will be executed if the proposal passes.
	Messages []*types.Any `protobuf:"bytes,4,rep,name=messages,proto3" json:"messages,omitempty"`
	// exec defines the mode of execution of the proposal,
	// whether it should be executed immediately on creation or not.
	// If so, proposers signatures are considered as Yes votes.
	Exec Exec `protobuf:"varint,5,opt,name=exec,proto3,enum=cosmos.group.v1.Exec" json:"exec,omitempty"`
	// title is the title of the proposal.
	//
	// Since: cosmos-sdk 0.47
	Title string `protobuf:"bytes,6,opt,name=title,proto3" json:"title,omitempty"`
	// summary is the summary of the proposal.
	//
	// Since: cosmos-sdk 0.47
	Summary string `protobuf:"bytes,7,opt,name=summary,proto3" json:"summary,omitempty"`
}

func (m *MsgSubmitProposal) Reset()         { *m = MsgSubmitProposal{} }
func (m *MsgSubmitProposal) String() string { return proto.CompactTextString(m) }
func (*MsgSubmitProposal) ProtoMessage()    {}
func (*MsgSubmitProposal) Descriptor() ([]byte, []int) {
	return fileDescriptor_6b8d3d629f136420, []int{18}
}

func (m *MsgSubmitProposal) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}

func (m *MsgSubmitProposal) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgSubmitProposal.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}

func (m *MsgSubmitProposal) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgSubmitProposal.Merge(m, src)
}

func (m *MsgSubmitProposal) XXX_Size() int {
	return m.Size()
}

func (m *MsgSubmitProposal) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgSubmitProposal.DiscardUnknown(m)
}

var xxx_messageInfo_MsgSubmitProposal proto.InternalMessageInfo

// MsgSubmitProposalResponse is the Msg/SubmitProposal response type.
type MsgSubmitProposalResponse struct {
	// proposal is the unique ID of the proposal.
	ProposalId uint64 `protobuf:"varint,1,opt,name=proposal_id,json=proposalId,proto3" json:"proposal_id,omitempty"`
}

func (m *MsgSubmitProposalResponse) Reset()         { *m = MsgSubmitProposalResponse{} }
func (m *MsgSubmitProposalResponse) String() string { return proto.CompactTextString(m) }
func (*MsgSubmitProposalResponse) ProtoMessage()    {}
func (*MsgSubmitProposalResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_6b8d3d629f136420, []int{19}
}

func (m *MsgSubmitProposalResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}

func (m *MsgSubmitProposalResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgSubmitProposalResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}

func (m *MsgSubmitProposalResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgSubmitProposalResponse.Merge(m, src)
}

func (m *MsgSubmitProposalResponse) XXX_Size() int {
	return m.Size()
}

func (m *MsgSubmitProposalResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgSubmitProposalResponse.DiscardUnknown(m)
}

var xxx_messageInfo_MsgSubmitProposalResponse proto.InternalMessageInfo

func (m *MsgSubmitProposalResponse) GetProposalId() uint64 {
	if m != nil {
		return m.ProposalId
	}
	return 0
}

// Route Implements Msg.
func (m MsgSubmitProposal) Route() string {
	return sdk.MsgTypeURL(&m)
}

// Type Implements Msg.
func (m MsgSubmitProposal) Type() string { return sdk.MsgTypeURL(&m) }

// GetSignBytes Implements Msg.
func (m MsgSubmitProposal) GetSignBytes() []byte {
	return sdk.MustSortJSON(codec.ModuleCdc.MustMarshalJSON(&m))
}

func (MsgSubmitProposal) XXX_MessageName() string {
	return "cosmos.group.v1.MsgSubmitProposal"
}

// GetSigners returns the expected signers for a MsgSubmitProposal.
func (m MsgSubmitProposal) GetSigners() []sdk.AccAddress {
	addrs, err := m.getProposerAccAddresses()
	if err != nil {
		panic(err)
	}

	return addrs
}

// ValidateBasic does a sanity check on the provided proposal, such as
// verifying proposer addresses, and performing ValidateBasic on each
// individual `sdk.Msg`.
func (m MsgSubmitProposal) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.GroupPolicyAddress)
	if err != nil {
		return sdkerrors.Wrap(err, "group policy")
	}

	if m.Title == "" {
		return sdkerrors.Wrap(errors.ErrEmpty, "title")
	}

	if m.Summary == "" {
		return sdkerrors.Wrap(errors.ErrEmpty, "summary")
	}

	if len(m.Proposers) == 0 {
		return sdkerrors.Wrap(errors.ErrEmpty, "proposers")
	}

	addrs, err := m.getProposerAccAddresses()
	if err != nil {
		return sdkerrors.Wrap(err, "group proposers")
	}

	if err := accAddresses(addrs).ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "proposers")
	}

	msgs, err := m.GetMsgs()
	if err != nil {
		return err
	}

	for i, msg := range msgs {
		if err := msg.ValidateBasic(); err != nil {
			return sdkerrors.Wrapf(err, "msg %d", i)
		}
	}
	return nil
}

type accAddresses []sdk.AccAddress

// ValidateBasic verifies that there's no duplicate address.
// Individual account address validation has to be done separately.
func (a accAddresses) ValidateBasic() error {
	index := make(map[string]struct{}, len(a))
	for i := range a {
		accAddr := a[i]
		addr := string(accAddr)
		if _, exists := index[addr]; exists {
			return sdkerrors.Wrapf(errors.ErrDuplicate, "address: %s", accAddr.String())
		}
		index[addr] = struct{}{}
	}
	return nil
}

// getProposerAccAddresses returns the proposers as `[]sdk.AccAddress`.
func (m *MsgSubmitProposal) getProposerAccAddresses() ([]sdk.AccAddress, error) {
	addrs := make([]sdk.AccAddress, len(m.Proposers))
	for i, proposer := range m.Proposers {
		addr, err := sdk.AccAddressFromBech32(proposer)
		if err != nil {
			return nil, sdkerrors.Wrap(err, "proposers")
		}
		addrs[i] = addr
	}

	return addrs, nil
}

// SetMsgs packs msgs into Any's
func (m *MsgSubmitProposal) SetMsgs(msgs []sdk.Msg) error {
	anys, err := tx.SetMsgs(msgs)
	if err != nil {
		return err
	}
	m.Messages = anys
	return nil
}

// GetMsgs unpacks m.Messages Any's into sdk.Msg's
func (m MsgSubmitProposal) GetMsgs() ([]sdk.Msg, error) {
	return tx.GetMsgs(m.Messages, "proposal")
}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (m MsgSubmitProposal) UnpackInterfaces(unpacker types.AnyUnpacker) error {
	return tx.UnpackInterfaces(unpacker, m.Messages)
}

func init() {
	proto.RegisterEnum("cosmos.group.v1.Exec", Exec_name, Exec_value)
	proto.RegisterType((*MsgSubmitProposal)(nil), "cosmos.group.v1.MsgSubmitProposal")
	proto.RegisterType((*MsgSubmitProposalResponse)(nil), "cosmos.group.v1.MsgSubmitProposalResponse")
}

func init() { proto.RegisterFile("cosmos/group/v1/tx.proto", fileDescriptor_6b8d3d629f136420) }

var fileDescriptor_6b8d3d629f136420 = []byte{
	// 1443 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xc4, 0x58, 0xcf, 0x6f, 0x1b, 0xc5,
	0x17, 0xcf, 0xda, 0xce, 0xaf, 0x97, 0x6f, 0xdd, 0x64, 0x9b, 0xb4, 0x9b, 0x6d, 0x6b, 0xbb, 0xd3,
	0xb4, 0x49, 0xad, 0xc6, 0x6e, 0x9c, 0x6f, 0x2b, 0x61, 0x10, 0xa8, 0x49, 0x0d, 0x0a, 0xc2, 0x10,
	0x6d, 0x5b, 0x0a, 0x5c, 0xcc, 0x26, 0xde, 0x6e, 0x57, 0x64, 0xbd, 0xc6, 0xb3, 0x4e, 0x93, 0x1b,
	0x3f, 0x2e, 0xd0, 0x0b, 0x48, 0xf0, 0x07, 0xc0, 0x8d, 0x63, 0x91, 0x7a, 0xe0, 0xc6, 0x0d, 0x55,
	0xe5, 0x52, 0x71, 0xe2, 0x84, 0xa0, 0x15, 0xea, 0x8d, 0x7f, 0x01, 0xb4, 0x33, 0xbb, 0xe3, 0x1d,
	0xef, 0xae, 0x77, 0x6b, 0x59, 0x70, 0x89, 0xb2, 0xf3, 0x3e, 0xef, 0xd7, 0xe7, 0xbd, 0x79, 0x33,
	0x63, 0x90, 0x76, 0x2d, 0x6c, 0x5a, 0xb8, 0xac, 0x77, 0xac, 0x6e, 0xbb, 0xbc, 0xbf, 0x56, 0xb6,
	0x0f, 0x4a, 0xed, 0x8e, 0x65, 0x5b, 0xe2, 0x51, 0x2a, 0x29, 0x11, 0x49, 0x69, 0x7f, 0x4d, 0x9e,
	0xd7, 0x2d, 0xdd, 0x22, 0xb2, 0xb2, 0xf3, 0x1f, 0x85, 0xc9, 0x8b, 0x14, 0xd6, 0xa0, 0x02, 0x57,
	0xc7, 0x15, 0xe9, 0x96, 0xa5, 0xef, 0x69, 0x65, 0xf2, 0xb5, 0xd3, 0xbd, 0x5d, 0x56, 0x5b, 0x87,
	0xae, 0xe8, 0x64, 0xc0, 0xed, 0x61, 0x5b, 0xf3, 0xf4, 0x4e, 0xb8, 0x42, 0x13, 0xeb, 0x8e, 0xc8,
	0xc4, 0xba, 0x2b, 0x98, 0x53, 0x4d, 0xa3, 0x65, 0x95, 0xc9, 0x5f, 0xba, 0x84, 0x7e, 0x16, 0x20,
	0x5b, 0xc7, 0xfa, 0x66, 0x47, 0x53, 0x6d, 0xed, 0x35, 0xc7, 0x9a, 0x58, 0x82, 0x71, 0xb5, 0x69,
	0x1a, 0x2d, 0x49, 0x28, 0x08, 0x2b, 0xd3, 0x1b, 0xd2, 0x2f, 0x0f, 0x56, 0xe7, 0xdd, 0xb8, 0xae,
	0x36, 0x9b, 0x1d, 0x0d, 0xe3, 0xeb, 0x76, 0xc7, 0x68, 0xe9, 0x0a, 0x85, 0x89, 0x9b, 0x30, 0x69,
	0x6a, 0xe6, 0x8e, 0xd6, 0xc1, 0x52, 0xaa, 0x90, 0x5e, 0x99, 0xa9, 0xe4, 0x4a, 0x7d, 0xa9, 0x97,
	0xea, 0x44, 0xae, 0x68, 0x1f, 0x76, 0x35, 0x6c, 0x6f, 0x4c, 0x3f, 0xfc, 0x2d, 0x3f, 0xf6, 0xdd,
	0xb3, 0xfb, 0x45, 0x41, 0xf1, 0x34, 0x45, 0x19, 0xa6, 0x4c, 0xcd, 0x56, 0x9b, 0xaa, 0xad, 0x4a,
	0x69, 0xc7, 0xaf, 0xc2, 0xbe, 0xab, 0x2b, 0x9f, 0x3c, 0xbb, 0x5f, 0xa4, 0xce, 0xee, 0x3d, 0xbb,
	0x5f, 0x74, 0x19, 0x5b, 0xc5, 0xcd, 0x0f, 0xca, 0x7c, 0xe8, 0x68, 0x1d, 0x8e, 0xf3, 0x2b, 0x8a,
	0x86, 0xdb, 0x56, 0x0b, 0x6b, 0xe2, 0x22, 0x4c, 0x91, 0x68, 0x1a, 0x46, 0x93, 0xe4, 0x95, 0x51,
	0x26, 0xc9, 0xf7, 0x56, 0x13, 0xfd, 0x29, 0xc0, 0x42, 0x1d, 0xeb, 0x37, 0xdb, 0x4d, 0x4f, 0xab,
	0xee, 0x06, 0xf5, 0xbc, 0x4c, 0xf8, 0x9d, 0xa4, 0x38, 0x27, 0xe2, 0x36, 0x64, 0x69, 0xaa, 0x8d,
	0x2e, 0xf1, 0x83, 0xa5, 0xf4, 0xf3, 0x72, 0x75, 0x84, 0x1a, 0xa0, 0x71, 0xe2, 0x6a, 0x99, 0x67,
	0xa5, 0xc0, 0xb3, 0x12, 0xcc, 0x06, 0xe5, 0xe1, 0x74, 0xa8, 0xc0, 0xe3, 0x08, 0xfd, 0x24, 0xc0,
	0x31, 0x1e, 0x71, 0x95, 0xa4, 0x35, 0x42, 0x1a, 0x2e, 0xc3, 0x74, 0x4b, 0xbb, 0xdb, 0xa0, 0xe6,
	0xd2, 0x31, 0xe6, 0xa6, 0x5a, 0xda, 0x5d, 0x12, 0x41, 0x75, 0x95, 0xcf, 0x35, 0x17, 0x99, 0x2b,
	0x81, 0xa3, 0xd3, 0x70, 0x32, 0x64, 0x99, 0xe5, 0xf9, 0xbd, 0x40, 0xda, 0x84, 0x63, 0x82, 0xb6,
	0xda, 0x28, 0x53, 0x1d, 0xd4, 0xd1, 0x97, 0xf8, 0x7c, 0xce, 0x0c, 0xa8, 0x1d, 0xd5, 0x40, 0x05,
	0xc8, 0x85, 0x4b, 0x58, 0x56, 0x5f, 0xa7, 0x60, 0x9e, 0x6f, 0xfe, 0x6d, 0x6b, 0xcf, 0xd8, 0x3d,
	0xfc, 0x97, 0x72, 0x12, 0x55, 0x38, 0xda, 0xd4, 0x76, 0x0d, 0x6c, 0x58, 0xad, 0x46, 0x9b, 0x78,
	0x96, 0x32, 0x05, 0x61, 0x65, 0xa6, 0x32, 0x5f, 0xa2, 0x73, 0xac, 0xe4, 0xcd, 0xb1, 0xd2, 0xd5,
	0xd6, 0xe1, 0x06, 0x7a, 0xf4, 0x60, 0x35, 0xd7, 0xdf, 0xfb, 0xd7, 0x5c, 0x03, 0x34, 0x72, 0x25,
	0xdb, 0xe4, 0xbe, 0xab, 0x95, 0xcf, 0xbe, 0xc9, 0x8f, 0xf1, 0xd4, 0xe5, 0x23, 0x87, 0x01, 0xd5,
	0x41, 0x0a, 0x9c, 0x0a, 0x5b, 0x67, 0x83, 0xa1, 0x02, 0x93, 0x2a, 0x65, 0x21, 0x96, 0x1f, 0x0f,
	0x88, 0x3e, 0x4d, 0xc1, 0x22, 0x5f, 0x0d, 0x6a, 0x74, 0xb8, 0xed, 0xf2, 0x3a, 0xcc, 0x53, 0xbe,
	0x29, 0x6b, 0x0d, 0x2f, 0x9c, 0x54, 0x8c, 0xba, 0xa8, 0xfb, 0x3d, 0x13, 0xc9, 0xb0, 0xfb, 0x6b,
	0x9d, 0x27, 0x75, 0x29, 0xb2, 0x1f, 0x7d, 0x79, 0xa2, 0xb3, 0x70, 0x26, 0x52, 0xc8, 0xba, 0xf2,
	0x87, 0x34, 0x48, 0x3c, 0xff, 0xb7, 0x0c, 0xfb, 0xce, 0x90, 0x9d, 0x39, 0x92, 0x93, 0xe6, 0x1c,
	0x64, 0x29, 0xdd, 0x7d, 0x9d, 0x7c, 0x44, 0xe7, 0x26, 0x41, 0x05, 0x16, 0xb8, 0xaa, 0x30, 0x74,
	0x86, 0xa0, 0x8f, 0xf9, 0xc8, 0x67, 0x3a, 0x6b, 0x7d, 0x3a, 0x2a, 0x76, 0x2b, 0x31, 0x5e, 0x10,
	0x56, 0xa6, 0xf8, 0x82, 0x61, 0xda, 0x2c, 0x21, 0xbb, 0x66, 0x62, 0xc4, 0xbb, 0xe6, 0x4a, 0x70,
	0xd7, 0x9c, 0x8d, 0xdc, 0x35, 0xbd, 0xea, 0xa0, 0xcf, 0x05, 0x28, 0x44, 0x09, 0x13, 0x9c, 0xab,
	0xa3, 0xec, 0x6b, 0xf4, 0x63, 0x0a, 0x50, 0x58, 0xb3, 0xf1, 0xa9, 0xff, 0xa7, 0x5b, 0x2f, 0xa4,
	0x92, 0xe9, 0x11, 0x57, 0xb2, 0x1a, 0xac, 0xe4, 0x72, 0xe4, 0x56, 0xe5, 0x6d, 0xa1, 0x8b, 0x50,
	0x8c, 0x27, 0x90, 0x6d, 0xdb, 0xbf, 0x04, 0x32, 0x36, 0x03, 0xf0, 0xa1, 0x0f, 0xca, 0x51, 0x32,
	0x3d, 0xe8, 0x64, 0xbd, 0x92, 0x94, 0x1e, 0x3e, 0x1f, 0x74, 0x1e, 0x96, 0x06, 0xc9, 0x19, 0x31,
	0x7f, 0xa4, 0x60, 0xae, 0x8e, 0xf5, 0xeb, 0xdd, 0x1d, 0xd3, 0xb0, 0xb7, 0x3b, 0x56, 0xdb, 0xc2,
	0xea, 0x5e, 0x64, 0x76, 0xc2, 0x10, 0xd9, 0x9d, 0x82, 0xe9, 0x36, 0xb1, 0xeb, 0x8d, 0xb9, 0x69,
	0xa5, 0xb7, 0x30, 0xf0, 0x04, 0xbe, 0xe4, 0xc8, 0x30, 0x56, 0x75, 0x0d, 0x4b, 0x19, 0x32, 0x1f,
	0x43, 0x5b, 0x4f, 0x61, 0x28, 0xf1, 0x02, 0x64, 0xb4, 0x03, 0x6d, 0x97, 0xcc, 0xa7, 0x6c, 0x65,
	0x21, 0x30, 0x4d, 0x6b, 0x07, 0xda, 0xae, 0x42, 0x20, 0xe2, 0x3c, 0x8c, 0xdb, 0x86, 0xbd, 0xa7,
	0x91, 0xf1, 0x34, 0xad, 0xd0, 0x0f, 0x51, 0x82, 0x49, 0xdc, 0x35, 0x4d, 0xb5, 0x73, 0x28, 0x4d,
	0x92, 0x75, 0xef, 0xb3, 0xfa, 0x82, 0xd7, 0xab, 0xbd, 0xe0, 0x9d, 0x82, 0x20, 0x5f, 0x41, 0xe8,
	0xe3, 0x25, 0xc0, 0x26, 0x7a, 0x89, 0x9c, 0xae, 0xfc, 0x22, 0x1b, 0x38, 0x79, 0x98, 0x69, 0xbb,
	0x6b, 0xbd, 0x99, 0x03, 0xde, 0xd2, 0x56, 0x13, 0x7d, 0x4b, 0x6f, 0xb1, 0xce, 0xac, 0x6a, 0x76,
	0xd4, 0xbb, 0xac, 0x46, 0x71, 0x8a, 0xfe, 0x9b, 0x40, 0x2a, 0xe1, 0x4d, 0xa0, 0x7a, 0xd9, 0xc9,
	0xd0, 0xfb, 0xea, 0x3f, 0x3a, 0x59, 0x7e, 0xfd, 0xb1, 0xb8, 0x17, 0xd4, 0xfe, 0x65, 0xd6, 0x64,
	0x7f, 0x0b, 0x30, 0x59, 0xc7, 0xfa, 0xdb, 0x96, 0x1d, 0x9f, 0xaf, 0xb3, 0x13, 0xf7, 0x2d, 0x5b,
	0xeb, 0xc4, 0x06, 0x4d, 0x61, 0xe2, 0x3a, 0x4c, 0x58, 0x6d, 0xdb, 0xb0, 0xe8, 0xfd, 0x20, 0x5b,
	0x39, 0x19, 0xa8, 0xba, 0xe3, 0xf7, 0x2d, 0x02, 0x51, 0x5c, 0x28, 0xd7, 0x76, 0x99, 0xbe, 0xb6,
	0x4b, 0xde, 0x44, 0xd5, 0x65, 0xb2, 0x3b, 0x49, 0x1c, 0x0e, 0x59, 0x52, 0x18, 0x59, 0x8e, 0x77,
	0x34, 0x07, 0x47, 0xdd, 0x7f, 0x19, 0x29, 0xf7, 0x28, 0x29, 0x8e, 0xb5, 0x78, 0x52, 0xfe, 0x0f,
	0x53, 0x8e, 0xc3, 0xae, 0x6d, 0xc5, 0xf3, 0xc2, 0x90, 0xf4, 0xa1, 0x39, 0x81, 0x0d, 0xbd, 0x35,
	0x20, 0x3e, 0x27, 0x00, 0xa4, 0x90, 0xf8, 0x48, 0x66, 0x5e, 0x63, 0xbe, 0x02, 0x13, 0x1d, 0x0d,
	0x77, 0xf7, 0x6c, 0xe2, 0x30, 0x5b, 0x59, 0x0e, 0x10, 0xe1, 0xd5, 0xb9, 0xe6, 0xfa, 0x53, 0x08,
	0x5c, 0x71, 0xd5, 0xd0, 0x17, 0x02, 0x1c, 0xa9, 0x63, 0xfd, 0x0d, 0x4d, 0xdd, 0x77, 0x5f, 0xe2,
	0x43, 0xdc, 0x4d, 0x07, 0xdc, 0xde, 0xe9, 0x8b, 0xd1, 0xdf, 0xac, 0xb9, 0xb0, 0xfc, 0x7a, 0xfe,
	0xd1, 0x09, 0xf2, 0x30, 0xee, 0x2d, 0x78, 0xb9, 0x16, 0x8b, 0x90, 0xa9, 0xd1, 0xa1, 0x30, 0x5b,
	0x7b, 0xa7, 0xb6, 0xd9, 0xb8, 0xf9, 0xe6, 0xf5, 0xed, 0xda, 0xe6, 0xd6, 0xab, 0x5b, 0xb5, 0x6b,
	0xb3, 0x63, 0xe2, 0xff, 0x60, 0x8a, 0xac, 0xde, 0x50, 0xde, 0x9d, 0x15, 0x2a, 0x8f, 0x66, 0x20,
	0x5d, 0xc7, 0xba, 0x78, 0x0b, 0x66, 0xfc, 0xbf, 0x32, 0xe4, 0x83, 0x57, 0x37, 0xee, 0xae, 0x21,
	0x2f, 0xc7, 0x00, 0x18, 0xf1, 0x7b, 0x20, 0x86, 0xbc, 0xdd, 0xcf, 0x87, 0xa9, 0x07, 0x71, 0x72,
	0x29, 0x19, 0x8e, 0x79, 0xbb, 0x0d, 0xb3, 0x81, 0x07, 0xf2, 0x52, 0x8c, 0x0d, 0x82, 0x92, 0x2f,
	0x26, 0x41, 0x31, 0x3f, 0x16, 0x1c, 0x0b, 0x7b, 0xa0, 0x2e, 0xc7, 0x86, 0x4b, 0x81, 0x72, 0x39,
	0x21, 0x90, 0x39, 0x34, 0x60, 0x2e, 0xf8, 0x76, 0x3c, 0x17, 0x53, 0x04, 0x0a, 0x93, 0x57, 0x13,
	0xc1, 0x98, 0xab, 0x2e, 0x2c, 0x84, 0x3f, 0x08, 0x2e, 0xc4, 0xd8, 0xe9, 0x41, 0xe5, 0xb5, 0xc4,
	0x50, 0xe6, 0xf6, 0x00, 0x8e, 0x47, 0x3c, 0xd9, 0x8a, 0x31, 0x64, 0xf9, 0xb0, 0x72, 0x25, 0x39,
	0x96, 0x79, 0xfe, 0x4a, 0x80, 0x7c, 0xdc, 0xdd, 0x75, 0x3d, 0x91, 0x5d, 0x5e, 0x49, 0x7e, 0x71,
	0x08, 0x25, 0x16, 0xd5, 0xc7, 0x02, 0x2c, 0x46, 0xdf, 0xf0, 0x56, 0x13, 0x99, 0x66, 0xfd, 0x76,
	0xf9, 0xb9, 0xe0, 0x2c, 0x86, 0xf7, 0x21, 0xdb, 0x77, 0x97, 0x42, 0x61, 0x86, 0x78, 0x8c, 0x5c,
	0x8c, 0xc7, 0xf8, 0x37, 0x6c, 0xe0, 0x2e, 0x10, 0xba, 0x61, 0xfb, 0x51, 0xe1, 0x1b, 0x36, 0xea,
	0xd0, 0x16, 0x37, 0x20, 0x43, 0x0e, 0x6c, 0x29, 0x4c, 0xcb, 0x91, 0xc8, 0x85, 0x28, 0x89, 0xdf,
	0x06, 0x99, 0xab, 0xa1, 0x36, 0x1c, 0x49, 0xb8, 0x0d, 0xee, 0x1c, 0xba, 0x01, 0xe0, 0x3b, 0x42,
	0x72, 0x61, 0xf8, 0x9e, 0x5c, 0x3e, 0x3f, 0x58, 0xee, 0x59, 0x95, 0xc7, 0x3f, 0x72, 0x5e, 0xd1,
	0x1b, 0x2f, 0x3f, 0x7c, 0x92, 0x13, 0x1e, 0x3f, 0xc9, 0x09, 0xbf, 0x3f, 0xc9, 0x09, 0x5f, 0x3e,
	0xcd, 0x8d, 0x3d, 0x7e, 0x9a, 0x1b, 0xfb, 0xf5, 0x69, 0x6e, 0xec, 0xbd, 0x25, 0xdd, 0xb0, 0xef,
	0x74, 0x77, 0x4a, 0xbb, 0x96, 0xe9, 0xfe, 0x8a, 0x5d, 0xf6, 0x9d, 0x2e, 0x07, 0xf4, 0x7c, 0xd9,
	0x99, 0x20, 0x17, 0xd1, 0xf5, 0x7f, 0x02, 0x00, 0x00, 0xff, 0xff, 0x4d, 0x77, 0x54, 0x7d, 0x37,
	0x17, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ context.Context
	_ grpc.ClientConn
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

func (m *MsgSubmitProposal) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgSubmitProposal) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgSubmitProposal) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Summary) > 0 {
		i -= len(m.Summary)
		copy(dAtA[i:], m.Summary)
		i = encodeVarintTx(dAtA, i, uint64(len(m.Summary)))
		i--
		dAtA[i] = 0x3a
	}
	if len(m.Title) > 0 {
		i -= len(m.Title)
		copy(dAtA[i:], m.Title)
		i = encodeVarintTx(dAtA, i, uint64(len(m.Title)))
		i--
		dAtA[i] = 0x32
	}
	if m.Exec != 0 {
		i = encodeVarintTx(dAtA, i, uint64(m.Exec))
		i--
		dAtA[i] = 0x28
	}
	if len(m.Messages) > 0 {
		for iNdEx := len(m.Messages) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Messages[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintTx(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x22
		}
	}
	if len(m.Metadata) > 0 {
		i -= len(m.Metadata)
		copy(dAtA[i:], m.Metadata)
		i = encodeVarintTx(dAtA, i, uint64(len(m.Metadata)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Proposers) > 0 {
		for iNdEx := len(m.Proposers) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Proposers[iNdEx])
			copy(dAtA[i:], m.Proposers[iNdEx])
			i = encodeVarintTx(dAtA, i, uint64(len(m.Proposers[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.GroupPolicyAddress) > 0 {
		i -= len(m.GroupPolicyAddress)
		copy(dAtA[i:], m.GroupPolicyAddress)
		i = encodeVarintTx(dAtA, i, uint64(len(m.GroupPolicyAddress)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *MsgSubmitProposalResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgSubmitProposalResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgSubmitProposalResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.ProposalId != 0 {
		i = encodeVarintTx(dAtA, i, uint64(m.ProposalId))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *MsgSubmitProposal) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.GroupPolicyAddress)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	if len(m.Proposers) > 0 {
		for _, s := range m.Proposers {
			l = len(s)
			n += 1 + l + sovTx(uint64(l))
		}
	}
	l = len(m.Metadata)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	if len(m.Messages) > 0 {
		for _, e := range m.Messages {
			l = e.Size()
			n += 1 + l + sovTx(uint64(l))
		}
	}
	if m.Exec != 0 {
		n += 1 + sovTx(uint64(m.Exec))
	}
	l = len(m.Title)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	l = len(m.Summary)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	return n
}

func (m *MsgSubmitProposalResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ProposalId != 0 {
		n += 1 + sovTx(uint64(m.ProposalId))
	}
	return n
}

func (m *MsgSubmitProposal) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTx
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
			return fmt.Errorf("proto: MsgSubmitProposal: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgSubmitProposal: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field GroupPolicyAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
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
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.GroupPolicyAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Proposers", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
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
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Proposers = append(m.Proposers, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metadata", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
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
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Metadata = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Messages", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
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
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Messages = append(m.Messages, &types.Any{})
			if err := m.Messages[len(m.Messages)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Exec", wireType)
			}
			m.Exec = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Exec |= Exec(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Title", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
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
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Title = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Summary", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
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
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Summary = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTx
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

func (m *MsgSubmitProposalResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTx
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
			return fmt.Errorf("proto: MsgSubmitProposalResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgSubmitProposalResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ProposalId", wireType)
			}
			m.ProposalId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ProposalId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTx
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
