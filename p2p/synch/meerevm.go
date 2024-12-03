package synch

import (
	"github.com/Qitmeer/qng/p2p/common"
	"github.com/Qitmeer/qng/p2p/peers"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

func (ps *PeerSync) connectMeerEVM(pe *peers.Peer) {
	ms := pe.GetMeerState()
	if ms == nil {
		return
	}
	meers := common.NewMeerState(ms)
	meerP2P := ps.sy.p2p.BlockChain().MeerChain().Server()
	if meerP2P.PeerCount() > 0 {
		for _, p := range meerP2P.Peers() {
			if p.ID() == meers.Id {
				return
			}
		}
	}
	if len(meers.Enode) > 0 {
		node, err := enode.Parse(enode.ValidSchemes, meers.Enode)
		if err != nil {
			log.Error("invalid enode: %v", err)
		}
		meerP2P.AddPeer(node)
	} else if len(meers.ENR) > 0 {
		node, err := enode.Parse(enode.ValidSchemes, meers.ENR)
		if err != nil {
			log.Error("invalid enr: %v", err)
		}
		meerP2P.AddPeer(node)
	}
}
