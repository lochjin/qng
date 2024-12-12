package synch

import (
	"context"
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/p2p/common"
	"github.com/Qitmeer/qng/p2p/peers"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

func (s *Sync) establishMeerConnection(pe *peers.Peer) error {
	proto := RPCMeerConn
	if !s.peerSync.IsRunning() {
		return errors.New("No run PeerSync")
	}
	if pe.Direction() != network.DirOutbound {
		return fmt.Errorf("Peer (%s) is not outbound", pe.GetID().String())
	}
	ms := pe.GetMeerState()
	if ms == nil {
		return fmt.Errorf("Peer (%s) no meer state", pe.GetID().String())
	}
	url := ms.Enode
	if len(url) <= 0 {
		url = ms.ENR
	}
	if len(url) <= 0 {
		return fmt.Errorf("Peer (%s) no meer state url", pe.GetID().String())
	}
	dest, err := CreateMeerNode(pe.Address(), url)
	if err != nil {
		return err
	}

	log.Trace("Send message", "protocol", getProtocol(s.p2p, proto), "peer", pe.IDWithAddress())

	curState := s.p2p.Host().Network().Connectedness(pe.GetID())
	if curState == network.CannotConnect {
		return common.NewErrorStr(common.ErrLibp2pConnect, curState.String()).ToError()
	}

	topic := getProtocol(s.p2p, proto)

	stream, err := s.p2p.Host().NewStream(context.Background(), pe.GetID(), protocol.ID(topic))
	if err != nil {
		return common.NewErrorStr(common.ErrStreamBase, fmt.Sprintf("open stream on topic %v failed", topic)).ToError()
	}
	defer stream.Close()

	common.EgressConnectMeter.Mark(1)
	qc, err := NewMeerConn(stream)
	if err != nil {
		return err
	}
	pe.SetMeerConn(true)
	defer pe.SetMeerConn(false)

	_, err = s.p2p.MeerServer().Connect(qc, dest)

	return err
}

func (s *Sync) registerMeerConnection() {
	basetopic := RPCMeerConn
	topic := getProtocol(s.p2p, basetopic)

	s.p2p.Host().SetStreamHandler(protocol.ID(topic), func(stream network.Stream) {
		if !s.p2p.IsRunning() {
			log.Debug("PeerSync is not running, ignore the handling", "protocol", topic)
			stream.Close()
			return
		}
		common.IngressConnectMeter.Mark(1)
		pe := s.p2p.Peers().Get(stream.Conn().RemotePeer())
		if (pe == nil || pe.ChainState() == nil) && basetopic != RPCChainState && basetopic != RPCChainStateV2 && basetopic != RPCGoodByeTopic {
			log.Debug("Peer is not init, ignore the handling", "protocol", topic, "pe", stream.Conn().RemotePeer())
			stream.Close()
			return
		}

		if pe == nil {
			stream.Close()
			return
		}
		pe.UpdateAddrDir(nil, stream.Conn().RemoteMultiaddr(), stream.Conn().Stat().Direction)

		log.Trace("Stream handler", "protocol", topic, "peer", pe.IDWithAddress())
		qc, err := NewMeerConn(stream)
		if err != nil {
			log.Error(err.Error())
			stream.Close()
			return
		}
		pe.SetMeerConn(true)
		defer pe.SetMeerConn(false)

		_, err = s.p2p.MeerServer().Connect(qc, nil)
		if err != nil {
			log.Error(err.Error())
			stream.Close()
			return
		}
	})
}
