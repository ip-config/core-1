package init

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"terra/types/assets"

	"terra/app"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/cosmos/cosmos-sdk/x/stake"

	"github.com/cosmos/cosmos-sdk/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var (
	flagNodeDirPrefix     = "node-dir-prefix"
	flagNumValidators     = "v"
	flagOutputDir         = "output-dir"
	flagNodeDaemonHome    = "node-daemon-home"
	flagNodeCliHome       = "node-cli-home"
	flagStartingIPAddress = "starting-ip-address"

	flagPredefinedNodes = "predefined-nodes"
	flagSeedNode        = "seed"
)

const nodeDirPerm = 0755

// get cmd to initialize all files for tendermint testnet and application
func TestnetFilesCmd(ctx *server.Context, cdc *codec.Codec) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Initialize files for a Terrad testnet",
		Long: `testnet will create "v" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).
Note, strict routability for addresses is turned off in the config file.
Example:
	terrad testnet --v 4 --output-dir ./output --starting-ip-address 192.168.10.2
	`,
		RunE: func(_ *cobra.Command, _ []string) error {
			config := ctx.Config
			return initTestnet(config, cdc)
		},
	}

	cmd.Flags().Int(flagNumValidators, 4,
		"Number of validators to initialize the testnet with",
	)
	cmd.Flags().StringP(flagOutputDir, "o", "./mytestnet",
		"Directory to store initialization data for the testnet",
	)
	cmd.Flags().String(flagNodeDirPrefix, "node",
		"Prefix the directory name for each node with (node results in node0, node1, ...)",
	)
	cmd.Flags().String(flagNodeDaemonHome, "terrad",
		"Home directory of the node's daemon configuration",
	)
	cmd.Flags().String(flagNodeCliHome, "terracli",
		"Home directory of the node's cli configuration",
	)
	cmd.Flags().String(flagStartingIPAddress, "192.168.0.1",
		"Starting IP address (192.168.0.1 results in persistent peers list ID0@192.168.0.1:46656, ID1@192.168.0.2:46656, ...)")

	cmd.Flags().String(client.FlagChainID, "",
		"Genesis file chain-id, if left blank will be randomly created")
	cmd.Flags().String(flagPredefinedNodes, "",
		"Predefined node list, using this will override --starting-ip-address, --node-dir-prefix and --v (ex. \"node101@192.168.0.1,node102@192.168.0.22,node103@192.168.0.56\")")
	cmd.Flags().Bool(flagSeedNode, false,
		"Create  as seed")

	return cmd
}

type Node struct {
	Name string
	Ip   string
}

func makeNodes() ([]Node, error) {
	predefined := viper.GetString(flagPredefinedNodes)

	if len(predefined) > 0 {
		// parse predefined nodes information
		re, _ := regexp.Compile("[\\s,]*(\\w+)@([\\d.]+)[\\s,]*")
		groups := re.FindAllStringSubmatch(viper.GetString(flagPredefinedNodes), -1)

		nodes := make([]Node, len(groups))
		for i, item := range groups {
			nodes[i] = Node{item[1], item[2]}
		}

		return nodes, nil
	}

	// manipulate names & ips from startingIPAddress
	startingIPAddr := viper.GetString(flagStartingIPAddress)
	NumValidators := viper.GetInt(flagNumValidators)

	nodes := make([]Node, NumValidators)
	for i := 0; i < NumValidators; i++ {
		ip, err := calculateIP(startingIPAddr, i)
		if err != nil {
			return nil, err
		}
		nodes[i] = Node{fmt.Sprintf("%s%d", viper.GetString(flagNodeDirPrefix), i), ip}
	}
	return nodes, nil
}

func initTestnet(config *cfg.Config, cdc *codec.Codec) error {
	nodes, err := makeNodes()
	if err != nil {
		return err
	}

	outDir := viper.GetString(flagOutputDir)
	numValidators := len(nodes)
	config.P2P.SeedMode = viper.GetBool(flagSeedNode)

	chainID := viper.GetString(client.FlagChainID)
	if chainID == "" {
		chainID = fmt.Sprintf("chain-%v", cmn.RandStr(6))
	}

	monikers := make([]string, numValidators)
	nodeIDs := make([]string, numValidators)
	valPubKeys := make([]crypto.PubKey, numValidators)

	var (
		accs     []app.GenesisAccount
		genFiles []string
	)

	// generate private keys, node IDs, and initial transactions
	for i := 0; i < numValidators; i++ {
		nodeDirName := nodes[i].Name
		nodeDaemonHomeName := viper.GetString(flagNodeDaemonHome)
		nodeCliHomeName := viper.GetString(flagNodeCliHome)
		nodeDir := filepath.Join(outDir, nodeDirName, nodeDaemonHomeName)
		clientDir := filepath.Join(outDir, nodeDirName, nodeCliHomeName)
		gentxsDir := filepath.Join(outDir, "gentxs")

		config.SetRoot(nodeDir)

		err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}

		err = os.MkdirAll(clientDir, nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}

		monikers = append(monikers, nodeDirName)
		config.Moniker = nodeDirName

		ip := nodes[i].Ip
		nodeIDs[i], valPubKeys[i], err = InitializeNodeValidatorFiles(config)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}

		memo := fmt.Sprintf("%s@%s:26656", nodeIDs[i], ip)
		genFiles = append(genFiles, config.GenesisFile())

		keyPass := ""
		if len(viper.GetString(flagPredefinedNodes)) > 0 {
			buf := client.BufferStdin()
			prompt := fmt.Sprintf(
				"Password for account '%s' (default %s):", nodeDirName, app.DefaultKeyPass,
			)

			keyPass, err = client.GetPassword(prompt, buf)
			if err != nil && keyPass != "" {
				// An error was returned that either failed to read the password from
				// STDIN or the given password is not empty but failed to meet minimum
				// length requirements.
				return err
			}
		}

		if keyPass == "" {
			keyPass = app.DefaultKeyPass
		}

		addr, secret, err := server.GenerateSaveCoinKey(clientDir, nodeDirName, keyPass, true)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}

		info := map[string]string{"secret": secret}

		cliPrint, err := json.Marshal(info)
		if err != nil {
			return err
		}

		// save private key seed words
		err = writeFile(fmt.Sprintf("%v.json", "key_seed"), clientDir, cliPrint)
		if err != nil {
			return err
		}

		accs = append(accs, app.GenesisAccount{
			Address: addr,
			Coins: sdk.Coins{
				sdk.NewInt64Coin(assets.TerraDenom, 1000),
				sdk.NewInt64Coin(assets.LunaDenom, 150),
			},
		})

		msg := stake.NewMsgCreateValidator(
			sdk.ValAddress(addr),
			valPubKeys[i],
			sdk.NewInt64Coin(assets.LunaDenom, 100),
			stake.NewDescription(nodeDirName, "", "", ""),
			stake.NewCommissionMsg(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec()),
		)
		tx := auth.NewStdTx([]sdk.Msg{msg}, auth.StdFee{}, []auth.StdSignature{}, memo)
		txBldr := authtx.NewTxBuilderFromCLI().WithChainID(chainID).WithMemo(memo)

		signedTx, err := txBldr.SignStdTx(nodeDirName, app.DefaultKeyPass, tx, false)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}

		txBytes, err := cdc.MarshalJSON(signedTx)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}

		// gather gentxs folder
		err = writeFile(fmt.Sprintf("%v.json", nodeDirName), gentxsDir, txBytes)
		if err != nil {
			_ = os.RemoveAll(outDir)
			return err
		}
	}

	if err := initGenFiles(cdc, chainID, accs, genFiles, numValidators); err != nil {
		return err
	}

	err = collectGenFiles(
		cdc, config, chainID, monikers, nodeIDs, valPubKeys, numValidators,
		outDir, nodes, viper.GetString(flagNodeDaemonHome),
	)
	if err != nil {
		return err
	}

	fmt.Printf("Successfully initialized %d node directories\n", numValidators)
	return nil
}

func initGenFiles(
	cdc *codec.Codec, chainID string, accs []app.GenesisAccount,
	genFiles []string, numValidators int,
) error {

	appGenState := app.NewDefaultGenesisState()
	appGenState.Accounts = accs

	appGenStateJSON, err := codec.MarshalJSONIndent(cdc, appGenState)
	if err != nil {
		return err
	}

	genDoc := types.GenesisDoc{
		ChainID:    chainID,
		AppState:   appGenStateJSON,
		Validators: nil,
	}

	// generate empty genesis files for each validator and save
	for i := 0; i < numValidators; i++ {
		if err := genDoc.SaveAs(genFiles[i]); err != nil {
			return err
		}
	}

	return nil
}

func collectGenFiles(
	cdc *codec.Codec, config *cfg.Config, chainID string,
	monikers, nodeIDs []string, valPubKeys []crypto.PubKey,
	numValidators int, outDir string, nodes []Node, nodeDaemonHomeName string,
) error {

	var appState json.RawMessage
	genTime := tmtime.Now()

	for i := 0; i < numValidators; i++ {
		nodeDirName := nodes[i].Name
		nodeDir := filepath.Join(outDir, nodeDirName, nodeDaemonHomeName)
		gentxsDir := filepath.Join(outDir, "gentxs")
		moniker := monikers[i]
		config.Moniker = nodeDirName

		config.SetRoot(nodeDir)

		nodeID, valPubKey := nodeIDs[i], valPubKeys[i]
		initCfg := initConfig{
			ChainID:   chainID,
			GenTxsDir: gentxsDir,
			Name:      moniker,
			NodeID:    nodeID,
			ValPubKey: valPubKey,
		}

		genDoc, err := loadGenesisDoc(cdc, config.GenesisFile())
		if err != nil {
			return err
		}

		nodeAppState, err := genAppStateFromConfig(cdc, config, initCfg, genDoc)
		if err != nil {
			return err
		}

		if appState == nil {
			// set the canonical application state (they should not differ)
			appState = nodeAppState
		}

		genFile := config.GenesisFile()

		// overwrite each validator's genesis file to have a canonical genesis time
		err = ExportGenesisFileWithTime(genFile, chainID, nil, appState, genTime)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeFile(name string, dir string, contents []byte) error {
	writePath := filepath.Join(dir)
	file := filepath.Join(writePath, name)

	err := cmn.EnsureDir(writePath, 0700)
	if err != nil {
		return err
	}

	err = cmn.WriteFile(file, contents, 0600)
	if err != nil {
		return err
	}

	return nil
}

func calculateIP(ip string, i int) (string, error) {
	ipv4 := net.ParseIP(ip).To4()
	if ipv4 == nil {
		return "", fmt.Errorf("%v: non ipv4 address", ip)
	}

	for j := 0; j < i; j++ {
		ipv4[3]++
	}

	return ipv4.String(), nil
}
