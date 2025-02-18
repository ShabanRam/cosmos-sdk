package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	abci_server "github.com/cometbft/cometbft/abci/server"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	corectx "cosmossdk.io/core/context"
	"cosmossdk.io/log"
	staking "cosmossdk.io/x/symStaking"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/server"
	servercmtlog "github.com/cosmos/cosmos-sdk/server/log"
	"github.com/cosmos/cosmos-sdk/server/mock"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/cosmos/cosmos-sdk/testutil/network"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/symGenutil"
	symGenutilcli "github.com/cosmos/cosmos-sdk/x/symGenutil/client/cli"
	symGenutiltest "github.com/cosmos/cosmos-sdk/x/symGenutil/client/testutil"
	symGenutiltypes "github.com/cosmos/cosmos-sdk/x/symGenutil/types"
)

var testMbm = module.NewManager(
	staking.NewAppModule(makeCodec(), nil, nil, nil),
	symGenutil.NewAppModule(makeCodec(), nil, nil, nil, nil, nil),
)

func TestInitCmd(t *testing.T) {
	tests := []struct {
		name      string
		flags     func(dir string) []string
		shouldErr bool
		err       error
	}{
		{
			name: "happy path",
			flags: func(dir string) []string {
				return []string{
					"appnode-test",
				}
			},
			shouldErr: false,
			err:       nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			logger := log.NewNopLogger()
			viper := viper.New()

			err := writeAndTrackDefaultConfig(viper, home)
			require.NoError(t, err)
			interfaceRegistry := types.NewInterfaceRegistry()
			marshaler := codec.NewProtoCodec(interfaceRegistry)
			clientCtx := client.Context{}.
				WithCodec(marshaler).
				WithLegacyAmino(makeAminoCodec()).
				WithHomeDir(home)

			ctx := context.Background()
			ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
			ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
			ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

			cmd := symGenutilcli.InitCmd(testMbm)
			cmd.SetArgs(
				tt.flags(home),
			)

			if tt.shouldErr {
				err := cmd.ExecuteContext(ctx)
				require.EqualError(t, err, tt.err.Error())
			} else {
				require.NoError(t, cmd.ExecuteContext(ctx))
			}
		})
	}
}

func TestInitRecover(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	viper := viper.New()

	err := writeAndTrackDefaultConfig(viper, home)
	require.NoError(t, err)
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithLegacyAmino(makeAminoCodec()).
		WithHomeDir(home)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
	ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

	cmd := symGenutilcli.InitCmd(testMbm)
	cmd.SetContext(ctx)
	mockIn := testutil.ApplyMockIODiscardOutErr(cmd)

	cmd.SetArgs([]string{
		"appnode-test",
		fmt.Sprintf("--%s=true", symGenutilcli.FlagRecover),
	})

	// use valid mnemonic and complete recovery key generation successfully
	mockIn.Reset("decide praise business actor peasant farm drastic weather extend front hurt later song give verb rhythm worry fun pond reform school tumble august one\n")
	require.NoError(t, cmd.ExecuteContext(ctx))
}

func TestInitDefaultBondDenom(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	viper := viper.New()

	err := writeAndTrackDefaultConfig(viper, home)
	require.NoError(t, err)
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithLegacyAmino(makeAminoCodec()).
		WithHomeDir(home)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
	ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

	cmd := symGenutilcli.InitCmd(testMbm)

	cmd.SetArgs([]string{
		"appnode-test",
		fmt.Sprintf("--%s=testtoken", symGenutilcli.FlagDefaultBondDenom),
	})
	require.NoError(t, cmd.ExecuteContext(ctx))
}

func TestEmptyState(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	viper := viper.New()

	err := writeAndTrackDefaultConfig(viper, home)
	require.NoError(t, err)
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithLegacyAmino(makeAminoCodec()).
		WithHomeDir(home)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
	ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

	cmd := symGenutilcli.InitCmd(testMbm)
	cmd.SetArgs([]string{"appnode-test"})

	require.NoError(t, cmd.ExecuteContext(ctx))

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd = symGenutilcli.ExportCmd(nil)
	require.NoError(t, cmd.ExecuteContext(ctx))

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		require.NoError(t, err)

		outC <- buf.String()
	}()

	w.Close()
	os.Stdout = old
	out := <-outC

	require.Contains(t, out, "genesis_time")
	require.Contains(t, out, "chain_id")
	require.Contains(t, out, "consensus")
	require.Contains(t, out, "app_hash")
	require.Contains(t, out, "app_state")
}

func TestStartStandAlone(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	err := symGenutiltest.ExecInitCmd(testMbm, home, marshaler)
	require.NoError(t, err)

	app, err := mock.NewApp(home, logger)
	require.NoError(t, err)

	svrAddr, _, closeFn, err := network.FreeTCPAddr()
	require.NoError(t, err)
	require.NoError(t, closeFn())

	cmtApp := server.NewCometABCIWrapper(app)
	svr, err := abci_server.NewServer(svrAddr, "socket", cmtApp)
	require.NoError(t, err, "error creating listener")

	svr.SetLogger(servercmtlog.CometLoggerWrapper{Logger: logger.With("module", "abci-server")})
	err = svr.Start()
	require.NoError(t, err)

	timer := time.NewTimer(time.Duration(2) * time.Second)
	for range timer.C {
		err = svr.Stop()
		require.NoError(t, err)
		break
	}
}

func TestInitNodeValidatorFiles(t *testing.T) {
	home := t.TempDir()
	cfg, err := symGenutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	nodeID, valPubKey, err := symGenutil.InitializeNodeValidatorFiles(cfg, ed25519.KeyType)
	require.NoError(t, err)

	require.NotEqual(t, "", nodeID)
	require.NotEqual(t, 0, len(valPubKey.Bytes()))
}

func TestInitConfig(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	viper := viper.New()

	err := writeAndTrackDefaultConfig(viper, home)
	require.NoError(t, err)
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithLegacyAmino(makeAminoCodec()).
		WithChainID("foo"). // add chain-id to clientCtx
		WithHomeDir(home)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
	ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

	cmd := symGenutilcli.InitCmd(testMbm)
	cmd.SetArgs([]string{"testnode"})

	err = cmd.ExecuteContext(ctx)
	require.NoError(t, err)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd = symGenutilcli.ExportCmd(nil)
	require.NoError(t, cmd.ExecuteContext(ctx))

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		require.NoError(t, err)
		outC <- buf.String()
	}()

	w.Close()
	os.Stdout = old
	out := <-outC

	require.Contains(t, out, "\"chain_id\": \"foo\"")
}

func TestInitWithHeight(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	viper := viper.New()
	cfg, err := symGenutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	err = writeAndTrackDefaultConfig(viper, home)
	require.NoError(t, err)
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithLegacyAmino(makeAminoCodec()).
		WithChainID("foo"). // add chain-id to clientCtx
		WithHomeDir(home)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
	ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

	testInitialHeight := int64(333)

	cmd := symGenutilcli.InitCmd(testMbm)

	fmt.Println("RootDir", viper.Get(flags.FlagHome))
	cmd.SetArgs([]string{"init-height-test", fmt.Sprintf("--%s=%d", flags.FlagInitHeight, testInitialHeight)})

	require.NoError(t, cmd.ExecuteContext(ctx))

	appGenesis, importErr := symGenutiltypes.AppGenesisFromFile(cfg.GenesisFile())
	require.NoError(t, importErr)

	require.Equal(t, testInitialHeight, appGenesis.InitialHeight)
}

func TestInitWithNegativeHeight(t *testing.T) {
	home := t.TempDir()
	logger := log.NewNopLogger()
	viper := viper.New()
	cfg, err := symGenutiltest.CreateDefaultCometConfig(home)
	require.NoError(t, err)

	err = writeAndTrackDefaultConfig(viper, home)
	require.NoError(t, err)
	interfaceRegistry := types.NewInterfaceRegistry()
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithLegacyAmino(makeAminoCodec()).
		WithChainID("foo"). // add chain-id to clientCtx
		WithHomeDir(home)

	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
	ctx = context.WithValue(ctx, corectx.ViperContextKey, viper)
	ctx = context.WithValue(ctx, corectx.LoggerContextKey, logger)

	testInitialHeight := int64(-333)

	cmd := symGenutilcli.InitCmd(testMbm)
	cmd.SetArgs([]string{"init-height-test", fmt.Sprintf("--%s=%d", flags.FlagInitHeight, testInitialHeight)})

	require.NoError(t, cmd.ExecuteContext(ctx))

	appGenesis, importErr := symGenutiltypes.AppGenesisFromFile(cfg.GenesisFile())
	require.NoError(t, importErr)

	require.Equal(t, int64(1), appGenesis.InitialHeight)
}

// custom tx codec
func makeAminoCodec() *codec.LegacyAmino {
	cdc := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(cdc)
	cryptocodec.RegisterCrypto(cdc)
	return cdc
}

func makeCodec() codec.Codec {
	interfaceRegistry := types.NewInterfaceRegistry()
	return codec.NewProtoCodec(interfaceRegistry)
}

func writeAndTrackDefaultConfig(v *viper.Viper, home string) error {
	cfg, err := symGenutiltest.CreateDefaultCometConfig(home)
	if err != nil {
		return err
	}
	return symGenutiltest.WriteAndTrackCometConfig(v, home, cfg)
}
