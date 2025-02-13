package meer

import (
	"fmt"
	mmeer "github.com/Qitmeer/qng/consensus/model/meer"
	"github.com/Qitmeer/qng/core/address"
	"github.com/Qitmeer/qng/core/blockchain/utxo"
	qtypes "github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/crypto/ecc"
	"github.com/Qitmeer/qng/engine/txscript"
	qparams "github.com/Qitmeer/qng/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

func makeHeader(cfg *ethconfig.Config, parent *types.Block, state *state.StateDB, timestamp int64, gaslimit uint64, difficulty *big.Int) *types.Header {
	ptt := int64(parent.Time())
	if timestamp <= ptt {
		timestamp = ptt + 1
	}

	header := &types.Header{
		Root:       state.IntermediateRoot(cfg.Genesis.Config.IsEIP158(parent.Number())),
		ParentHash: parent.Hash(),
		Coinbase:   parent.Coinbase(),
		Difficulty: difficulty,
		GasLimit:   gaslimit,
		Number:     new(big.Int).Add(parent.Number(), common.Big1),
		Time:       uint64(timestamp),
	}
	if cfg.Genesis.Config.IsLondon(header.Number) {
		header.BaseFee = eip1559.CalcBaseFee(cfg.Genesis.Config, parent.Header())
		if !cfg.Genesis.Config.IsLondon(parent.Number()) {
			parentGasLimit := parent.GasLimit() * cfg.Genesis.Config.ElasticityMultiplier()
			header.GasLimit = core.CalcGasLimit(parentGasLimit, parentGasLimit)
		}
	}
	if cfg.Genesis.Config.IsCancun(header.Number, header.Time) {
		var (
			parentExcessBlobGas uint64
			parentBlobGasUsed   uint64
		)
		if parent.ExcessBlobGas() != nil {
			parentExcessBlobGas = *parent.ExcessBlobGas()
			parentBlobGasUsed = *parent.BlobGasUsed()
		}
		excessBlobGas := eip4844.CalcExcessBlobGas(parentExcessBlobGas, parentBlobGasUsed)
		header.ExcessBlobGas = &excessBlobGas
		header.BlobGasUsed = new(uint64)
		header.ParentBeaconRoot = new(common.Hash)
	}
	return header
}

type fakeChainReader struct {
	config *params.ChainConfig
}

// Config returns the chain configuration.
func (cr *fakeChainReader) Config() *params.ChainConfig {
	return cr.config
}

func (cr *fakeChainReader) CurrentHeader() *types.Header                            { return nil }
func (cr *fakeChainReader) GetHeaderByNumber(number uint64) *types.Header           { return nil }
func (cr *fakeChainReader) GetHeaderByHash(hash common.Hash) *types.Header          { return nil }
func (cr *fakeChainReader) GetHeader(hash common.Hash, number uint64) *types.Header { return nil }
func (cr *fakeChainReader) GetBlock(hash common.Hash, number uint64) *types.Block   { return nil }
func (cr *fakeChainReader) GetTd(hash common.Hash, number uint64) *big.Int          { return nil }

func BuildEVMBlock(block *qtypes.SerializedBlock) (*mmeer.Block, error) {
	result := &mmeer.Block{Id: block.Hash(), Txs: []mmeer.Tx{}, Time: block.Block().Header.Timestamp}

	for idx, tx := range block.Transactions() {
		if idx == 0 {
			continue
		}
		if tx.IsDuplicate {
			continue
		}

		if qtypes.IsCrossChainExportTx(tx.Tx) {
			if tx.Object != nil {
				result.Txs = append(result.Txs, tx.Object.(*mmeer.ExportTx))
				continue
			}
			ctx, err := mmeer.NewExportTx(tx.Tx)
			if err != nil {
				return nil, err
			}
			result.Txs = append(result.Txs, ctx)
		} else if qtypes.IsCrossChainImportTx(tx.Tx) {
			ctx, err := mmeer.NewImportTx(tx.Tx)
			if err != nil {
				return nil, err
			}
			err = ctx.SetCoinbaseTx(block.Transactions()[0].Tx)
			if err != nil {
				return nil, err
			}
			result.Txs = append(result.Txs, ctx)
		} else if qtypes.IsCrossChainVMTx(tx.Tx) {
			if tx.Object != nil {
				vt := tx.Object.(*mmeer.VMTx)
				if vt.Coinbase.IsEqual(block.Transactions()[0].Hash()) {
					result.Txs = append(result.Txs, vt)
					continue
				}
			}
			ctx, err := mmeer.NewVMTx(tx.Tx, block.Transactions()[0].Tx)
			if err != nil {
				return nil, err
			}
			tx.Object = ctx
			result.Txs = append(result.Txs, ctx)
		}
	}
	return result, nil
}

func CheckUTXOPubkey(pubKey ecc.PublicKey, entry *utxo.UtxoEntry) ([]byte, error) {
	if len(entry.PkScript()) <= 0 {
		return nil, fmt.Errorf("PkScript is empty")
	}

	addrUn, err := address.NewSecpPubKeyAddress(pubKey.SerializeUncompressed(), qparams.ActiveNetParams.Params)
	if err != nil {
		return nil, err
	}

	addr, err := address.NewSecpPubKeyAddress(pubKey.SerializeCompressed(), qparams.ActiveNetParams.Params)
	if err != nil {
		return nil, err
	}

	scriptClass, pksAddrs, _, err := txscript.ExtractPkScriptAddrs(entry.PkScript(), qparams.ActiveNetParams.Params)
	if err != nil {
		return nil, err
	}
	if len(pksAddrs) != 1 {
		return nil, fmt.Errorf("PKScript num no support:%d", len(pksAddrs))
	}

	switch scriptClass {
	case txscript.PubKeyHashTy, txscript.PubkeyHashAltTy, txscript.PubKeyTy, txscript.PubkeyAltTy:
		if pksAddrs[0].Encode() == addr.PKHAddress().String() ||
			pksAddrs[0].Encode() == addrUn.PKHAddress().String() {
			return pubKey.SerializeUncompressed(), nil
		}
	default:
		return nil, fmt.Errorf("UTXO error about no support %s", scriptClass.String())
	}
	return nil, fmt.Errorf("UTXO error")
}
