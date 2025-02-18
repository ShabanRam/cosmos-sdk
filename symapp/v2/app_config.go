//nolint:unused,nolintlint // ignore unused code linting and directive `//nolint:unused // ignore unused code linting` is unused for linter "unused"
package symapp

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	accountsmodulev1 "cosmossdk.io/api/cosmos/accounts/module/v1"
	runtimev2 "cosmossdk.io/api/cosmos/app/runtime/v2"
	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	authmodulev1 "cosmossdk.io/api/cosmos/auth/module/v1"
	authzmodulev1 "cosmossdk.io/api/cosmos/authz/module/v1"
	bankmodulev1 "cosmossdk.io/api/cosmos/bank/module/v1"
	circuitmodulev1 "cosmossdk.io/api/cosmos/circuit/module/v1"
	consensusmodulev1 "cosmossdk.io/api/cosmos/consensus/module/v1"
	feegrantmodulev1 "cosmossdk.io/api/cosmos/feegrant/module/v1"
	groupmodulev1 "cosmossdk.io/api/cosmos/group/module/v1"
	poolmodulev1 "cosmossdk.io/api/cosmos/protocolpool/module/v1"
	genutilmodulev1 "cosmossdk.io/api/cosmos/symGenutil/module/v1"
	govmodulev1 "cosmossdk.io/api/cosmos/symGov/module/v1"
	slashingmodulev1 "cosmossdk.io/api/cosmos/symSlash/module/v1"
	stakingmodulev1 "cosmossdk.io/api/cosmos/symStaking/module/v1"
	txconfigv1 "cosmossdk.io/api/cosmos/tx/config/v1"
	upgrademodulev1 "cosmossdk.io/api/cosmos/upgrade/module/v1"
	vestingmodulev1 "cosmossdk.io/api/cosmos/vesting/module/v1"
	"cosmossdk.io/depinject/appconfig"
	"cosmossdk.io/x/accounts"
	_ "cosmossdk.io/x/auth"           // import for side-effects
	_ "cosmossdk.io/x/auth/tx/config" // import for side-effects
	authtypes "cosmossdk.io/x/auth/types"
	_ "cosmossdk.io/x/auth/vesting" // import for side-effects
	vestingtypes "cosmossdk.io/x/auth/vesting/types"
	"cosmossdk.io/x/authz"
	_ "cosmossdk.io/x/authz/module" // import for side-effects
	_ "cosmossdk.io/x/bank"         // import for side-effects
	banktypes "cosmossdk.io/x/bank/types"
	_ "cosmossdk.io/x/circuit" // import for side-effects
	circuittypes "cosmossdk.io/x/circuit/types"
	_ "cosmossdk.io/x/consensus" // import for side-effects
	consensustypes "cosmossdk.io/x/consensus/types"
	"cosmossdk.io/x/feegrant"
	_ "cosmossdk.io/x/feegrant/module" // import for side-effects
	"cosmossdk.io/x/group"
	_ "cosmossdk.io/x/group/module" // import for side-effects
	_ "cosmossdk.io/x/protocolpool" // import for side-effects
	pooltypes "cosmossdk.io/x/protocolpool/types"
	_ "cosmossdk.io/x/symGov" // import for side-effects
	govtypes "cosmossdk.io/x/symGov/types"
	_ "cosmossdk.io/x/symSlash" // import for side-effects
	slashingtypes "cosmossdk.io/x/symSlash/types"
	_ "cosmossdk.io/x/symStaking" // import for side-effects
	stakingtypes "cosmossdk.io/x/symStaking/types"
	_ "cosmossdk.io/x/upgrade" // import for side-effects
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/runtime"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/symGenutil/types"
)

var (
	// module account permissions
	moduleAccPerms = []*authmodulev1.ModuleAccountPermission{
		{Account: authtypes.FeeCollectorName},
		{Account: pooltypes.ModuleName},
		{Account: pooltypes.StreamAccount},
		{Account: govtypes.ModuleName, Permissions: []string{authtypes.Burner}},
	}

	// blocked account addresses
	blockAccAddrs = []string{
		authtypes.FeeCollectorName,
		// We allow the following module accounts to receive funds:
		// govtypes.ModuleName
		// pooltypes.ModuleName
	}

	// application configuration (used by depinject)
	appConfig = appconfig.Compose(&appv1alpha1.Config{
		Modules: []*appv1alpha1.ModuleConfig{
			{
				Name: runtime.ModuleName,
				Config: appconfig.WrapAny(&runtimev2.Module{
					AppName: "symappV2",
					// NOTE: upgrade module is required to be prioritized
					PreBlockers: []string{
						upgradetypes.ModuleName,
					},
					// During begin block slashing happens after distr.BeginBlocker so that
					// there is nothing left over in the validator fee pool, so as to keep the
					// CanWithdrawInvariant invariant.
					// NOTE: staking module is required if HistoricalEntries param > 0
					BeginBlockers: []string{
						slashingtypes.ModuleName,
						stakingtypes.ModuleName,
						authz.ModuleName,
					},
					EndBlockers: []string{
						govtypes.ModuleName,
						stakingtypes.ModuleName,
						feegrant.ModuleName,
						group.ModuleName,
						pooltypes.ModuleName,
					},
					OverrideStoreKeys: []*runtimev2.StoreKeyConfig{
						{
							ModuleName: authtypes.ModuleName,
							KvStoreKey: "acc",
						},
					},
					// NOTE: The genutils module must occur after staking so that pools are
					// properly initialized with tokens from genesis accounts.
					// NOTE: The genutils module must also occur after auth so that it can access the params from auth.
					InitGenesis: []string{
						accounts.ModuleName,
						authtypes.ModuleName,
						banktypes.ModuleName,
						stakingtypes.ModuleName,
						slashingtypes.ModuleName,
						govtypes.ModuleName,
						genutiltypes.ModuleName,
						authz.ModuleName,
						feegrant.ModuleName,
						group.ModuleName,
						upgradetypes.ModuleName,
						vestingtypes.ModuleName,
						circuittypes.ModuleName,
						pooltypes.ModuleName,
					},
					// When ExportGenesis is not specified, the export genesis module order
					// is equal to the init genesis order
					// ExportGenesis: []string{},
					// Uncomment if you want to set a custom migration order here.
					// OrderMigrations: []string{},
					// TODO GasConfig was added to the config in runtimev2.  Where/how was it set in v1?
					GasConfig: &runtimev2.GasConfig{
						ValidateTxGasLimit: 100_000,
						QueryGasLimit:      100_000,
						SimulationGasLimit: 100_000,
					},
				}),
			},
			{
				Name: authtypes.ModuleName,
				Config: appconfig.WrapAny(&authmodulev1.Module{
					Bech32Prefix:             "cosmos",
					ModuleAccountPermissions: moduleAccPerms,
					// By default modules authority is the governance module. This is configurable with the following:
					// Authority: "group", // A custom module authority can be set using a module name
					// Authority: "cosmos1cwwv22j5ca08ggdv9c2uky355k908694z577tv", // or a specific address
				}),
			},
			{
				Name:   vestingtypes.ModuleName,
				Config: appconfig.WrapAny(&vestingmodulev1.Module{}),
			},
			{
				Name: banktypes.ModuleName,
				Config: appconfig.WrapAny(&bankmodulev1.Module{
					BlockedModuleAccountsOverride: blockAccAddrs,
				}),
			},
			{
				Name: stakingtypes.ModuleName,
				Config: appconfig.WrapAny(&stakingmodulev1.Module{
					// NOTE: specifying a prefix is only necessary when using bech32 addresses
					// If not specified, the auth Bech32Prefix appended with "valoper" and "valcons" is used by default
					Bech32PrefixValidator: "cosmosvaloper",
					Bech32PrefixConsensus: "cosmosvalcons",
				}),
			},
			{
				Name:   slashingtypes.ModuleName,
				Config: appconfig.WrapAny(&slashingmodulev1.Module{}),
			},
			{
				Name:   "tx",
				Config: appconfig.WrapAny(&txconfigv1.Config{}),
			},
			{
				Name:   genutiltypes.ModuleName,
				Config: appconfig.WrapAny(&genutilmodulev1.Module{}),
			},
			{
				Name:   authz.ModuleName,
				Config: appconfig.WrapAny(&authzmodulev1.Module{}),
			},
			{
				Name:   upgradetypes.ModuleName,
				Config: appconfig.WrapAny(&upgrademodulev1.Module{}),
			},
			{
				Name: group.ModuleName,
				Config: appconfig.WrapAny(&groupmodulev1.Module{
					MaxExecutionPeriod: durationpb.New(time.Second * 1209600),
					MaxMetadataLen:     255,
				}),
			},
			{
				Name:   feegrant.ModuleName,
				Config: appconfig.WrapAny(&feegrantmodulev1.Module{}),
			},
			{
				Name:   govtypes.ModuleName,
				Config: appconfig.WrapAny(&govmodulev1.Module{}),
			},
			{
				Name: consensustypes.ModuleName,
				Config: appconfig.WrapAny(&consensusmodulev1.Module{
					Authority: "consensus",
				}),
			},
			{
				Name:   accounts.ModuleName,
				Config: appconfig.WrapAny(&accountsmodulev1.Module{}),
			},
			{
				Name:   circuittypes.ModuleName,
				Config: appconfig.WrapAny(&circuitmodulev1.Module{}),
			},
			{
				Name:   pooltypes.ModuleName,
				Config: appconfig.WrapAny(&poolmodulev1.Module{}),
			},
		},
	})
)
