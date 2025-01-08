package synch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/Qitmeer/qng/p2p/common"
	"github.com/Qitmeer/qng/p2p/peers"
	pb "github.com/Qitmeer/qng/p2p/proto/v1"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"io"
)

const (
	SnapSyncReqMsg = 0x64
	SnapSyncRspMsg = 0x65
)

func (ps *PeerSync) establishPeerConnection(pe *peers.Peer) {
	if !ps.checkPeerConnection(pe) {
		return
	}
	err := ps.sy.establishPeerConnection(pe)
	if err != nil {
		log.Error(err.Error())
	}
}

func (ps *PeerSync) checkPeerConnection(pe *peers.Peer) bool {
	if !ps.IsRunning() {
		return false
	}
	if pe.GetReadWrite() != nil {
		return false
	}
	if !pe.IsSupportLongChan() {
		return false
	}
	if pe.Direction() != network.DirOutbound {
		return false
	}
	return true
}

func (s *Sync) establishPeerConnection(pe *peers.Peer) error {
	proto := RPCPeerConn
	log.Trace("Send message", "protocol", getProtocol(s.p2p, proto), "peer", pe.IDWithAddress())

	curState := s.p2p.Host().Network().Connectedness(pe.GetID())
	if curState == network.CannotConnect || curState == network.NotConnected {
		return common.NewErrorStr(common.ErrLibp2pConnect, curState.String()).ToError()
	}

	topic := getProtocol(s.p2p, proto)
	ctx := context.Background()
	stream, err := s.p2p.Host().NewStream(ctx, pe.GetID(), protocol.ID(topic))
	if err != nil {
		return common.NewErrorStr(common.ErrStreamBase, fmt.Sprintf("open stream on topic %v failed", topic)).ToError()
	}

	common.EgressConnectMeter.Mark(1)

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	pe.SetReadWrite(rw)

	go func() {
		s.processMessage(stream, pe)
		pe.SetReadWrite(nil)
		err := stream.Close()
		if err != nil {
			log.Warn(err.Error())
		}
	}()
	return err
}

func (s *Sync) registerPeerConnection() {
	basetopic := RPCPeerConn
	topic := getProtocol(s.p2p, basetopic)

	s.p2p.Host().SetStreamHandler(protocol.ID(topic), func(stream network.Stream) {
		defer func() {
			err := stream.Close()
			if err != nil {
				log.Error(err.Error())
			}
		}()
		if !s.p2p.IsRunning() {
			log.Debug("PeerSync is not running, ignore the handling", "protocol", topic)
			return
		}
		common.IngressConnectMeter.Mark(1)
		pe := s.p2p.Peers().Get(stream.Conn().RemotePeer())
		if (pe == nil || pe.ChainState() == nil) && basetopic != RPCChainState && basetopic != RPCChainStateV2 && basetopic != RPCGoodByeTopic {
			log.Debug("Peer is not init, ignore the handling", "protocol", topic, "pe", stream.Conn().RemotePeer())
			return
		}

		if pe == nil {
			return
		}
		pe.UpdateAddrDir(nil, stream.Conn().RemoteMultiaddr(), stream.Conn().Stat().Direction)

		log.Trace("Stream handler", "protocol", topic, "peer", pe.IDWithAddress())
		if pe.GetReadWrite() != nil {
			log.Warn("Stream handler duplicate", "protocol", topic, "peer", pe.IDWithAddress())
			return
		}
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		pe.SetReadWrite(rw)
		defer pe.SetReadWrite(nil)

		s.processMessage(stream, pe)
	})
}

func (s *Sync) processMessage(stream network.Stream, pe *peers.Peer) {
	rw := pe.GetReadWrite()
	for {
		dataHead := make([]byte, 8)
		size, err := rw.Read(dataHead)
		if err == io.EOF {
			log.Debug("Base Stream closed by peer", "peer", pe.IDWithAddress())
			break
		}
		if err != nil {
			log.Warn("Error reading from base stream", "peer", pe.IDWithAddress(), "error", err)
			break
		}
		if size != 8 {
			log.Warn("Error message head size", "peer", pe.IDWithAddress())
			break
		}
		dataSize := int64(binary.BigEndian.Uint64(dataHead))
		log.Debug("Receive message head", "peer", pe.IDWithAddress(), "size", dataSize)
		msgData := make([]byte, dataSize)
		size, err = io.ReadFull(rw, msgData)
		if err == io.EOF {
			log.Debug("Base Stream closed by peer", "peer", pe.IDWithAddress())
			break
		}
		if err != nil {
			log.Warn("Error reading from base stream", "peer", pe.IDWithAddress(), "error", err)
			break
		}
		if int64(size) != dataSize {
			log.Warn("Receive error size message data")
			continue
		}
		msgID := msgData[0]
		log.Debug("Receive message", "msgid", msgID, "peer", pe.IDWithAddress(), "size", size)
		//
		s.p2p.IncreaseBytesRecv(stream.Conn().RemotePeer(), size)
		if msgID == SnapSyncReqMsg {
			msg := &pb.SnapSyncReq{}
			err = s.p2p.Encoding().DecodeWithMaxLength(bytes.NewReader(msgData[1:]), msg)
			if err != nil {
				log.Warn(err.Error())
				break
			}
			rsp, e := s.doSnapSyncHandler(msg, pe)
			if e != nil {
				log.Warn(err.Error())
				continue
			}
			var buff bytes.Buffer
			err := buff.WriteByte(SnapSyncRspMsg)
			if err != nil {
				log.Warn(err.Error())
				break
			}
			size, err = s.p2p.Encoding().EncodeWithMaxLength(&buff, rsp)
			if err != nil {
				log.Warn(err.Error())
				break
			}
			bys := buff.Bytes()
			bl := len(bys)
			if bl > 0 {
				log.Debug("Send message", "msgid", SnapSyncRspMsg, "peer", pe.IDWithAddress(), "size", size, "buff", bl)
				bs := make([]byte, 8)
				binary.BigEndian.PutUint64(bs, uint64(bl))
				size, err = rw.Write(bs)
				if err != nil {
					log.Warn(err.Error())
					break
				}
				if size != 8 {
					log.Warn("dataHead size error", "size", size)
					break
				}
				size, err = rw.Write(bys)
				if err != nil {
					log.Warn(err.Error())
					break
				}
				s.p2p.IncreaseBytesSent(pe.GetID(), size)
				err = rw.Flush()
				if err != nil {
					log.Warn(err.Error())
					break
				}
			} else {
				break
			}
		} else if msgID == SnapSyncRspMsg {
			msg := &pb.SnapSyncRsp{}
			err = s.p2p.Encoding().DecodeWithMaxLength(bytes.NewReader(msgData[1:]), msg)
			if err != nil {
				log.Warn(err.Error())
				break
			}
			select {
			case pe.Recv() <- msg:
			case <-s.peerSync.quit:
			}

		}
	}
}
