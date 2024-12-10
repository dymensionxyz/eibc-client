package types

import (
	"encoding/hex"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ = sdk.Msg(&MsgFulfillOrderAuthorized{})

func NewMsgFulfillOrderAuthorized(
	orderId,
	rollappId,
	granterAddress,
	operatorFeeAddress,
	expectedFee string,
	price sdk.Coins,
	amount sdk.IntProto,
	fulfillerFeePart sdk.DecProto,
	settlementValidated bool,
) *MsgFulfillOrderAuthorized {
	return &MsgFulfillOrderAuthorized{
		OrderId:             orderId,
		RollappId:           rollappId,
		LpAddress:           granterAddress,
		OperatorFeeAddress:  operatorFeeAddress,
		ExpectedFee:         expectedFee,
		Price:               price,
		Amount:              amount,
		OperatorFeeShare:    fulfillerFeePart,
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

	if !msg.Price.IsValid() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "price is invalid")
	}

	if msg.Amount.Int.IsNil() || msg.Amount.Int.IsNegative() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "amount cannot be empty or negative")
	}

	if msg.OperatorFeeShare.Dec.IsNil() || msg.OperatorFeeShare.Dec.IsNegative() {
		return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "operator fee share cannot be empty or negative")
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
