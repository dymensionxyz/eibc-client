package types

import (
	"encoding/hex"
	"errors"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ = sdk.Msg(&MsgFulfillOrderAuthorized{})
	_ = sdk.Msg(&MsgFinalizePacketByPacketKey{})
)

func NewMsgFulfillOrderAuthorized(
	orderId,
	rollappId,
	granterAddress,
	operatorFeeAddress,
	expectedFee string,
	price sdk.Coins,
	operatorFeePart sdk.Dec,
	settlementValidated bool,
) *MsgFulfillOrderAuthorized {
	return &MsgFulfillOrderAuthorized{
		OrderId:             orderId,
		RollappId:           rollappId,
		LpAddress:           granterAddress,
		OperatorFeeAddress:  operatorFeeAddress,
		ExpectedFee:         expectedFee,
		Price:               price,
		OperatorFeeShare:    sdk.DecProto{Dec: operatorFeePart},
		SettlementValidated: settlementValidated,
	}
}

func (msg *MsgFulfillOrderAuthorized) Route() string {
	return RouterKey
}

func (msg *MsgFulfillOrderAuthorized) Type() string {
	return sdk.MsgTypeURL(msg)
}

func (*MsgFulfillOrderAuthorized) XXX_MessageName() string {
	return "dymensionxyz.dymension.eibc.MsgFulfillOrderAuthorized"
}

func (msg *MsgFulfillOrderAuthorized) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.LpAddress)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgFulfillOrderAuthorized) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgFulfillOrderAuthorized) ValidateBasic() error {
	if msg.RollappId == "" {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "rollapp id cannot be empty")
	}

	err := validateCommon(msg.OrderId, msg.ExpectedFee, msg.OperatorFeeAddress, msg.LpAddress)
	if err != nil {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, err.Error())
	}

	if msg.Price.IsAnyNegative() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "price cannot be negative")
	}

	if msg.OperatorFeeShare.Dec.IsNegative() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "operator fee share cannot be negative")
	}

	if msg.OperatorFeeShare.Dec.GT(sdk.OneDec()) {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "operator fee share cannot be greater than 1")
	}

	return nil
}

func (msg *MsgFulfillOrderAuthorized) GetLPBech32Address() []byte {
	return sdk.MustAccAddressFromBech32(msg.LpAddress)
}

func (msg *MsgFulfillOrderAuthorized) GetOperatorFeeBech32Address() []byte {
	return sdk.MustAccAddressFromBech32(msg.OperatorFeeAddress)
}

func isValidOrderId(orderId string) bool {
	hashBytes, err := hex.DecodeString(orderId)
	if err != nil {
		// The string is not a valid hexadecimal string
		return false
	}
	// SHA-256 hashes are 32 bytes long
	return len(hashBytes) == 32
}

func validateCommon(orderId, fee string, address ...string) error {
	if !isValidOrderId(orderId) {
		return fmt.Errorf("%w: %s", ErrInvalidOrderID, orderId)
	}

	for _, addr := range address {
		_, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			return err
		}
	}

	feeInt, ok := sdk.NewIntFromString(fee)
	if !ok {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, fmt.Sprintf("parse fee: %s", fee))
	}

	if feeInt.IsNegative() {
		return ErrNegativeFee
	}

	return nil
}

func (m MsgFinalizePacketByPacketKey) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return errors.Join(
			sdkerrors.ErrInvalidAddress,
			errorsmod.Wrapf(err, "sender must be a valid bech32 address: %s", m.Sender),
		)
	}
	if len(m.PacketKey) == 0 {
		return fmt.Errorf("packet key must be non-empty")
	}

	return nil
}

func (m MsgFinalizePacketByPacketKey) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (*MsgFinalizePacketByPacketKey) XXX_MessageName() string {
	return "dymensionxyz.dymension.delayedack.MsgFinalizePacketByPacketKey"
}
