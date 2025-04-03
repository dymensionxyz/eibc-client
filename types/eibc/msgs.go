package types

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	// ModuleName defines the module name
	ModuleName = "delayedack"

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	TypeMsgFinalizedPacketByPacketKey = "finalized_packet_by_packet_key"
)

func (m MsgFinalizePacketByPacketKey) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return errors.Join(
			sdkerrors.ErrInvalidAddress,
			errorsmod.Wrapf(err, "sender must be a valid bech32 address: %s", m.Sender),
		)
	}
	if len(m.PacketKey) == 0 {
		return errors.New("rollappId must be non-empty")
	}

	if _, err := DecodePacketKey(m.PacketKey); err != nil {
		return errors.New("packet key must be a valid base64 encoded string")
	}
	return nil
}

func (m MsgFinalizePacketByPacketKey) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m MsgFinalizePacketByPacketKey) MustDecodePacketKey() []byte {
	packetKey, err := DecodePacketKey(m.PacketKey)
	if err != nil {
		panic(fmt.Errorf("failed to decode base64 packet key: %w", err))
	}
	return packetKey
}

func (m *MsgFinalizePacketByPacketKey) Route() string {
	return RouterKey
}

func (m *MsgFinalizePacketByPacketKey) Type() string {
	return TypeMsgFinalizedPacketByPacketKey
}

func (*MsgFinalizePacketByPacketKey) XXX_MessageName() string {
	return "dymensionxyz.dymension.delayedack.MsgFinalizePacketByPacketKey"
}

// DecodePacketKey decodes packet key from base64 to bytes.
func DecodePacketKey(packetKey string) ([]byte, error) {
	rollappPacketKeyBytes := make([]byte, base64.StdEncoding.DecodedLen(len(packetKey)))
	_, err := base64.StdEncoding.Decode(rollappPacketKeyBytes, []byte(packetKey))
	if err != nil {
		return nil, err
	}
	rollappPacketKeyBytes = bytes.TrimRight(rollappPacketKeyBytes, "\x00") // remove padding
	return rollappPacketKeyBytes, nil
}
