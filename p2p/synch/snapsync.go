package synch

import (
	"context"
	"fmt"
	"github.com/Qitmeer/qng/core/event"
	"github.com/Qitmeer/qng/core/json"
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
	err := ps.sy.p2p.BlockChain().BeginSnapSyncing()
	if err != nil {
		log.Error(err.Error())
		return false
	}
	for !ps.snapStatus.isCompleted() {
		ret, err := ps.syncSnapStatus(bestPeer)
		if err != nil {
			log.Warn("Snap-sync", "err", err.Error())
			break
		}
		err = ps.snapStatus.processRsp(ret, ps.sy.p2p)
		if err != nil {
			break
		}
	}
	ps.sy.p2p.BlockChain().EndSnapSyncing()
	// ------
	ps.sy.p2p.Consensus().Events().Send(event.New(event.DownloaderEnd))
	if ps.snapStatus.isCompleted() {
		log.Info("Snap-sync has ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "processID", ps.getProcessID())
	} else {
		log.Warn("Snap-sync illegal ended", "spend", time.Since(startTime).Truncate(time.Second).String(), "processID", ps.getProcessID())
	}

	ps.SetSyncPeer(nil)
	ps.processwg.Done()

	return true
}

func (ps *PeerSync) syncSnapStatus(pe *peers.Peer) (*pb.SnapSyncRsp, error) {
	point := pe.SyncPoint()
	mainLocator, mainStateRoot := ps.dagSync.GetMainLocator(point, true)
	locator := []*pb.Locator{}
	for i := 0; i < len(mainLocator); i++ {
		locator = append(locator, &pb.Locator{
			Block:     &pb.Hash{Hash: mainLocator[i].Bytes()},
			StateRoot: &pb.Hash{Hash: mainStateRoot[i].Bytes()},
		})
	}
	req := &pb.SnapSyncReq{
		Locator: locator,
	}

	targetBlock, stateRoot := ps.snapStatus.GetTarget()
	if targetBlock != nil {
		req.TargetBlock = &pb.Hash{Hash: targetBlock.Bytes()}
		req.StateRoot = &pb.Hash{Hash: stateRoot.Bytes()}
	}

	ret, err := ps.sy.Send(pe, RPCSyncSnap, req)
	if err != nil {
		return nil, err
	}
	return ret.(*pb.SnapSyncRsp), nil
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
		err := fmt.Errorf("message is not type *pb.Hash")
		return ErrMessage(err)
	}
	return s.EncodeResponseMsg(stream, m)
}
