package miner

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Qitmeer/qng/common/hash"
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

	discrete      bool
	discreteNum   int
	discreteBlock chan *hash.Hash

	updateHashes      chan uint64
	queryHashesPerSec chan float64
	workWg            sync.WaitGroup
	updateNumWorks    chan struct{}
	numWorks          uint32
	hasNewWork        atomic.Bool

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

	log.Info("Start CPU Worker...")

	w.miner.updateBlockTemplate(false)

	w.wg.Add(1)
	go w.workController()

	return nil
}

func (w *PoAWorker) Stop() {
	if atomic.AddInt32(&w.shutdown, 1) != 1 {
		log.Warn(fmt.Sprintf("CPU Worker is already in the process of shutting down"))
		return
	}
	log.Info("Stop CPU Worker...")

	close(w.quit)
	w.wg.Wait()
}

func (w *PoAWorker) workController() {
	// launchWorkers groups common code to launch a specified number of
	// workers for generating blocks.
	var runningWorks []chan struct{}
	launchWorkers := func(numWorkers uint32) {
		for i := uint32(0); i < numWorkers; i++ {
			quit := make(chan struct{})
			runningWorks = append(runningWorks, quit)

			w.workWg.Add(1)
			go w.generateBlocks()
		}
	}

	// Launch the current number of workers by default.
	runningWorks = make([]chan struct{}, 0, w.numWorks)
	launchWorkers(w.numWorks)

out:
	for {
		select {
		// Update the number of running workers.
		case <-w.updateNumWorks:
			// No change.
			numRunning := uint32(len(runningWorks))
			if w.numWorks == numRunning {
				continue
			}

			// Add new workers.
			if w.numWorks > numRunning {
				launchWorkers(w.numWorks - numRunning)
				continue
			}

			// Signal the most recently created goroutines to exit.
			for i := numRunning - 1; i >= w.numWorks; i-- {
				close(runningWorks[i])
				runningWorks[i] = nil
				runningWorks = runningWorks[:i]
			}

		case <-w.quit:
			for _, quit := range runningWorks {
				close(quit)
			}
			break out
		}
	}

	// Wait until all workers shut down to stop the speed monitor since
	// they rely on being able to send updates to it.
	w.workWg.Wait()
	w.wg.Done()
}

func (w *PoAWorker) IsRunning() bool {
	return atomic.LoadInt32(&w.started) != 0
}

func (w *PoAWorker) Update() {
	if atomic.LoadInt32(&w.shutdown) != 0 {
		return
	}
	w.hasNewWork.Store(true)
}

func (w *PoAWorker) generateDiscrete(num int, block chan *hash.Hash) bool {
	if atomic.LoadInt32(&w.shutdown) != 0 {
		return false
	}
	w.Lock()
	defer w.Unlock()
	if w.discrete && w.discreteNum > 0 {
		log.Info(fmt.Sprintf("It already exists generate blocks by discrete: left=%d", w.discreteNum))
		return false
	}
	w.discrete = true
	w.discreteNum = num
	w.discreteBlock = block
	return true
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
		if w.discrete && w.discreteNum <= 0 || !w.discrete && !w.hasNewWork.Load() {
			time.Sleep(time.Second)
			continue
		}
		start := time.Now()
		if err := w.miner.CanMining(); err != nil {
			log.Warn(err.Error())
			time.Sleep(time.Second)
			continue
		}
		if params.ActiveNetParams.Params.IsDevelopDiff() {
			w.miner.updateBlockTemplate(true)
			if !w.miner.cfg.GenerateNoDevGap {
				time.Sleep(params.ActiveNetParams.Params.TargetTimePerBlock)
			}
		}

		sb := w.solveBlock()
		if sb != nil {
			w.hasNewWork.Store(false)

			block := types.NewBlock(sb)
			startSB := time.Now()
			info, err := w.miner.submitBlock(block)
			if err != nil {
				if !strings.Contains(err.Error(), "expired") {
					log.Error(fmt.Sprintf("Failed to submit new block:%s ,%v", block.Hash().String(), err))
					w.cleanDiscrete()
				} else {
					log.Warn(fmt.Sprintf("Failed to submit new block:%s ,%v", block.Hash().String(), err))
				}
				if !w.discrete {
					w.hasNewWork.Store(true)
				}
				continue
			} else {
				w.miner.StatsSubmit(startSB, block.Block().BlockHash().String(), len(block.Block().Transactions)-1)
			}
			log.Info(fmt.Sprintf("%v", info), "cost", time.Since(start).String(), "txs", len(block.Transactions()))

			w.Lock()
			if w.discrete && w.discreteNum > 0 {
				w.discreteNum--
				if w.discreteBlock != nil {
					w.discreteBlock <- block.Hash()
				}
				if w.discreteNum <= 0 {
					w.cleanDiscrete()
				}
			}
			w.Unlock()
		} else {
			w.Lock()
			w.cleanDiscrete()
			w.Unlock()
		}
	}

	w.workWg.Done()
	log.Info(fmt.Sprintf("Generate blocks worker done:%s", w.GetType()))
}

func (w *PoAWorker) cleanDiscrete() {
	if w.discrete {
		w.discreteNum = 0
		if w.discreteBlock != nil {
			close(w.discreteBlock)
			w.discreteBlock = nil
		}
	}
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
	hashesCompleted := uint64(0)
	// TODO, decided if need extra nonce for coinbase-tx
	// Note that the entire extra nonce range is iterated and the offset is
	// added relying on the fact that overflow will wrap around 0 as
	// provided by the Go spec.
	// for extraNonce := uint64(0); extraNonce < maxExtraNonce; extraNonce++ {

	// Update the extra nonce in the block template with the
	// new value by regenerating the coinbase script and
	// setting the merkle root to the new value.
	// TODO, decided if need extra nonce for coinbase-tx
	// updateExtraNonce(msgBlock, extraNonce+enOffset)

	// Update the extra nonce in the block template header with the
	// new value.
	// binary.LittleEndian.PutUint64(header.ExtraData[:], extraNonce+enOffset)

	// Search through the entire nonce range for a solution while
	// periodically checking for early quit and stale block
	// conditions along with updates to the speed monitor.
	for i := uint64(0); i <= maxNonce; i++ {
		select {
		case <-w.quit:
			return nil

		case <-ticker.C:
			w.updateHashes <- hashesCompleted
			hashesCompleted = 0

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
		hashesCompleted += 2
		header.Engine = instance
		if params.ActiveNetParams.Params.IsDevelopDiff() {
			return block
		}
		if header.PoW().FindSolver(header.Digest(), header.BlockHash(), header.Difficulty) {
			w.updateHashes <- hashesCompleted
			return block
		}
		// Each hash is actually a double hash (tow hashes), so
	}
	return nil
}

func NewPoAWorker(miner *Miner) *PoAWorker {
	w := PoAWorker{
		quit:              make(chan struct{}),
		discrete:          false,
		discreteNum:       0,
		miner:             miner,
		updateHashes:      make(chan uint64),
		queryHashesPerSec: make(chan float64),
		updateNumWorks:    make(chan struct{}),
		numWorks:          defaultNumWorkers,
	}

	return &w
}
