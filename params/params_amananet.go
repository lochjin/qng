// Copyright (c) 2017-2025 The qitmeer developers
// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2015-2017 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package params

import (
	"github.com/Qitmeer/qng/consensus/engine/config"
	"math/big"
	"time"

	"github.com/Qitmeer/qng/core/protocol"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/ledger"
	eparams "github.com/ethereum/go-ethereum/params"
)

const amanaTargetTimePerBlock = 3

var AmanaNetParams = Params{
	Name:           "amananet",
	Net:            protocol.AmanaNet,
	DefaultPort:    "48130",
	DefaultUDPPort: 48140,
	Bootstrap:      []string{},

	GenesisBlock:   types.NewBlock(&amanaNetGenesisBlock),
	GenesisHash:    &amanaNetGenesisHash,
	CoinbaseConfig: CoinbaseConfigs{},
	LedgerParams: ledger.LedgerParams{
		GenesisAmountUnit: 1000 * 1e8,
		MaxLockHeight:     10 * 365 * 5,
	},
	ConsensusConfig: &config.POAConfig{
		Period: amanaTargetTimePerBlock,
		Epoch:  100,
	},
	GenerateSupported:  true,
	MaximumBlockSizes:  []int{1000000, 1310720},
	MaxTxSize:          1000000,
	TargetTimePerBlock: time.Second * amanaTargetTimePerBlock,
	TargetTimespan:     time.Second * amanaTargetTimePerBlock * 100,

	// Subsidy parameters.
	BaseSubsidy:              50000000000,
	MulSubsidy:               100,
	DivSubsidy:               101,
	SubsidyReductionInterval: 128,
	WorkRewardProportion:     10,
	StakeRewardProportion:    0,
	BlockTaxProportion:       0,

	// Checkpoints ordered from oldest to newest.
	Checkpoints: nil,

	// Address encoding magics
	NetworkAddressPrefix: "A",
	Bech32HRPSegwit:      "a",
	PubKeyAddrID:         [2]byte{0x41, 0x6B}, // starts with Ak
	PubKeyHashAddrID:     [2]byte{0x41, 0x6D}, // starts with Am
	PKHEdwardsAddrID:     [2]byte{0x41, 0x65}, // starts with Ae
	PKHSchnorrAddrID:     [2]byte{0x41, 0x72}, // starts with Ar
	ScriptHashAddrID:     [2]byte{0x41, 0x53}, // starts with AS
	PrivateKeyID:         [2]byte{0x50, 0x61}, // starts with Pa

	// BIP32 hierarchical deterministic extended key magics
	HDPrivateKeyID: [4]byte{0x61, 0x70, 0x72, 0x76}, // starts with aprv
	HDPublicKeyID:  [4]byte{0x61, 0x70, 0x75, 0x62}, // starts with apub

	// BIP44 coin type used in the hierarchical deterministic path for
	// address generation.
	SLIP0044CoinType: 813,
	LegacyCoinType:   223,

	OrganizationPkScript: hexMustDecode("76a91429209320e66d96839785dd07e643a7f1592edc5a88ac"),

	// Because it's only for testing, it comes from testwallet.go
	TokenAdminPkScript: hexMustDecode("00000000c96d6d76a914785bfbf4ecad8b72f2582be83616c5d364a3244288ac"),

	CoinbaseMaturity: 16,

	MeerConfig: &eparams.ChainConfig{
		ChainID:                       eparams.AmanaChainConfig.ChainID,
		HomesteadBlock:                big.NewInt(0),
		DAOForkBlock:                  big.NewInt(0),
		DAOForkSupport:                false,
		EIP150Block:                   big.NewInt(0),
		EIP155Block:                   big.NewInt(0),
		EIP158Block:                   big.NewInt(0),
		ByzantiumBlock:                big.NewInt(0),
		ConstantinopleBlock:           big.NewInt(0),
		PetersburgBlock:               big.NewInt(0),
		IstanbulBlock:                 big.NewInt(0),
		MuirGlacierBlock:              big.NewInt(0),
		BerlinBlock:                   big.NewInt(0),
		LondonBlock:                   big.NewInt(0),
		ArrowGlacierBlock:             big.NewInt(0),
		GrayGlacierBlock:              big.NewInt(0),
		TerminalTotalDifficulty:       big.NewInt(0),
		TerminalTotalDifficultyPassed: true,
		ShanghaiTime:                  newUint64(0),
		CancunTime:                    newUint64(0),
	},
	EmptyBlockForkBlock: big.NewInt(0),
	GasLimitForkBlock:   big.NewInt(0),
	CancunForkBlock:     big.NewInt(0),
	MeerChangeForkBlock: big.NewInt(0),
}
