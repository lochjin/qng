package amana2

import (
	"encoding/json"
	"fmt"
	"github.com/Qitmeer/qng/config"
	mconsensus "github.com/Qitmeer/qng/meerevm/amana2/consensus"
	mcommon "github.com/Qitmeer/qng/meerevm/common"
	"github.com/Qitmeer/qng/meerevm/eth"
	"github.com/Qitmeer/qng/p2p/common"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"os"
	"path/filepath"
	"time"
)

var (
	// ClientIdentifier is a hard coded identifier to report into the network.
	ClientIdentifier = mconsensus.Identifier

	exclusionFlags = utils.NetworkFlags
)

func MakeConfig(cfg *config.Config) (*eth.Config, error) {
	datadir := cfg.DataDir
	genesis, err := Genesis(cfg.AmanaGenesis)
	if err != nil {
		return nil, err
	}

	econfig := ethconfig.Defaults

	econfig.NetworkId = genesis.Config.ChainID.Uint64()
	econfig.Genesis = genesis
	econfig.NoPruning = false
	econfig.SkipBcVersionCheck = false
	econfig.ConsensusEngine = createConsensusEngine

	if cfg.EVMTrieTimeout > 0 {
		econfig.TrieTimeout = time.Second * time.Duration(cfg.EVMTrieTimeout)
	}
	if len(cfg.StateScheme) > 0 {
		econfig.StateScheme = cfg.StateScheme
	}

	nodeConf := node.DefaultConfig

	nodeConf.DataDir = datadir
	nodeConf.Name = ClientIdentifier
	nodeConf.Version = params.VersionWithMeta
	nodeConf.HTTPModules = append(nodeConf.HTTPModules, "eth")
	nodeConf.WSModules = append(nodeConf.WSModules, "eth")
	if !cfg.DisableRPC {
		nodeConf.IPCPath = ClientIdentifier + ".ipc"
	}

	if len(datadir) > 0 {
		nodeConf.KeyStoreDir = filepath.Join(datadir, "keystore")
	}

	nodeConf.HTTPPort, nodeConf.WSPort, nodeConf.AuthPort = getDefaultPort()
	nodeConf.P2P.ListenAddr = ""
	nodeConf.P2P.NoDial = true
	nodeConf.P2P.NoDiscovery = true
	nodeConf.P2P.DiscoveryV4 = false
	nodeConf.P2P.DiscoveryV5 = false
	nodeConf.P2P.NAT = nil

	pk, err := common.PrivateKey(cfg.DataDir, "", 0600)
	if err != nil {
		return nil, err
	}
	nodeConf.P2P.PrivateKey, err = common.ToECDSAPrivKey(pk)
	if err != nil {
		return nil, err
	}

	return &eth.Config{
		Eth:     econfig,
		Node:    nodeConf,
		Metrics: metrics.DefaultConfig,
	}, nil
}

func MakeParams(cfg *config.Config) (*eth.Config, []string, error) {
	ecfg, err := MakeConfig(cfg)
	if err != nil {
		return ecfg, nil, err
	}
	args, err := mcommon.ProcessEnv(cfg.EVMEnv, ecfg.Node.Name, exclusionFlags)
	return ecfg, args, err
}

func getDefaultPort() (int, int, int) {
	return 8525, 8526, 8527
}

func createConsensusEngine(config *params.ChainConfig, db ethdb.Database) (consensus.Engine, error) {
	return mconsensus.New(), nil
}

func Genesis(genesisFile string) (*core.Genesis, error) {
	if len(genesisFile) > 0 {
		file, err := os.Open(genesisFile)
		if err != nil {
			return nil, fmt.Errorf("Failed to read genesis file: %v", err)
		}
		defer file.Close()

		genesis := new(core.Genesis)
		if err := json.NewDecoder(file).Decode(genesis); err != nil {
			return nil, fmt.Errorf("invalid genesis file: %v", err)
		}
		fileName := filepath.Base(genesisFile)
		extension := filepath.Ext(genesisFile)
		fileName = fileName[:len(fileName)-len(extension)]
		err = params.AddMeerChainConfig(&params.MeerChainConfig{ChainID: genesis.Config.ChainID, Name: fileName, Type: params.Amana})
		if err != nil {
			return nil, err
		}
		return genesis, nil
	}
	return AmanaGenesis(), nil
}
