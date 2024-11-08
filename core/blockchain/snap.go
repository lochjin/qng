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
	stxo       []utxo.SpentTxOut
	dagBlock   meerdag.IBlock
	main       bool
	tokenState token.TokenState
	prevTSHash *hash.Hash
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
	b.SetSnapSyncing(false)
}

func (b *BlockChain) ProcessBlockBySnap(sds []*SnapData) error {

	return nil
}
