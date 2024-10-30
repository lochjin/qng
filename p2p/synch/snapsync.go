package synch

import (
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/blockchain"
	"github.com/Qitmeer/qng/core/event"
	"github.com/Qitmeer/qng/core/json"
	"github.com/Qitmeer/qng/p2p/peers"
	"sync"
	"time"
)

type SnapStatus struct {
	locker *sync.RWMutex

	blockHash *hash.Hash
	stateRoot *hash.Hash
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

func (ps *PeerSync) startSnapSync(bestPeer *peers.Peer, best *blockchain.BestState, startTime time.Time) {
	if !ps.IsSnapSync() {
		ps.snapStatus = NewSnapStatus()
	}
	log.Info("Snap syncing", "cur", best.GraphState.String(), "target", ps.snapStatus.ToString(), "peer", bestPeer.GetID().String(), "processID", ps.getProcessID())
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderStart))
	add := 0

	if add > 0 {
		if !ps.IsInterrupt() {
			if ps.IsCurrent() {
				log.Info("You're up to date now.")
			}
		}
	}
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderEnd))

	log.Info("Snap-sync has ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "processID", ps.getProcessID())
}
