package types

import (
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetRecipientBech32Address returns the recipient address as a string.
// Should be called after ValidateBasic hence should not panic.
func (m *DemandOrder) GetRecipientBech32Address() sdk.AccAddress {
	recipientBech32, err := sdk.AccAddressFromBech32(m.Recipient)
	if err != nil {
		panic(err)
	}
	return recipientBech32
}

// GetFeeAmount returns the fee amount of the demand order.
func (m *DemandOrder) GetFeeAmount() math.Int {
	return m.Fee.AmountOf(m.Price[0].Denom)
}

func (m *DemandOrder) IsFulfilled() bool {
	return m.FulfillerAddress != "" || m.DeprecatedIsFulfilled
}
