package miner

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Qitmeer/qng/common/roughtime"
	"github.com/Qitmeer/qng/consensus/engine/pow"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/params"
)

type PoAWorker struct {
	started  int32
	shutdown int32
	wg       sync.WaitGroup
	quit     chan struct{}

	miner *Miner
	sync.Mutex
}

func (w *PoAWorker) GetType() string {
	return PoAWorkerType
}

func (w *PoAWorker) Start() error {
	err := w.miner.initCoinbase()
	if err != nil {
		log.Error(err.Error())
		return err
	}
	// Already started?
	if atomic.AddInt32(&w.started, 1) != 1 {
		return nil
	}

	log.Info("Start PoA Worker...")

	w.miner.updateBlockTemplate(false)

	w.wg.Add(1)
	go w.generateBlocks()

	return nil
}

func (w *PoAWorker) Stop() {
	if atomic.AddInt32(&w.shutdown, 1) != 1 {
		log.Warn(fmt.Sprintf("PoA Worker is already in the process of shutting down"))
		return
	}
	log.Info("Stop PoA Worker...")

	close(w.quit)
	w.wg.Wait()
}

func (w *PoAWorker) IsRunning() bool {
	return atomic.LoadInt32(&w.started) != 0
}

func (w *PoAWorker) Update() {
}

func (w *PoAWorker) generateBlocks() {
	log.Info(fmt.Sprintf("Starting generate blocks worker:%s", w.GetType()))
out:
	for {
		// Quit when the miner is stopped.
		select {
		case <-w.quit:
			break out
		default:
			// Non-blocking select to fall through
		}
		start := time.Now()
		if err := w.miner.CanMining(); err != nil {
			log.Warn(err.Error())
			time.Sleep(time.Second)
			continue
		}

		sb := w.solveBlock()
		if sb != nil {
			block := types.NewBlock(sb)
			startSB := time.Now()
			info, err := w.miner.submitBlock(block)
			if err != nil {
				if !strings.Contains(err.Error(), "expired") {
					log.Error(fmt.Sprintf("Failed to submit new block:%s ,%v", block.Hash().String(), err))
				} else {
					log.Warn(fmt.Sprintf("Failed to submit new block:%s ,%v", block.Hash().String(), err))
				}
				continue
			} else {
				w.miner.StatsSubmit(startSB, block.Block().BlockHash().String(), len(block.Block().Transactions)-1)
			}
			log.Info(fmt.Sprintf("%v", info), "cost", time.Since(start).String(), "txs", len(block.Transactions()))

		}
	}

	w.wg.Done()
	log.Info(fmt.Sprintf("Generate blocks worker done:%s", w.GetType()))
}

func (w *PoAWorker) solveBlock() *types.Block {
	if w.miner.template == nil {
		return nil
	}
	// Start a ticker which is used to signal checks for stale work and
	// updates to the speed monitor.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Create a couple of convenience variables.
	block, err := w.miner.template.Block.Clone()
	if err != nil {
		log.Error(err.Error())
		return nil
	}
	header := &block.Header

	// Initial state.
	lastGenerated := roughtime.Now()

	for i := uint64(0); i <= maxNonce; i++ {
		select {
		case <-w.quit:
			return nil

		case <-ticker.C:

			// The current block is stale if the memory pool
			// has been updated since the block template was
			// generated and it has been at least 3 seconds,
			// or if it's been one minute.
			if roughtime.Now().After(lastGenerated.Add(gbtRegenerateSeconds * time.Second)) {
				return nil
			}
		default:
			// Non-blocking select to fall through
		}
		instance := pow.GetInstance(w.miner.powType, 0, []byte{})
		instance.SetNonce(uint64(i))
		instance.SetMainHeight(pow.MainHeight(w.miner.template.Height))
		instance.SetParams(params.ActiveNetParams.Params.ToPoWConfig().PowConfig)
		header.Engine = instance
		if params.ActiveNetParams.Params.IsDevelopDiff() {
			return block
		}
		if header.PoW().FindSolver(header.Digest(), header.BlockHash(), header.Difficulty) {
			return block
		}
		// Each hash is actually a double hash (tow hashes), so
	}
	return nil
}

func NewPoAWorker(miner *Miner) *PoAWorker {
	w := PoAWorker{
		quit:  make(chan struct{}),
		miner: miner,
	}

	return &w
}
