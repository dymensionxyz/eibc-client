package types

// DONTCOVER

import (
	errorsmod "cosmossdk.io/errors"
)

// x/eibc module sentinel errors
var (
	ErrInvalidOrderID = errorsmod.Register(ModuleName, 3, "Invalid order ID")
	ErrNegativeFee    = errorsmod.Register(ModuleName, 13, "Fee must be greater than or equal to 0")
)
