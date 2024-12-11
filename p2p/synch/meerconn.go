package synch

import (
	"github.com/libp2p/go-libp2p/core/network"
	manet "github.com/multiformats/go-multiaddr/net"
	"net"
	"time"
)

type MeerConn struct {
	stream     network.Stream
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (mc *MeerConn) Read(b []byte) (n int, err error) {
	return mc.stream.Read(b)
}

func (mc *MeerConn) Write(b []byte) (n int, err error) {
	return mc.stream.Write(b)
}

func (mc *MeerConn) Close() error {
	return mc.Close()
}

func (mc *MeerConn) LocalAddr() net.Addr {
	return mc.localAddr
}

func (mc *MeerConn) RemoteAddr() net.Addr {
	return mc.remoteAddr
}

func (mc *MeerConn) SetDeadline(t time.Time) error {
	return mc.stream.SetDeadline(t)
}

func (mc *MeerConn) SetReadDeadline(t time.Time) error {
	return mc.stream.SetReadDeadline(t)
}

func (mc *MeerConn) SetWriteDeadline(t time.Time) error {
	return mc.stream.SetWriteDeadline(t)
}

func NewMeerConn(stream network.Stream) (*MeerConn, error) {
	localAddr, err := manet.ToNetAddr(stream.Conn().LocalMultiaddr())
	if err != nil {
		return nil, err
	}
	remoteAddr, err := manet.ToNetAddr(stream.Conn().RemoteMultiaddr())
	if err != nil {
		return nil, err
	}
	return &MeerConn{
		stream:     stream,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
	}, nil
}
