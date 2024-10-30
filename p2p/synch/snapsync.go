package synch

import (
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/json"
	"sync"
)

type SnapStatus struct {
	locker *sync.RWMutex

	blockHash hash.Hash
	stateRoot hash.Hash
}

func (s SnapStatus) ToInfo() *json.SnapSyncInfo {
	s.locker.RLock()
	defer s.locker.RUnlock()
	return &json.SnapSyncInfo{
		TargetBlock: s.blockHash.String(),
		StateRoot:   s.stateRoot.String(),
	}
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
