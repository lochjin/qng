package synch

import (
	"context"
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/event"
	"github.com/Qitmeer/qng/core/json"
	"github.com/Qitmeer/qng/p2p/common"
	"github.com/Qitmeer/qng/p2p/peers"
	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"sync"
	"time"
)

type SnapStatus struct {
	locker *sync.RWMutex

	blockHash *hash.Hash
	stateRoot *hash.Hash

	peid peer.ID
}

func (s *SnapStatus) IsInit() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.isInit()
}

func (s *SnapStatus) isInit() bool {
	return s.blockHash != nil
}

func (s *SnapStatus) ToInfo() *json.SnapSyncInfo {
	s.locker.RLock()
	defer s.locker.RUnlock()
	if !s.isInit() {
		return &json.SnapSyncInfo{
			TargetBlock: "initializing",
			StateRoot:   "initializing",
		}
	}
	return &json.SnapSyncInfo{
		TargetBlock: s.blockHash.String(),
		StateRoot:   s.stateRoot.String(),
	}
}

func (s *SnapStatus) ToString() string {
	s.locker.RLock()
	defer s.locker.RUnlock()
	if !s.isInit() {
		return "initializing"
	}
	return fmt.Sprintf("TargetBlock:%s, StateRoot:%s", s.blockHash.String(), s.stateRoot.String())
}

func (s *SnapStatus) PeerID() peer.ID {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.peid
}

func NewSnapStatus() *SnapStatus {
	return &SnapStatus{}
}

func (ps *PeerSync) IsSnapSync() bool {
	return ps.snapStatus != nil
}

func (ps *PeerSync) GetSnapSyncInfo() *json.SnapSyncInfo {
	if !ps.IsSnapSync() {
		return nil
	}
	return ps.snapStatus.ToInfo()
}

func (ps *PeerSync) startSnapSync() bool {
	best := ps.Chain().BestSnapshot()
	bestPeer := ps.getBestPeer(true)
	if bestPeer == nil {
		return false
	}
	gs := bestPeer.GraphState()
	if gs.GetTotal() < best.GraphState.GetTotal()+MaxBlockLocatorsPerMsg {
		return false
	}
	// Start syncing from the best peer if one was selected.
	ps.processID++
	ps.processwg.Add(1)
	ps.SetSyncPeer(bestPeer)

cleanup:
	for {
		select {
		case <-ps.interrupt:
		default:
			break cleanup
		}
	}
	ps.interrupt = make(chan struct{})
	startTime := time.Now()
	ps.lastSync = startTime

	ps.snapStatus = NewSnapStatus()

	log.Info("Snap syncing", "cur", best.GraphState.String(), "target", ps.snapStatus.ToString(), "peer", bestPeer.GetID().String(), "processID", ps.getProcessID())
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderStart))
	// ------

	// ------
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderEnd))
	log.Info("Snap-sync has ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "processID", ps.getProcessID())

	ps.SetSyncPeer(nil)
	ps.processwg.Done()

	return true
}

func (s *Sync) sendSnapSyncRequest(stream network.Stream, pe *peers.Peer) *common.Error {

	return nil
}

func (s *Sync) snapSyncHandler(ctx context.Context, msg interface{}, stream libp2pcore.Stream, pe *peers.Peer) *common.Error {
	return nil
}
