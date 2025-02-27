package meer

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

// Config returns the chain configuration.
func (b *BlockChain) Config() *params.ChainConfig {
	return b.chain.Config().Eth.Genesis.Config
}

func (b *BlockChain) CurrentHeader() *types.Header {
	return b.chain.Ether().BlockChain().CurrentBlock()
}

func (b *BlockChain) GetHeaderByNumber(number uint64) *types.Header {
	return nil
}

func (b *BlockChain) GetHeaderByHash(hash common.Hash) *types.Header {
	return nil
}

func (b *BlockChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return nil
}

func (b *BlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return nil
}

func (b *BlockChain) GetTd(hash common.Hash, number uint64) *big.Int {
	return nil
}
