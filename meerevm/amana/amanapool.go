/*
 * Copyright (c) 2017-2020 The qitmeer developers
 */

package amana

import (
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/consensus/model"
	"github.com/Qitmeer/qng/core/blockchain/opreturn"
	"github.com/Qitmeer/qng/params"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/miner"
	"sync"
	"sync/atomic"
	"time"

	qevent "github.com/Qitmeer/qng/core/event"
	qtypes "github.com/Qitmeer/qng/core/types"
	qcommon "github.com/Qitmeer/qng/meerevm/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

const (
	txChanSize        = 4096
	chainHeadChanSize = 10
	blockTag          = byte(1)
)

type resetTemplateMsg struct {
	reply chan struct{}
}

type snapshotTx struct {
	tx    *qtypes.Tx
	eHash common.Hash
}

type AmanaPool struct {
	wg      sync.WaitGroup
	quit    chan struct{}
	running int32

	consensus model.Consensus
	config    *miner.Config
	eth       *eth.Ethereum
	ethTxPool *legacypool.LegacyPool

	// Subscriptions
	txsCh        chan core.NewTxsEvent
	txsSub       event.Subscription
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription

	snapshotMu    sync.RWMutex // The lock used to protect the snapshots below
	snapshotBlock *types.Block
	snapshotQTxsM map[string]*snapshotTx
	snapshotTxsM  map[string]*snapshotTx

	resetTemplate chan *resetTemplateMsg

	qTxPool model.TxPool

	syncing atomic.Bool // The indicator whether the node is still syncing.
	dirty   atomic.Bool

	p2pSer model.P2PService
}

func (m *AmanaPool) Start() {
	if m.isRunning() {
		log.Info("Amana pool was started")
		return
	}
	atomic.StoreInt32(&m.running, 1)

	m.quit = make(chan struct{})
	m.wg.Add(1)
	go m.handler()
	m.subscribe()

	m.updateTemplate(true)
}

func (m *AmanaPool) Stop() {
	if !m.isRunning() {
		log.Info("Amana pool was stopped")
		return
	}
	atomic.StoreInt32(&m.running, 0)

	log.Info(fmt.Sprintf("Amana pool stopping"))
	close(m.quit)
	m.wg.Wait()

	log.Info(fmt.Sprintf("Amana pool stopped"))
}

func (m *AmanaPool) isRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}

func (m *AmanaPool) handler() {
	defer m.txsSub.Unsubscribe()
	defer m.chainHeadSub.Unsubscribe()
	defer m.wg.Done()

	stallTicker := time.NewTicker(params.ActiveNetParams.TargetTimePerBlock)
	defer stallTicker.Stop()

	for {
		select {
		case ev := <-m.txsCh:
			if m.qTxPool == nil {
				continue
			}
			if !m.qTxPool.IsSupportVMTx() {
				for _, tx := range ev.Txs {
					m.ethTxPool.RemoveTx(tx.Hash(), true)
				}
				continue
			}

			m.qTxPool.TriggerDirty()
			m.AnnounceNewTransactions(ev.Txs)
			m.dirty.Store(true)
		// System stopped
		case <-m.quit:
			return
		case <-m.txsSub.Err():
			return
		case <-m.chainHeadCh:
			m.updateTemplate(false)
		case <-m.chainHeadSub.Err():
			return
		case msg := <-m.resetTemplate:
			m.updateTemplate(true)
			select {
			case <-m.quit:
				return
			default:
			}
			msg.reply <- struct{}{}
		case <-stallTicker.C:
			m.handleStallSample()
		}
	}
}

func (m *AmanaPool) handleStallSample() {
	if m.p2pSer == nil {
		// The service is not ready yet, try again next time
		return
	}
	if m.syncing.Load() {
		return
	}
	if !m.eth.Synced() && m.p2pSer.IsCurrent() {
		m.eth.SetSynced()
	}
	if !m.dirty.Load() {
		return
	}
	m.snapshotMu.Lock()
	block := m.snapshotBlock
	m.snapshotMu.Unlock()
	if block != nil {
		if time.Since(time.Unix(int64(block.Time()), 0)) <= params.ActiveNetParams.TargetTimePerBlock {
			return
		}
	}

	go m.ResetTemplate()
}

func (m *AmanaPool) updateTemplate(force bool) error {
	if m.syncing.Load() {
		return nil
	}
	parentHash := m.eth.BlockChain().CurrentBlock().Hash()
	m.snapshotMu.Lock()
	if m.snapshotBlock != nil && !force {
		if parentHash == m.snapshotBlock.ParentHash() {
			m.snapshotMu.Unlock()
			return nil
		}
	}
	m.snapshotMu.Unlock()

	block, _, _ := m.eth.Miner().ForcePending()
	if block == nil {
		return nil
	}

	m.snapshotMu.Lock()
	defer m.snapshotMu.Unlock()

	m.snapshotBlock = block
	m.snapshotQTxsM = map[string]*snapshotTx{}
	m.snapshotTxsM = map[string]*snapshotTx{}
	txsNum := len(block.Transactions())
	if txsNum > 0 {
		for _, tx := range block.Transactions() {
			mtx := qcommon.ToQNGTx(tx, true)
			stx := &snapshotTx{tx: mtx, eHash: tx.Hash()}
			m.snapshotQTxsM[mtx.Hash().String()] = stx
			m.snapshotTxsM[tx.Hash().String()] = stx
		}
	}
	//
	log.Debug("amanapool update block template", "txs", txsNum)
	m.dirty.Store(false)
	return nil
}

func (m *AmanaPool) GetTxs() ([]*qtypes.Tx, []*hash.Hash, error) {
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()

	result := []*qtypes.Tx{}
	mtxhs := []*hash.Hash{}

	if m.snapshotBlock != nil && len(m.snapshotBlock.Transactions()) > 0 {
		for _, tx := range m.snapshotBlock.Transactions() {
			qtx, ok := m.snapshotTxsM[tx.Hash().String()]
			if !ok {
				continue
			}
			result = append(result, qtx.tx)
			mtxhs = append(mtxhs, qcommon.FromEVMHash(tx.Hash()))
		}
	}

	return result, mtxhs, nil
}

// all: contain txs in pending and queue
func (m *AmanaPool) HasTx(h *hash.Hash) bool {
	m.snapshotMu.RLock()
	_, ok := m.snapshotQTxsM[h.String()]
	m.snapshotMu.RUnlock()
	return ok
}

func (m *AmanaPool) GetSize() int64 {
	m.snapshotMu.RLock()
	defer m.snapshotMu.RUnlock()

	if m.snapshotBlock != nil {
		return int64(len(m.snapshotBlock.Transactions()))
	}
	return 0
}

func (m *AmanaPool) AddTx(tx *qtypes.Tx) (int64, error) {
	h := qcommon.ToEVMHash(&tx.Tx.TxIn[0].PreviousOut.Hash)
	if m.eth.TxPool().Has(h) {
		return 0, fmt.Errorf("already exists:%s (evm:%s)", tx.Hash().String(), h.String())
	}
	txb := qcommon.ToTxHex(tx.Tx.TxIn[0].SignScript)
	var txmb = &types.Transaction{}
	err := txmb.UnmarshalBinary(txb)
	if err != nil {
		return 0, err
	}

	errs := m.ethTxPool.AddRemotesSync(types.Transactions{txmb})
	if len(errs) > 0 && errs[0] != nil {
		return 0, errs[0]
	}

	log.Debug("Amana pool:add", "hash", tx.Hash(), "eHash", txmb.Hash())

	//
	cost := txmb.Cost()
	cost = cost.Sub(cost, txmb.Value())
	cost = cost.Div(cost, qcommon.Precision)
	return cost.Int64(), nil
}

func (m *AmanaPool) RemoveTx(tx *qtypes.Tx) error {
	if !m.isRunning() {
		return fmt.Errorf("Amana pool is not running")
	}
	if !opreturn.IsMeerEVMTx(tx.Tx) {
		return fmt.Errorf("%s is not %v", tx.Hash().String(), qtypes.TxTypeCrossChainVM)
	}
	h := qcommon.ToEVMHash(&tx.Tx.TxIn[0].PreviousOut.Hash)
	if m.eth.TxPool().Has(h) {
		m.ethTxPool.RemoveTx(h, false)
		log.Debug(fmt.Sprintf("Amana pool:remove tx %s(%s) from eth", tx.Hash(), h))
	}
	return nil
}

func (m *AmanaPool) ResetTemplate() error {
	if !m.isRunning() {
		err := errors.New("amana pool is not running")
		log.Warn(err.Error())
		return err
	}
	msg := &resetTemplateMsg{reply: make(chan struct{})}
	go func() {
		log.Debug("Try to reset Amana pool")
		m.resetTemplate <- msg
	}()
	select {
	case <-m.quit:
		return errors.New("AmanaPool is quit")
	case <-msg.reply:

	}
	return nil
}

func (m *AmanaPool) SetTxPool(tp model.TxPool) {
	m.qTxPool = tp
}

func (m *AmanaPool) SetP2P(ser model.P2PService) {
	m.p2pSer = ser
}

func (m *AmanaPool) subscribe() {
	ch := make(chan *qevent.Event)
	sub := m.consensus.Events().Subscribe(ch)
	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case ev := <-ch:
				if ev.Data != nil {
					switch value := ev.Data.(type) {
					case int:
						if value == qevent.DownloaderStart {
							m.syncing.Store(true)
						} else if value == qevent.DownloaderEnd {
							m.syncing.Store(false)
							m.ResetTemplate()
						}
					}
				}
				if ev.Ack != nil {
					ev.Ack <- struct{}{}
				}
			case <-m.quit:
				log.Info("Close AmanaPool Event Subscribe")
				return
			}
		}
	}()
}

func (m *AmanaPool) AnnounceNewTransactions(txs []*types.Transaction) error {
	txds := []*qtypes.TxDesc{}

	for _, tx := range txs {
		m.snapshotMu.RLock()
		qtx, ok := m.snapshotTxsM[tx.Hash().String()]
		m.snapshotMu.RUnlock()
		if !ok || qtx == nil {
			qtx = &snapshotTx{tx: qcommon.ToQNGTx(tx, true), eHash: tx.Hash()}
			if qtx.tx == nil {
				continue
			}
		}
		cost := tx.Cost()
		cost = cost.Sub(cost, tx.Value())
		cost = cost.Div(cost, qcommon.Precision)
		fee := cost.Int64()

		td := &qtypes.TxDesc{
			Tx:       qtx.tx,
			Added:    time.Now(),
			Height:   m.qTxPool.GetMainHeight(),
			Fee:      fee,
			FeePerKB: fee * 1000 / int64(qtx.tx.Tx.SerializeSize()),
		}

		txds = append(txds, td)
	}
	m.p2pSer.Notify().AnnounceNewTransactions(nil, txds, nil)
	return nil
}

func newAmanaPool(consensus model.Consensus, eth *eth.Ethereum) *AmanaPool {
	log.Info(fmt.Sprintf("New Amana pool"))
	m := &AmanaPool{}
	m.consensus = consensus
	m.eth = eth
	m.txsCh = make(chan core.NewTxsEvent, txChanSize)
	m.chainHeadCh = make(chan core.ChainHeadEvent, chainHeadChanSize)
	m.quit = make(chan struct{})
	m.resetTemplate = make(chan *resetTemplateMsg)
	m.chainHeadSub = eth.BlockChain().SubscribeChainHeadEvent(m.chainHeadCh)
	for _, sp := range eth.TxPool().Subpools() {
		ltp, ok := sp.(*legacypool.LegacyPool)
		if ok {
			m.ethTxPool = ltp
			break
		}
	}
	m.txsSub = m.eth.TxPool().SubscribeTransactions(m.txsCh, true)
	m.snapshotQTxsM = map[string]*snapshotTx{}
	m.snapshotTxsM = map[string]*snapshotTx{}
	return m
}
