package types

import (
	"encoding/binary"
)

var _ binary.ByteOrder

const (
	// ModuleName defines the module name
	ModuleName = "eibc"
	// RouterKey defines the module's message routing key
	RouterKey = ModuleName
)
