/*
 * Copyright (c) 2017-2020 The qitmeer developers
 */

package p2p

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	circuit "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	ma "github.com/multiformats/go-multiaddr"
	"go.opencensus.io/trace"
)

// MakePeer from multiaddress string.
func MakePeer(addr string) (*peer.AddrInfo, error) {
	maddr, err := MultiAddrFromString(addr)
	if err != nil {
		return nil, err
	}
	return peer.AddrInfoFromP2pAddr(maddr)
}

func dialRelayNode(ctx context.Context, h host.Host, relayAddr string) error {
	ctx, span := trace.StartSpan(ctx, "p2p_dialRelayNode")
	defer span.End()

	p, err := MakePeer(relayAddr)
	if err != nil {
		return err
	}

	return h.Connect(ctx, *p)
}

func tryReserveRelay(ctx context.Context, h host.Host, relayAddrStr string) error {
	relayAddr, err := ma.NewMultiaddr(relayAddrStr)
	if err != nil {
		return fmt.Errorf("invalid relay multiaddr: %w", err)
	}

	relayInfo, err := peer.AddrInfosFromP2pAddrs(relayAddr)
	if err != nil {
		return fmt.Errorf("failed to parse relay addr info: %w", err)
	}

	// do reservation
	rsvp, err := circuit.Reserve(ctx, h, relayInfo[0])
	if err != nil {
		return fmt.Errorf("reservation failed: %w", err)
	}
	h.Peerstore().AddAddrs(h.ID(), rsvp.Addrs, peerstore.PermanentAddrTTL)
	log.Info(fmt.Sprintf("Relay reservation successful. Expiration: %v, Addrs %v", rsvp.Expiration, rsvp.Addrs))
	return nil
}
