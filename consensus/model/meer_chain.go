package model

import (
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/rpc/api"
	"github.com/ethereum/go-ethereum/eth/downloader"
)

type MeerChain interface {
	RegisterAPIs(apis []api.API)
	GetBlockIDByTxHash(txhash *hash.Hash) uint64
	SyncMode() downloader.SyncMode
}
