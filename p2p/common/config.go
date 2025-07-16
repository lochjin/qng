/*
 * Copyright (c) 2017-2020 The qitmeer developers
 */

package common

import (
	"github.com/Qitmeer/qng/core/protocol"
	"github.com/Qitmeer/qng/params"
	"os"
)

// Config for the p2p service.
// to initialize the p2p service.
type Config struct {
	NoDiscovery          bool
	EnableUPnP           bool
	StaticPeers          []string
	BootstrapNodeAddr    []string
	Discv5BootStrapAddr  []string
	DataDir              string
	MaxPeers             uint
	MaxInbound           int
	MetaDataDir          string
	ReadWritePermissions os.FileMode
	AllowListCIDR        string
	DenyListCIDR         []string
	TCPPort              uint
	UDPPort              uint
	EnableNoise          bool
	RelayNodeAddr        string
	LocalIP              string
	HostAddress          string
	HostDNS              string
	PrivateKey           string
	Encoding             string
	// ProtocolVersion specifies the maximum protocol version to use and
	// advertise.
	ProtocolVersion uint32
	Services        protocol.ServiceFlag
	UserAgent       string
	// DisableRelayTx specifies if the remote peer should be informed to
	// not send inv messages for transactions.
	DisableRelayTx bool
	MaxOrphanTxs   int
	Params         *params.Params
	Banning        bool // Open or not ban module
	DisableListen  bool
	LANPeers       []string
	IsCircuit      bool
	Consistency    bool

	// EnableVNC specifies whether to enable the VNC stream handler.
	EnableVNC bool
	// VNCBindAddr defines the local TCP address of the VNC server to forward to (default: 127.0.0.1:5900).
	VNCBindAddr string
	// VNCAllowedPeerIDs defines the list of allowed peer IDs for accessing the VNC stream.
	VNCAllowedPeerIDs []string `long:"vncallowpeer" description:"List of allowed peer IDs for VNC access over libp2p"`
	// EnableVNCProxy specifies whether to act as a VNC proxy client (reverse tunnel).
	EnableVNCProxy bool `long:"vncproxy" description:"Enable VNC bridge client mode to connect to a remote peer"`
	// VNCProxyPeerID defines the remote peer ID to connect to for accessing VNC.
	VNCProxyPeerID string `long:"vncpeer" description:"Remote libp2p peer ID that provides the VNC service"`
	// VNCProxyListenAddr defines the local listen address for the VNC client (e.g. vncviewer).
	VNCProxyListenAddr string `long:"vnclisten" description:"Local TCP address to accept vncviewer connections (default: 127.0.0.1:5900)"`
}
