package synch

import (
	"context"
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/core/blockchain"
	"github.com/Qitmeer/qng/core/blockchain/token"
	"github.com/Qitmeer/qng/core/blockchain/utxo"
	"github.com/Qitmeer/qng/core/event"
	"github.com/Qitmeer/qng/core/json"
	"github.com/Qitmeer/qng/core/types"
	"github.com/Qitmeer/qng/meerdag"
	"github.com/Qitmeer/qng/p2p/common"
	"github.com/Qitmeer/qng/p2p/peers"
	pb "github.com/Qitmeer/qng/p2p/proto/v1"
	libp2pcore "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/network"
	"time"
)

const (
	MinSnapSyncNumber     = 200
	SnapSyncReqInterval   = time.Second * 5
	MinSnapSyncChainDepth = 20000
)

func (ps *PeerSync) loadSnapSync() {
	data, err := ps.sy.p2p.Consensus().DatabaseContext().GetSnapSync()
	if err != nil {
		log.Error(err.Error())
		return
	}
	if len(data) <= 0 {
		return
	}

	snapStatus := NewSnapStatusFromBytes(data)
	if snapStatus == nil {
		return
	}
	snapStatus.syncPoint = ps.Chain().BlockDAG().GetBlockById(uint(snapStatus.PointID))
	if snapStatus.syncPoint == nil {
		return
	}
	ps.snapStatus = snapStatus
	log.Info("Load snap sync info", "data", ps.snapStatus.ToInfo())
	err = ps.Chain().BeginSnapSyncing()
	if err != nil {
		log.Info("End snap-sync", "err", err.Error())
		return
	}

}

func (ps *PeerSync) saveSnapSync() {
	if !ps.IsSnapSync() {
		return
	}
	data, err := ps.snapStatus.Bytes()
	if err != nil {
		log.Error(err.Error())
		return
	}
	err = ps.sy.p2p.Consensus().DatabaseContext().PutSnapSync(data)
	if err != nil {
		log.Error(err.Error())
		return
	}
	log.Info("Save snap sync info")
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
	if !ps.sy.p2p.IsSnap() {
		if ps.IsSnapSync() {
			log.Error("There is an unfinished snap-sync, please enable the snap service")
		}
		return false
	}

	if ps.IsSnapSync() {
		return ps.continueSnapSync()
	}

	best := ps.Chain().BestSnapshot()
	bestPeer := ps.getBestPeer(false)
	if bestPeer == nil {
		return false
	}
	gs := bestPeer.GraphState()
	if gs.GetTotal() < best.GraphState.GetTotal()+MinSnapSyncChainDepth {
		return false
	}
	if !isValidSnapPeer(bestPeer) {
		snapPeer, change := ps.getSnapSyncPeer(ps.sy.p2p.Consensus().Config().SnapTimeout)
		if snapPeer == nil {
			if change {
				return false
			}
			return true
		}
		bestPeer = snapPeer
	}
	if !ps.IsRunning() {
		return true
	}
	// Start syncing from the best peer if one was selected.
	ps.processID++
	ps.processwg.Add(1)
	ps.SetSyncPeer(bestPeer)

	defer func() {
		defer ps.processwg.Done()
		ps.SetSyncPeer(nil)
	}()

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

	ps.snapStatus = NewSnapStatus(bestPeer.GetID())

	log.Info("Snap syncing", "cur", best.GraphState.String(), "peer", bestPeer.GetID().String(), "processID", ps.getProcessID())
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderStart))
	// ------
	err := ps.Chain().BeginSnapSyncing()
	if err != nil {
		log.Info("End snap-sync", "err", err.Error())
		return true
	}
	add := 0
	endSnapSyncing := func() {
		ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderEnd))
		if ps.snapStatus.IsCompleted() {
			log.Info("Snap-sync has ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "add", add, "processID", ps.getProcessID())
		} else {
			log.Warn("Snap-sync illegal ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "add", add, "processID", ps.getProcessID())
		}
		sp := ps.snapStatus.GetSyncPoint()
		if sp != nil {
			bestPeer.UpdateSyncPoint(sp.GetHash())
			log.Debug("Snap-sync update sync point", "point", sp.GetHash().String())
		}
		if ps.snapStatus.IsCompleted() {
			ps.snapStatus = nil
		} else {
			go ps.TryAgainUpdateSyncPeer(false)
		}
	}

	for !ps.snapStatus.IsPointCompleted() {
		ret := ps.trySyncSnapStatus(bestPeer)
		if ret == nil {
			log.Warn("Snap-sync no rsp data")
			endSnapSyncing()
			return true //Exit midway
		}
		sds, err := ps.processRsp(ret)
		if err != nil {
			log.Warn(err.Error())
			continue
		}
		if len(sds) <= 0 {
			log.Warn("No process snap sync data")
			continue
		}
		latest, err := ps.Chain().ProcessBlockBySnap(sds)
		if err != nil {
			panic(err.Error())
		}
		ps.snapStatus.SetSyncPoint(latest)
		add += len(sds)
		log.Trace("Snap-sync", "point", latest.GetHash().String(), "data_num", len(sds), "total", add)
	}

	err = ps.Chain().MeerChain().SyncTo(ps.snapStatus.GetSyncPoint().GetState().GetEVMHash())
	if err != nil {
		log.Error(err.Error())
	} else {
		ps.snapStatus.CompleteEVM()
	}
	ps.sy.p2p.BlockChain().EndSnapSyncing()
	endSnapSyncing()
	return true
}

func (ps *PeerSync) continueSnapSync() bool {
	if ps.snapStatus.IsEVMCompleted() {
		log.Warn("No need continue snap-sync")
		return false
	}
	log.Info("Continue snap-sync", "point", ps.snapStatus.syncPoint.GetHash().String(), "point_order", ps.snapStatus.GetSyncPoint().GetOrder(), "status", fmt.Sprintf("%v", ps.snapStatus.ToInfo()))

	best := ps.Chain().BestSnapshot()
	bestPeer, _ := ps.getSnapSyncPeer(0)
	if bestPeer == nil {
		return true
	}

	if !ps.IsRunning() {
		return true
	}
	// Start syncing from the best peer if one was selected.
	ps.processID++
	ps.processwg.Add(1)
	ps.SetSyncPeer(bestPeer)

	defer func() {
		defer ps.processwg.Done()
		ps.SetSyncPeer(nil)
	}()

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

	ps.snapStatus.ResetPeer(bestPeer.GetID())

	log.Info("Snap syncing", "cur", best.GraphState.String(), "peer", bestPeer.GetID().String(), "processID", ps.getProcessID())
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderStart))

	err := ps.Chain().MeerChain().SyncTo(ps.snapStatus.GetSyncPoint().GetState().GetEVMHash())
	if err != nil {
		log.Error(err.Error())
		return true
	} else {
		ps.snapStatus.CompleteEVM()
	}
	ps.sy.p2p.BlockChain().EndSnapSyncing()

	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderEnd))

	sp := ps.snapStatus.GetSyncPoint()
	if sp != nil {
		bestPeer.UpdateSyncPoint(sp.GetHash())
		log.Debug("Snap-sync update sync point", "point", sp.GetHash().String())
	}
	if ps.snapStatus.IsCompleted() {
		log.Info("Snap-sync has ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "processID", ps.getProcessID())
		ps.snapStatus = nil
	} else {
		log.Error("snapStatus is not completed")
	}
	return true
}

func (ps *PeerSync) trySyncSnapStatus(pe *peers.Peer) *pb.SnapSyncRsp {
	var ret *pb.SnapSyncRsp
	for ret == nil {
		tryReq := 10
		for i := 0; i < tryReq; i++ {
			if !ps.IsRunning() {
				return nil
			}
			rsp, err := ps.syncSnapStatus(pe)
			if err != nil {
				log.Warn("Snap-sync waiting for next try", "err", err.Error(), "cur", i, "max", tryReq)
				time.Sleep(SnapSyncReqInterval)
				continue
			}
			ret = rsp
			break
		}
		if ret != nil {
			return ret
		}
		log.Warn("Try change snap peer")
		newPeer, _ := ps.getSnapSyncPeer(0)
		if newPeer == nil {
			return nil
		}
		pe = newPeer
		ps.snapStatus.ResetPeer(pe.GetID())
	}
	return ret
}

func (ps *PeerSync) syncSnapStatus(pe *peers.Peer) (*pb.SnapSyncRsp, error) {
	req := &pb.SnapSyncReq{Locator: []*pb.Locator{}}

	targetBlock, stateRoot := ps.snapStatus.GetTarget()
	if targetBlock != nil {
		sp := ps.snapStatus.GetSyncPoint()
		req.Target = &pb.Locator{Block: &pb.Hash{Hash: targetBlock.Bytes()}, Root: &pb.Hash{Hash: stateRoot.Bytes()}}
		req.Locator = append(req.Locator, &pb.Locator{
			Block: &pb.Hash{Hash: sp.GetHash().Bytes()},
			Root:  &pb.Hash{Hash: sp.GetState().Root().Bytes()},
		})
	} else {
		zeroH := &pb.Hash{Hash: hash.ZeroHash.Bytes()}
		req.Target = &pb.Locator{Block: zeroH, Root: zeroH}
		point := pe.SyncPoint()
		mainLocator, mainStateRoot := ps.dagSync.GetMainLocator(point, true)
		for i := 0; i < len(mainLocator); i++ {
			req.Locator = append(req.Locator, &pb.Locator{
				Block: &pb.Hash{Hash: mainLocator[i].Bytes()},
				Root:  &pb.Hash{Hash: mainStateRoot[i].Bytes()},
			})
		}
	}

	ret, err := ps.sy.Send(pe, RPCSyncSnap, req)
	if err != nil {
		return nil, err
	}
	return ret.(*pb.SnapSyncRsp), nil
}

func (ps *PeerSync) processRsp(ssr *pb.SnapSyncRsp) ([]*blockchain.SnapData, error) {
	if isLocatorEmpty(ssr.Target) {
		return nil, fmt.Errorf("No snap sync data")
	}
	targetBlock := changePBHashToHash(ssr.Target.Block)
	stateRoot := changePBHashToHash(ssr.Target.Root)
	ps.snapStatus.SetTarget(targetBlock, stateRoot)

	log.Trace("Snap-sync receive", "targetBlock", targetBlock.String(), "stateRoot", stateRoot.String(), "dataNum", len(ssr.Datas), "processID", ps.getProcessID())

	if len(ssr.Datas) <= 0 {
		return nil, nil
	}
	ret := []*blockchain.SnapData{}
	for _, data := range ssr.Datas {
		sd := blockchain.NewSnapData()
		if len(data.Block) > 0 {
			block, err := types.NewBlockFromBytes(data.Block)
			if err != nil {
				return nil, err
			}
			sd.SetBlock(block)
		}
		if len(data.Stxos) > 0 {
			sts, err := utxo.DeserializeSpendJournalEntry(data.Stxos)
			if err != nil {
				return nil, err
			}
			sd.SetStxos(sts)
		}
		if len(data.DagBlock) > 0 {
			dblock, err := ps.Chain().BlockDAG().NewBlockFromBytes(data.DagBlock)
			if err != nil {
				return nil, err
			}
			sd.SetDAGBlock(dblock)
		}
		sd.SetMain(data.Main)
		if len(data.TokenState) > 0 {
			ts, err := token.NewTokenStateFromBytes(data.TokenState)
			if err != nil {
				return nil, err
			}
			sd.SetTokenState(ts)
		}
		if !isZeroPBHash(data.PrevTSHash) {
			sd.SetPrevTSHash(changePBHashToHash(data.PrevTSHash))
		}
		ret = append(ret, sd)
	}
	return ret, nil
}

func (ps *PeerSync) getSnapSyncPeer(timeout int) (*peers.Peer, bool) {
	start := time.Now()
	var pe *peers.Peer
	for pe == nil {
		pe = ps.getBestPeer(true)
		if pe == nil {
			if timeout > 0 {
				if time.Since(start) > time.Duration(timeout)*time.Second {
					return nil, true
				}
			}
			log.Debug("Try to get snap-sync peer", "cost", time.Since(start).String())
			time.Sleep(SnapSyncReqInterval)
		}
		if !ps.IsRunning() {
			return nil, false
		}
	}
	return pe, false
}

func (s *Sync) sendSnapSyncRequest(stream network.Stream, pe *peers.Peer) (*pb.SnapSyncRsp, *common.Error) {
	e := ReadRspCode(stream, s.p2p)
	if !e.Code.IsSuccess() {
		e.Add("get block date request rsp")
		return nil, e
	}
	msg := &pb.SnapSyncRsp{}
	if err := DecodeMessage(stream, s.p2p, msg); err != nil {
		return nil, common.NewError(common.ErrStreamRead, err)
	}
	return msg, nil
}

func (s *Sync) snapSyncHandler(ctx context.Context, msg interface{}, stream libp2pcore.Stream, pe *peers.Peer) *common.Error {
	m, ok := msg.(*pb.SnapSyncReq)
	if !ok {
		err := fmt.Errorf("message is not type *pb.SnapSyncReq")
		return ErrMessage(err)
	}
	log.Debug("Received Snap-sync request", "peer", pe.GetID().String())
	blocks, target, err := s.peerSync.dagSync.CalcSnapSyncBlocks(changePBLocatorsToLocators(m.Locator), MaxBlockLocatorsPerMsg, changePBLocatorToLocator(m.Target))
	if err != nil {
		log.Error(err.Error())
		return ErrMessage(err)
	}
	if len(blocks) <= 0 {
		return ErrMessage(fmt.Errorf("Can't find blocks for snap-sync"))
	}
	rsp := &pb.SnapSyncRsp{Datas: []*pb.TransferData{}}
	if isLocatorEmpty(m.Target) {
		rsp.Target = &pb.Locator{
			Block: &pb.Hash{Hash: target.GetHash().Bytes()},
			Root:  &pb.Hash{Hash: target.GetState().Root().Bytes()},
		}
	} else {
		rsp.Target = m.Target
	}
	for _, block := range blocks {
		data := &pb.TransferData{}
		blkBytes, err := s.p2p.BlockChain().FetchBlockBytesByHash(block.GetHash())
		if err != nil {
			return ErrMessage(err)
		}
		data.Block = blkBytes

		stxo, err := s.p2p.BlockChain().DB().GetSpendJournal(block.GetHash())
		if err != nil {
			return ErrMessage(err)
		}
		data.Stxos = stxo

		data.DagBlock = block.Bytes()
		data.Main = s.p2p.BlockChain().BlockDAG().IsOnMainChain(block.GetID())
		ts := s.p2p.BlockChain().GetTokenState(uint32(block.GetID()))
		if ts != nil {
			prevTSHash, err := meerdag.DBGetDAGBlockHashByID(s.p2p.BlockChain().DB(), uint64(ts.PrevStateID))
			if err != nil {
				return ErrMessage(err)
			}
			serializedData, err := ts.Serialize()
			if err != nil {
				return ErrMessage(err)
			}
			data.TokenState = serializedData
			data.PrevTSHash = &pb.Hash{
				Hash: prevTSHash.Bytes(),
			}
		} else {
			data.PrevTSHash = &pb.Hash{
				Hash: hash.ZeroHash.Bytes(),
			}
		}
		if uint64(rsp.SizeSSZ()+data.SizeSSZ()+BLOCKDATA_SSZ_HEAD_SIZE) >= s.p2p.Encoding().GetMaxChunkSize() {
			break
		}
		rsp.Datas = append(rsp.Datas, data)
	}
	return s.EncodeResponseMsg(stream, rsp)
}
