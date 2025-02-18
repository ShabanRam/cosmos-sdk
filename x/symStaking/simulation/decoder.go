package simulation

import (
	"bytes"
	"fmt"

	"cosmossdk.io/math"
	"cosmossdk.io/x/symStaking/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/kv"
)

// NewDecodeStore returns a decoder function closure that unmarshals the KVPair's
// Value to the corresponding staking type.
func NewDecodeStore(cdc codec.Codec) func(kvA, kvB kv.Pair) string {
	return func(kvA, kvB kv.Pair) string {
		switch {
		case bytes.Equal(kvA.Key[:1], types.LastTotalPowerKey):
			var powerA, powerB math.Int
			if err := powerA.Unmarshal(kvA.Value); err != nil {
				panic(err)
			}
			if err := powerB.Unmarshal(kvB.Value); err != nil {
				panic(err)
			}
			return fmt.Sprintf("%v\n%v", powerA, powerB)

		case bytes.Equal(kvA.Key[:1], types.ValidatorsKey):
			var validatorA, validatorB types.Validator

			cdc.MustUnmarshal(kvA.Value, &validatorA)
			cdc.MustUnmarshal(kvB.Value, &validatorB)

			return fmt.Sprintf("%v\n%v", validatorA, validatorB)
		case bytes.Equal(kvA.Key[:1], types.LastValidatorPowerKey),
			bytes.Equal(kvA.Key[:1], types.ValidatorsByConsAddrKey),
			bytes.Equal(kvA.Key[:1], types.ValidatorsByPowerIndexKey):
			return fmt.Sprintf("%v\n%v", sdk.ValAddress(kvA.Value), sdk.ValAddress(kvB.Value))

		case bytes.Equal(kvA.Key[:1], types.ParamsKey):
			var paramsA, paramsB types.Params

			cdc.MustUnmarshal(kvA.Value, &paramsA)
			cdc.MustUnmarshal(kvB.Value, &paramsB)

			return fmt.Sprintf("%v\n%v", paramsA, paramsB)
		default:
			panic(fmt.Sprintf("invalid staking key prefix %X", kvA.Key[:1]))
		}
	}
}
