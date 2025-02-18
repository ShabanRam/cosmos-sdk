package symapp

import (
	_ "embed"
	"path/filepath"

	"github.com/spf13/viper"

	clienthelpers "cosmossdk.io/client/v2/helpers"
	coreapp "cosmossdk.io/core/app"
	"cosmossdk.io/core/legacy"
	"cosmossdk.io/core/log"
	"cosmossdk.io/core/transaction"
	"cosmossdk.io/depinject"
	"cosmossdk.io/runtime/v2"
	serverv2 "cosmossdk.io/server/v2"
	"cosmossdk.io/store/v2"
	"cosmossdk.io/store/v2/commitment/iavl"
	"cosmossdk.io/store/v2/db"
	"cosmossdk.io/store/v2/root"
	"cosmossdk.io/x/accounts"
	authkeeper "cosmossdk.io/x/auth/keeper"
	authzkeeper "cosmossdk.io/x/authz/keeper"
	bankkeeper "cosmossdk.io/x/bank/keeper"
	circuitkeeper "cosmossdk.io/x/circuit/keeper"
	consensuskeeper "cosmossdk.io/x/consensus/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	groupkeeper "cosmossdk.io/x/group/keeper"
	_ "cosmossdk.io/x/protocolpool"
	poolkeeper "cosmossdk.io/x/protocolpool/keeper"
	govkeeper "cosmossdk.io/x/symGov/keeper"
	slashingkeeper "cosmossdk.io/x/symSlash/keeper"
	stakingkeeper "cosmossdk.io/x/symStaking/keeper"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
)

// DefaultNodeHome default home directories for the application daemon
var DefaultNodeHome string

// symapp extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type SymApp[T transaction.Tx] struct {
	*runtime.App[T]
	legacyAmino       legacy.Amino
	appCodec          codec.Codec
	txConfig          client.TxConfig
	interfaceRegistry codectypes.InterfaceRegistry

	// keepers
	AccountsKeeper        accounts.Keeper
	AuthKeeper            authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	GovKeeper             *govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	GroupKeeper           groupkeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper
	CircuitBreakerKeeper  circuitkeeper.Keeper
	PoolKeeper            poolkeeper.Keeper
}

func init() {
	var err error
	DefaultNodeHome, err = clienthelpers.GetNodeHomeDirectory(".symappv2")
	if err != nil {
		panic(err)
	}
}

// AppConfig returns the default app config.
func AppConfig() depinject.Config {
	return depinject.Configs(
		appConfig, // Alternatively use appconfig.LoadYAML(AppConfigYAML)
	)
}

// NewSymApp returns a reference to an initialized symapp.
func NewSymApp[T transaction.Tx](
	logger log.Logger,
	viper *viper.Viper,
) *SymApp[T] {
	viper.Set(serverv2.FlagHome, DefaultNodeHome) // TODO possibly set earlier when viper is created
	scRawDb, err := db.NewGoLevelDB("application", filepath.Join(DefaultNodeHome, "data"), nil)
	if err != nil {
		panic(err)
	}
	var (
		app        = &SymApp[T]{}
		appBuilder *runtime.AppBuilder[T]

		// merge the AppConfig and other configuration in one config
		appConfig = depinject.Configs(
			AppConfig(),
			depinject.Supply(
				logger,
				&root.FactoryOptions{
					Logger:  logger,
					RootDir: DefaultNodeHome,
					SSType:  0,
					SCType:  0,
					SCPruningOption: &store.PruningOption{
						KeepRecent: 0,
						Interval:   0,
					},
					IavlConfig: &iavl.Config{
						CacheSize:              100_000,
						SkipFastStorageUpgrade: true,
					},
					SCRawDB: scRawDb,
				},
				viper,

				// ADVANCED CONFIGURATION

				//
				// AUTH
				//
				// For providing a custom function required in auth to generate custom account types
				// add it below. By default the auth module uses simulation.RandomGenesisAccounts.
				//
				// authtypes.RandomGenesisAccountsFn(simulation.RandomGenesisAccounts),
				//
				// For providing a custom a base account type add it below.
				// By default the auth module uses authtypes.ProtoBaseAccount().
				//
				// func() sdk.AccountI { return authtypes.ProtoBaseAccount() },
				//
				// For providing a different address codec, add it below.
				// By default the auth module uses a Bech32 address codec,
				// with the prefix defined in the auth module configuration.
				//
				// func() address.Codec { return <- custom address codec type -> }

				//
				// STAKING
				//
				// For provinding a different validator and consensus address codec, add it below.
				// By default the staking module uses the bech32 prefix provided in the auth config,
				// and appends "valoper" and "valcons" for validator and consensus addresses respectively.
				// When providing a custom address codec in auth, custom address codecs must be provided here as well.
				//
				// func() runtime.ValidatorAddressCodec { return <- custom validator address codec type -> }
				// func() runtime.ConsensusAddressCodec { return <- custom consensus address codec type -> }

				//
				// MINT
				//

				// For providing a custom inflation function for x/mint add here your
				// custom function that implements the minttypes.InflationCalculationFn
				// interface.
			),
			depinject.Provide(
				codec.ProvideInterfaceRegistry,
				codec.ProvideAddressCodec,
				codec.ProvideProtoCodec,
				codec.ProvideLegacyAmino,
			),
			depinject.Invoke(
				std.RegisterInterfaces,
				std.RegisterLegacyAminoCodec,
			),
		)
	)

	if err := depinject.Inject(appConfig,
		&appBuilder,
		&app.appCodec,
		&app.legacyAmino,
		&app.txConfig,
		&app.interfaceRegistry,
		&app.AuthKeeper,
		&app.BankKeeper,
		&app.StakingKeeper,
		&app.SlashingKeeper,
		&app.GovKeeper,
		&app.UpgradeKeeper,
		&app.AuthzKeeper,
		&app.FeeGrantKeeper,
		&app.GroupKeeper,
		&app.ConsensusParamsKeeper,
		&app.CircuitBreakerKeeper,
		&app.PoolKeeper,
	); err != nil {
		panic(err)
	}

	app.App, err = appBuilder.Build()
	if err != nil {
		panic(err)
	}

	/****  Module Options ****/

	// RegisterUpgradeHandlers is used for registering any on-chain upgrades.
	app.RegisterUpgradeHandlers()

	// TODO (here or in runtime/v2)
	// wire simulation manager
	// wire snapshot manager
	// wire unordered tx manager

	if err := app.LoadLatest(); err != nil {
		panic(err)
	}
	return app
}

// AppCodec returns symapp's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *SymApp[T]) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns symapp's InterfaceRegistry.
func (app *SymApp[T]) InterfaceRegistry() coreapp.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns symapp's TxConfig.
func (app *SymApp[T]) TxConfig() client.TxConfig {
	return app.txConfig
}

// GetConsensusAuthority gets the consensus authority.
func (app *SymApp[T]) GetConsensusAuthority() string {
	return app.ConsensusParamsKeeper.GetAuthority()
}

// GetStore gets the app store.
func (app *SymApp[T]) GetStore() any {
	return app.App.GetStore()
}
