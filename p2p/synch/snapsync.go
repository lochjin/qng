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
	if !ps.IsSnapSync() && gs.GetTotal() < best.GraphState.GetTotal()+MaxBlockLocatorsPerMsg {
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

	if ps.IsSnapSync() {
		ps.snapStatus.Reset(bestPeer.GetID())
	} else {
		ps.snapStatus = NewSnapStatus(bestPeer.GetID())
	}

	log.Info("Snap syncing", "cur", best.GraphState.String(), "target", ps.snapStatus.ToString(), "peer", bestPeer.GetID().String(), "processID", ps.getProcessID())
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderStart))
	// ------
	err := ps.Chain().BeginSnapSyncing()
	if err != nil {
		return false
	}
	add := 0
	ps.snapStatus.locker.Lock()
	for !ps.snapStatus.isCompleted() {
		ret, err := ps.syncSnapStatus(bestPeer)
		if err != nil {
			log.Warn("Snap-sync", "err", err.Error())
			break
		}
		sds, err := ps.processRsp(ret)
		if err != nil {
			log.Warn(err.Error())
			break
		}
		if len(sds) <= 0 {
			log.Warn("No process snap sync data")
			continue
		}
		latest, err := ps.Chain().ProcessBlockBySnap(sds)
		if err != nil {
			log.Warn(err.Error())
			break
		}
		ps.snapStatus.setSyncPoint(latest)
		add += len(sds)
	}
	ps.snapStatus.locker.Unlock()

	ps.sy.p2p.BlockChain().EndSnapSyncing()
	sp := ps.snapStatus.GetSyncPoint()
	if sp != nil {
		bestPeer.UpdateSyncPoint(sp.GetHash())
		log.Debug("Snap-sync update point", "point", sp.GetHash().String())
	}
	// ------
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderEnd))
	if ps.snapStatus.IsCompleted() {
		log.Info("Snap-sync has ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "add", add, "processID", ps.getProcessID())
	} else {
		log.Warn("Snap-sync illegal ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "add", add, "processID", ps.getProcessID())
	}

	ps.SetSyncPeer(nil)
	ps.processwg.Done()

	if ps.snapStatus.IsCompleted() {
		ps.snapStatus = nil
		return false
	} else {
		ps.snapStatus = nil
		go ps.TryAgainUpdateSyncPeer(false)
		return true
	}
}

func (ps *PeerSync) syncSnapStatus(pe *peers.Peer) (*pb.SnapSyncRsp, error) {
	req := &pb.SnapSyncReq{Locator: []*pb.Locator{}}

	targetBlock, stateRoot := ps.snapStatus.getTarget()
	if targetBlock != nil {
		sp := ps.snapStatus.getSyncPoint()
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
	ps.snapStatus.setTarget(targetBlock, stateRoot)

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
