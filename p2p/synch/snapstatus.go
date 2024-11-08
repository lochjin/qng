package synch

import (
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/blockchain"
	"github.com/Qitmeer/qng/core/json"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/database/common"
	"github.com/Qitmeer/qng/p2p/peers"
	pb "github.com/Qitmeer/qng/p2p/proto/v1"
	"github.com/libp2p/go-libp2p/core/peer"
	"sync"
)

type SnapStatus struct {
	locker *sync.RWMutex

	targetBlock *hash.Hash
	stateRoot   *hash.Hash
	peid        peer.ID

	tstatus []*pb.TransferStatus
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

func (s *SnapStatus) GetTarget() (*hash.Hash, *hash.Hash) {
	s.locker.RLock()
	defer s.locker.RUnlock()

	return s.targetBlock, s.stateRoot
}

func (s *SnapStatus) isCompleted() bool {
	if s.tstatus == nil {
		return false
	}
	for i := 0; i < Max.Index(); i++ {
		if !isTransferCompleted(s.tstatus[i]) {
			return false
		}
	}
	return true
}

func (s *SnapStatus) processRsp(ssr *pb.SnapSyncRsp, p2p peers.P2P) ([]*blockchain.SnapData, error) {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.targetBlock = changePBHashToHash(ssr.TargetBlock)
	s.stateRoot = changePBHashToHash(ssr.StateRoot)
	// Legitimacy verification
	if len(ssr.Status) != Max.Index() {
		return nil, fmt.Errorf("SnapSyncRsp.TransferStatus len is %d, expect %d", len(ssr.Status), Max.Index())
	}
	if s.tstatus != nil {
		for i := 0; i < Max.Index(); i++ {
			if s.tstatus[i].Start+uint64(s.tstatus[i].Num) != ssr.Status[i].Start {
				return nil, fmt.Errorf("SnapSyncRsp.TransferStatus start is %d. (type:%d)", ssr.Status[i].Start, i)
			}
		}
	}
	s.tstatus = ssr.Status
	log.Trace("Snap-sync receive", "targetBlock", s.targetBlock.String(), "stateRoot", s.stateRoot.String())

	// process datas
	if len(ssr.Datas) > 0 {
		offset := 0
		for i := 0; i < Max.Index(); i++ {
			ts := s.tstatus[i]
			if isTransferCompleted(ts) {
				continue
			}
			if ts.Num <= 0 {
				continue
			}
			log.Debug("SnapSyncRsp", "type", TransferDataType(i).String(), "start", ts.Start, "num", ts.Num, "total", ts.Total)

			if TransferDataType(i) == Block {
				for j := 0; j < int(ts.Num); j++ {
					idx := offset + j
					block, err := types.NewBlockFromBytes(ssr.Datas[idx].Value)
					if err != nil {
						return nil, err
					}
					err = p2p.BlockChain().DB().PutBlock(block)
					if err != nil {
						return nil, err
					}
				}
			} else if TransferDataType(i) == UTXO {
				opts := []*common.UtxoOpt{}
				for j := 0; j < int(ts.Num); j++ {
					idx := offset + j
					opts = append(opts, &common.UtxoOpt{
						Add:  true,
						Key:  ssr.Datas[idx].Key,
						Data: ssr.Datas[idx].Value,
					})
				}
				err := p2p.BlockChain().DB().UpdateUtxo(opts)
				if err != nil {
					return nil, err
				}
			}

			offset += int(ts.Num)
			ts.Start = ts.Start + uint64(ts.Num)
			ts.Num = 0

			if isTransferCompleted(ts) {
				log.Info("SnapSyncRsp completed", "type", TransferDataType(i).String())
			}
			if offset >= len(ssr.Datas) {
				break
			}
		}
	}
	return nil, nil
}

func NewSnapStatus(peid peer.ID) *SnapStatus {
	return &SnapStatus{peid: peid}
}

type TransferDataType uint32

const (
	// block
	Block TransferDataType = iota

	// utxo
	UTXO

	// stxo
	STXO

	// DAG
	BlockDAG

	// main
	MainChain

	// max
	Max
)

func (t TransferDataType) Index() int {
	return int(t)
}

func (t TransferDataType) String() string {
	if s := transDataTypeStrings[t]; s != "" {
		return s
	}
	return fmt.Sprintf("Unknown TransferDataType (%d)", uint32(t))
}

var transDataTypeStrings = map[TransferDataType]string{
	Block:     "Block",
	UTXO:      "UTXO",
	STXO:      "STXO",
	BlockDAG:  "BlockDAG",
	MainChain: "MainChain",
	Max:       "Max",
}

func isTransferCompleted(ts *pb.TransferStatus) bool {
	if ts.Start != ts.Total || ts.Num != 0 {
		return false
	}
	return true
}
