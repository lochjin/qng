package blockchain

import (
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/blockchain/token"
	"github.com/Qitmeer/qng/core/blockchain/utxo"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/meerdag"
	"time"
)

type SnapData struct {
	block      *types.SerializedBlock
	stxos      []utxo.SpentTxOut
	dagBlock   meerdag.IBlock
	main       bool
	tokenState *token.TokenState
	prevTSHash *hash.Hash
}

func (s *SnapData) SetBlock(block *types.SerializedBlock) {
	s.block = block
}

func (s *SnapData) SetStxos(stxos []utxo.SpentTxOut) {
	s.stxos = stxos
}

func (s *SnapData) SetDAGBlock(block meerdag.IBlock) {
	s.dagBlock = block
}

func (s *SnapData) SetMain(main bool) {
	s.main = main
}

func (s *SnapData) SetTokenState(tokenState *token.TokenState) {
	s.tokenState = tokenState
}

func (s *SnapData) SetPrevTSHash(prevTSHash *hash.Hash) {
	s.prevTSHash = prevTSHash
}

func NewSnapData() *SnapData {
	return &SnapData{}
}

func (b *BlockChain) IsSnapSyncing() bool {
	return b.snapSyncing.Load()
}

func (b *BlockChain) SetSnapSyncing(val bool) {
	b.snapSyncing.Store(val)
}

func (b *BlockChain) BeginSnapSyncing() error {
	b.SetSnapSyncing(true)
	for {
		size := b.ProcessQueueSize()
		if size <= 0 {
			break
		}
		select {
		case <-time.After(time.Second):
			log.Trace("Try to snap syncing", "size", size)
		case <-b.quit:
			b.SetSnapSyncing(false)
			return fmt.Errorf("shutdown")
		}
	}
	return nil
}

func (b *BlockChain) EndSnapSyncing() {
	b.bd.RecalDiffAnticone()
	b.SetSnapSyncing(false)
}

func (b *BlockChain) ProcessBlockBySnap(sds []*SnapData) (meerdag.IBlock, error) {
	if len(sds) <= 0 {
		return nil, nil
	}
	var latest meerdag.IBlock
	b.ChainLock()
	defer b.ChainUnlock()
	txNum := int64(0)
	for _, sd := range sds {
		if !b.DB().HasBlock(sd.block.Hash()) {
			err := dbMaybeStoreBlock(b.DB(), sd.block)
			if err != nil {
				return latest, err
			}
		}
		node := NewBlockNode(sd.block)
		dblock, err := b.bd.AddDirectBlock(node, sd.dagBlock, sd.main)
		if err != nil {
			return latest, err
		}

		latest = dblock

		txNum += int64(len(sd.block.Transactions()))
	}
	b.progressLogger.LogBlockOrders(latest, int64(len(sds)), txNum)
	return latest, nil
}
