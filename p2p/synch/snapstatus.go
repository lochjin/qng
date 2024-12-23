package synch

import (
	"bytes"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/json"
	s "github.com/Qitmeer/qng/core/serialization"
	"github.com/Qitmeer/qng/meerdag"
	"github.com/libp2p/go-libp2p/core/peer"
	"sync"
)

type SnapStatus struct {
	locker *sync.RWMutex

	targetBlock  *hash.Hash
	stateRoot    *hash.Hash
	peid         peer.ID
	evmCompleted bool

	PointID   uint64
	syncPoint meerdag.IBlock
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
			PeerID:      "initializing",
			TargetBlock: "initializing",
			StateRoot:   "initializing",
		}
	}
	info := &json.SnapSyncInfo{
		PeerID:      s.peid.String(),
		TargetBlock: s.targetBlock.String(),
		StateRoot:   s.stateRoot.String(),
	}
	if s.syncPoint != nil {
		info.Point = s.syncPoint.GetHash().String()
		info.PointOrder = uint64(s.syncPoint.GetOrder())
	}
	info.EVMCompleted = s.evmCompleted
	info.Completed = s.isCompleted()
	return info
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

func (s *SnapStatus) ResetPeer(peid peer.ID) {
	s.locker.Lock()
	defer s.locker.Unlock()

	if peid == s.peid {
		log.Info("No need reset peer", "peid", peid.String())
		return
	}
	log.Info("Reset peer", "peid", s.peid.String(), "new peid", peid.String())
	s.peid = peid
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
	s.PointID = uint64(point.GetID())
}

func (s *SnapStatus) IsCompleted() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.isCompleted()
}

func (s *SnapStatus) isCompleted() bool {
	return s.isPointCompleted() && s.evmCompleted
}

func (s *SnapStatus) IsPointCompleted() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	if s.syncPoint == nil {
		return false
	}
	return s.syncPoint.GetHash().IsEqual(s.targetBlock)
}

func (s *SnapStatus) isPointCompleted() bool {
	if s.syncPoint == nil {
		return false
	}
	return s.syncPoint.GetHash().IsEqual(s.targetBlock)
}

func (s *SnapStatus) IsEVMCompleted() bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.evmCompleted
}

func (s *SnapStatus) CompleteEVM() {
	s.locker.Lock()
	defer s.locker.Unlock()
	s.evmCompleted = true
}

func (ss *SnapStatus) Bytes() ([]byte, error) {
	w := &bytes.Buffer{}
	err := s.WriteElements(w, ss.targetBlock)
	if err != nil {
		return nil, err
	}
	err = s.WriteElements(w, ss.stateRoot)
	if err != nil {
		return nil, err
	}

	peidBytes := []byte(ss.peid)
	err = s.WriteElements(w, uint32(len(peidBytes)))
	if err != nil {
		return nil, err
	}
	_, err = w.Write(peidBytes)
	if err != nil {
		return nil, err
	}
	err = s.WriteElements(w, ss.PointID)
	if err != nil {
		return nil, err
	}
	err = s.WriteElements(w, ss.evmCompleted)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (ss *SnapStatus) Decode(data []byte) error {
	r := bytes.NewReader(data)
	var targetBlock hash.Hash
	err := s.ReadElements(r, &targetBlock)
	if err != nil {
		return err
	}
	ss.targetBlock = &targetBlock

	var stateRoot hash.Hash
	err = s.ReadElements(r, &stateRoot)
	if err != nil {
		return err
	}
	ss.stateRoot = &stateRoot

	var peidSize uint32
	err = s.ReadElements(r, &peidSize)
	if err != nil {
		return err
	}
	peid := make([]byte, peidSize)
	_, err = r.Read(peid)
	if err != nil {
		return err
	}
	ss.peid, err = peer.IDFromBytes(peid)
	if err != nil {
		return err
	}
	err = s.ReadElements(r, &ss.PointID)
	if err != nil {
		return err
	}
	err = s.ReadElements(r, &ss.evmCompleted)
	if err != nil {
		return err
	}
	return nil
}

func NewSnapStatus(peid peer.ID) *SnapStatus {
	return &SnapStatus{
		peid:         peid,
		locker:       &sync.RWMutex{},
		evmCompleted: false,
	}
}

func NewSnapStatusFromBytes(data []byte) *SnapStatus {
	ss := &SnapStatus{
		locker: &sync.RWMutex{},
	}
	err := ss.Decode(data)
	if err != nil {
		log.Error(err.Error())
		return nil
	}
	return ss
}
