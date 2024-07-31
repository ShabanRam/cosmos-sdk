package symSlash

import (
	"fmt"

	modulev1 "cosmossdk.io/api/cosmos/symSlash/module/v1"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/comet"
	"cosmossdk.io/depinject"
	"cosmossdk.io/depinject/appconfig"
	authtypes "cosmossdk.io/x/auth/types"
	"cosmossdk.io/x/symSlash/keeper"
	"cosmossdk.io/x/symSlash/types"
	staking "cosmossdk.io/x/symStaking/types"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
)

var _ depinject.OnePerModuleType = AppModule{}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (am AppModule) IsOnePerModuleType() {}

func init() {
	appconfig.RegisterModule(
		&modulev1.Module{},
		appconfig.Provide(ProvideModule),
	)
}

type ModuleInputs struct {
	depinject.In

	Config       *modulev1.Module
	Environment  appmodule.Environment
	Cdc          codec.Codec
	Registry     cdctypes.InterfaceRegistry
	CometService comet.Service

	AccountKeeper types.AccountKeeper
	BankKeeper    types.BankKeeper
	StakingKeeper types.StakingKeeper
}

type ModuleOutputs struct {
	depinject.Out

	Keeper keeper.Keeper
	Module appmodule.AppModule
	Hooks  staking.StakingHooksWrapper
}

func ProvideModule(in ModuleInputs) ModuleOutputs {
	// default to governance authority if not provided
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	if in.Config.Authority != "" {
		authority = authtypes.NewModuleAddressOrBech32Address(in.Config.Authority)
	}

	authStr, err := in.AccountKeeper.AddressCodec().BytesToString(authority)
	if err != nil {
		panic(fmt.Errorf("unable to decode authority in slashing: %w", err))
	}

	k := keeper.NewKeeper(in.Environment, in.Cdc, nil, in.StakingKeeper, authStr)
	m := NewAppModule(in.Cdc, k, in.AccountKeeper, in.BankKeeper, in.StakingKeeper, in.Registry, in.CometService)
	return ModuleOutputs{
		Keeper: k,
		Module: m,
		Hooks:  staking.StakingHooksWrapper{StakingHooks: k.Hooks()},
	}
}
