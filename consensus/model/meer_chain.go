package model

import (
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/rpc/api"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/p2p"
)

type MeerChain interface {
	RegisterAPIs(apis []api.API)
	GetBlockIDByTxHash(txhash *hash.Hash) uint64
	SyncMode() downloader.SyncMode
	Downloader() *downloader.Downloader
	SyncTo(target common.Hash) error
	Server() *p2p.Server
}
