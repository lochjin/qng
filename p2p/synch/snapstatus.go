package synch

import (
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/json"
	"github.com/Qitmeer/qng/meerdag"
	"github.com/libp2p/go-libp2p/core/peer"
	"sync"
)

type SnapStatus struct {
	locker *sync.RWMutex

	targetBlock *hash.Hash
	stateRoot   *hash.Hash
	peid        peer.ID

	syncPoint meerdag.IBlock

	evmCompleted bool
}

func (s *SnapStatus) IsInit() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.isInit()
}

func (s *SnapStatus) isInit() bool {
	return s.targetBlock != nil
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
		TargetBlock: s.targetBlock.String(),
		StateRoot:   s.stateRoot.String(),
	}
}

func (s *SnapStatus) ToString() string {
	s.locker.RLock()
	defer s.locker.RUnlock()
	if !s.isInit() {
		return "initializing"
	}
	return fmt.Sprintf("TargetBlock:%s, StateRoot:%s", s.targetBlock.String(), s.stateRoot.String())
}

func (s *SnapStatus) PeerID() peer.ID {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.peid
}

func (s *SnapStatus) Reset(peid peer.ID) {
	s.locker.Lock()
	defer s.locker.Unlock()

	if peid == s.peid {
		return
	}
	s.peid = peid
	s.targetBlock = nil
	s.stateRoot = nil
	log.Info("Reset snap status", "peid", peid.String())
}

func (s *SnapStatus) GetTargetBlock() *hash.Hash {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.targetBlock
}

func (s *SnapStatus) GetStateRoot() *hash.Hash {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.stateRoot
}

func (s *SnapStatus) SetTarget(targetBlock *hash.Hash, stateRoot *hash.Hash) {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.setTarget(targetBlock, stateRoot)
}

func (s *SnapStatus) setTarget(targetBlock *hash.Hash, stateRoot *hash.Hash) {
	if s.targetBlock == targetBlock && s.stateRoot == stateRoot {
		return
	} else if s.targetBlock != nil && s.targetBlock.IsEqual(targetBlock) &&
		s.stateRoot != nil && s.stateRoot.IsEqual(stateRoot) {
		return
	}
	s.targetBlock = targetBlock
	s.stateRoot = stateRoot

	log.Info("Snap status set new target", "targetBlock", targetBlock.String(), "stateRoot", stateRoot.String())
}

func (s *SnapStatus) GetTarget() (*hash.Hash, *hash.Hash) {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.getTarget()
}

func (s *SnapStatus) getTarget() (*hash.Hash, *hash.Hash) {
	return s.targetBlock, s.stateRoot
}

func (s *SnapStatus) GetSyncPoint() meerdag.IBlock {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.getSyncPoint()
}

func (s *SnapStatus) getSyncPoint() meerdag.IBlock {
	return s.syncPoint
}

func (s *SnapStatus) SetSyncPoint(point meerdag.IBlock) {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.setSyncPoint(point)
}

func (s *SnapStatus) setSyncPoint(point meerdag.IBlock) {
	s.syncPoint = point
}

func (s *SnapStatus) IsCompleted() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.isCompleted()
}

func (s *SnapStatus) isCompleted() bool {
	return s.isPointCompleted() && s.evmCompleted
}

func (s *SnapStatus) isPointCompleted() bool {
	if s.syncPoint == nil {
		return false
	}
	return s.syncPoint.GetHash().IsEqual(s.targetBlock)
}

func NewSnapStatus(peid peer.ID) *SnapStatus {
	return &SnapStatus{
		peid:         peid,
		locker:       &sync.RWMutex{},
		evmCompleted: false,
	}
}
