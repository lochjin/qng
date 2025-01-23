/*
 * Copyright (c) 2017-2020 The qitmeer developers
 */

package synch

import (
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/blockchain"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/meerdag"
	"github.com/Qitmeer/qng/p2p/peers"
	pb "github.com/Qitmeer/qng/p2p/proto/v1"
	"time"
)

func (ps *PeerSync) syncBlocks(pe *peers.Peer) int {
	getSyncPeer := func() *peers.Peer {
		start := time.Now()
		var pee *peers.Peer
		for pee == nil {
			pee = ps.getBestPeer(LongSyncMode, nil)
			if pee == nil {
				log.Debug("Try to get sync peer", "cost", time.Since(start).String())
				time.Sleep(time.Second * 5)
			}
			if !ps.IsRunning() {
				return nil
			}
		}
		return pee
	}
	behaviorFlags := blockchain.BFP2PAdd
	add := 0
	init := true
	zeroGS := &pb.GraphState{Tips: []*pb.Hash{}}
	var gs *pb.GraphState
	var mainLocator []*hash.Hash
	var latest *hash.Hash
	for pe != nil {
		if ps.IsInterrupt() {
			break
		}
		point := pe.SyncPoint()
		if init {
			gs = ps.sy.getGraphState()
			mainLocator, _ = ps.dagSync.GetMainLocator(point, false)
		} else {
			gs = zeroGS
			mainLocator = []*hash.Hash{point, latest}
		}

		req := &pb.SyncBlocksReq{Init: init, GraphState: gs, MainLocator: changeHashsToPBHashs(mainLocator)}
		ret, err := pe.Request(SyncBlocksReqMsg, req)
		if err != nil {
			log.Error(err.Error())
			if !ps.isValidSyncPeer(pe, LongSyncMode) {
				pe = getSyncPeer()
			} else {
				break
			}
			init = true
			continue
		}
		if ret != nil {
			init = false
			rsp := ret.(*pb.SyncBlocksRsp)
			log.Debug("Receive blocks", "count", len(rsp.Blocks), "point", changePBHashToHash(rsp.SyncPoint).String())
			if isZeroPBHash(rsp.SyncPoint) {
				break
			}
			if len(rsp.Blocks) > 0 {
				for _, bs := range rsp.Blocks {
					block, err := types.NewBlockFromBytes(bs.Data)
					if err != nil {
						log.Warn("NewBlockFromBytes", "err", err, "processID", ps.getProcessID())
						init = true
						break
					}
					if ps.sy.p2p.BlockChain().BlockDAG().HasBlock(block.Hash()) {
						continue
					}
					pid := pe.GetID()
					_, IsOrphan, err := ps.sy.p2p.BlockChain().ProcessBlock(block, behaviorFlags, &pid)
					if err != nil {
						log.Error("Failed to process block", "hash", block.Hash(), "err", err, "processID", ps.getProcessID())
						init = true
						break
					}
					if IsOrphan {
						init = true
						break
					}
					add++
					latest = block.Hash()
				}
			} else {
				break
			}
			if rsp.Complete {
				break
			}
		}
	}
	return add
}

func (s *Sync) syncBlocksHandler(id uint64, msg interface{}, pe *peers.Peer) error {
	m, ok := msg.(*pb.SyncBlocksReq)
	if !ok {
		return fmt.Errorf("message is not type *SyncBlocksReq")
	}
	rsp := &pb.SyncBlocksRsp{Blocks: []*pb.BytesData{}}
	var blocks []*hash.Hash
	var point *hash.Hash
	complete := false
	if m.Init {
		pe.UpdateGraphState(m.GraphState)
		blocks, complete, point = s.PeerSync().dagSync.CalcSyncBlocks(changePBHashsToHashs(m.MainLocator), meerdag.SubDAGMode, MaxBlockLocatorsPerMsg)
	} else {
		blocks, complete, point = s.PeerSync().dagSync.CalcSyncBlocks(changePBHashsToHashs(m.MainLocator), meerdag.ContinueMode, MaxBlockLocatorsPerMsg)
	}
	if point == nil {
		rsp.Complete = true
		rsp.SyncPoint = &pb.Hash{Hash: hash.ZeroHash.Bytes()}
	} else {
		rsp.Complete = complete
		pe.UpdateSyncPoint(point)
		rsp.SyncPoint = &pb.Hash{Hash: point.Bytes()}
		if len(blocks) > 0 {
			for _, bh := range blocks {
				bs, err := s.p2p.BlockChain().FetchBlockBytesByHash(bh)
				if err != nil {
					return err
				}
				pbbd := &pb.BytesData{Data: bs}
				if uint64(rsp.SizeSSZ()+pbbd.SizeSSZ()+BLOCKDATA_SSZ_HEAD_SIZE) >= s.p2p.Encoding().GetMaxChunkSize() {
					break
				}
				rsp.Blocks = append(rsp.Blocks, pbbd)
			}
		} else {
			rsp.Complete = true
		}
	}
	return pe.Respond(SyncBlocksRspMsg, rsp, id)
}
