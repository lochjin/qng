/*
 * Copyright (c) 2017-2025 The qitmeer developers
 */

package poa

import (
	"encoding/json"
	"fmt"
	"github.com/Qitmeer/qng/common/hash"
	"github.com/Qitmeer/qng/consensus/model"
	"github.com/Qitmeer/qng/rpc/api"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

// API is a user facing RPC API to allow controlling the signer and voting
// mechanisms of the proof-of-authority scheme.
type API struct {
	chain  model.BlockChain
	dagpoa *DagPoA
}

func NewPublicAPI(dagpoa *DagPoA, chain model.BlockChain) *API {
	return &API{chain: chain, dagpoa: dagpoa}
}

// GetSnapshot retrieves the state snapshot at a given block.
func (api *API) GetSnapshot(number *rpc.BlockNumber) (*Snapshot, error) {
	// Retrieve the requested block number (or current if none requested)
	var block model.Block
	if number == nil || *number == rpc.LatestBlockNumber {
		block = api.chain.GetMainChainTip()
	} else {
		block = api.chain.GetBlockByOrder(uint64(number.Int64()))

	}
	// Ensure we have an actually valid block and return its snapshot
	if block == nil {
		return nil, errUnknownBlock
	}
	return api.dagpoa.snapshot(uint64(block.GetHeight()), block.GetHash())
}

// GetSnapshotAtHash retrieves the state snapshot at a given block.
func (api *API) GetSnapshotAtHash(hash hash.Hash) (*Snapshot, error) {
	block := api.chain.GetBlock(&hash)
	if block == nil {
		return nil, errUnknownBlock
	}
	return api.dagpoa.snapshot(uint64(block.GetHeight()), block.GetHash())
}

// GetSigners retrieves the list of authorized signers at the specified block.
func (api *API) GetSigners(number *rpc.BlockNumber) ([]common.Address, error) {
	var block model.Block
	if number == nil || *number == rpc.LatestBlockNumber {
		block = api.chain.GetMainChainTip()
	} else {
		block = api.chain.GetBlockByOrder(uint64(number.Int64()))

	}
	// Ensure we have an actually valid block and return its snapshot
	if block == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.dagpoa.snapshot(uint64(block.GetHeight()), block.GetHash())
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

// GetSignersAtHash retrieves the list of authorized signers at the specified block.
func (api *API) GetSignersAtHash(hash hash.Hash) ([]common.Address, error) {
	block := api.chain.GetBlock(&hash)
	if block == nil {
		return nil, errUnknownBlock
	}

	snap, err := api.dagpoa.snapshot(uint64(block.GetHeight()), block.GetHash())
	if err != nil {
		return nil, err
	}
	return snap.signers(), nil
}

// Proposals returns the current proposals the node tries to uphold and vote on.
func (api *API) Proposals() map[common.Address]bool {
	api.dagpoa.lock.RLock()
	defer api.dagpoa.lock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.dagpoa.proposals {
		proposals[address] = auth
	}
	return proposals
}

// Propose injects a new authorization proposal that the signer will attempt to
// push through.
func (api *API) Propose(address common.Address, auth bool) {
	api.dagpoa.lock.Lock()
	defer api.dagpoa.lock.Unlock()

	api.dagpoa.proposals[address] = auth
}

// Discard drops a currently running proposal, stopping the signer from casting
// further votes (either for or against).
func (api *API) Discard(address common.Address) {
	api.dagpoa.lock.Lock()
	defer api.dagpoa.lock.Unlock()

	delete(api.dagpoa.proposals, address)
}

type status struct {
	InturnPercent float64                `json:"inturnPercent"`
	SigningStatus map[common.Address]int `json:"sealerActivity"`
	NumBlocks     uint64                 `json:"numBlocks"`
}

// Status returns the status of the last N blocks,
// - the number of active signers,
// - the number of signers,
// - the percentage of in-turn blocks
func (api *API) Status() (*status, error) {
	var (
		numBlocks = uint64(64)
		block     = api.chain.GetMainChainTip()
		diff      = uint64(0)
		optimals  = 0
	)
	snap, err := api.dagpoa.snapshot(uint64(block.GetHeight()), block.GetHash())
	if err != nil {
		return nil, err
	}
	var (
		signers = snap.signers()
	)
	signStatus := make(map[common.Address]int)
	for _, s := range signers {
		signStatus[s] = 0
	}

	for idx := uint64(0); idx < numBlocks; idx++ {
		if block == nil {
			break
		}
		h := api.chain.GetBlockHeader(block)
		if h == nil {
			return nil, fmt.Errorf("missing block %s", block.GetHash())
		}
		if h.Difficulty == diffInTurn {
			optimals++
		}
		diff += uint64(h.Difficulty)
		sealer, err := api.dagpoa.Author(h)
		if err != nil {
			return nil, err
		}
		signStatus[sealer]++

		if block.GetID() == 0 {
			break
		}
		block = api.chain.GetBlockById(block.GetMainParent())
	}

	return &status{
		InturnPercent: float64(100*optimals) / float64(numBlocks),
		SigningStatus: signStatus,
		NumBlocks:     numBlocks,
	}, nil
}

type blockNumberOrHashOrRLP struct {
	*rpc.BlockNumberOrHash
	RLP hexutil.Bytes `json:"rlp,omitempty"`
}

func (sb *blockNumberOrHashOrRLP) UnmarshalJSON(data []byte) error {
	bnOrHash := new(rpc.BlockNumberOrHash)
	// Try to unmarshal bNrOrHash
	if err := bnOrHash.UnmarshalJSON(data); err == nil {
		sb.BlockNumberOrHash = bnOrHash
		return nil
	}
	// Try to unmarshal RLP
	var input string
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	blob, err := hexutil.Decode(input)
	if err != nil {
		return err
	}
	sb.RLP = blob
	return nil
}

// GetSigner returns the signer for a specific qit block.
func (a *API) GetSigner(hashOrNumber string) (common.Address, error) {
	hn, err := api.NewHashOrNumber(hashOrNumber)
	if err != nil {
		mt := a.chain.GetMainChainTip()
		if mt == nil {
			return common.Address{}, fmt.Errorf("missing block")
		}
		header := a.chain.GetBlockHeader(mt)
		if header == nil {
			return common.Address{}, fmt.Errorf("missing block")
		}
		return a.dagpoa.Author(header)
	}
	var block model.Block
	if hn.IsHash() {
		block = a.chain.GetBlock(hn.Hash)
	} else {
		block = a.chain.GetBlockByOrder(hn.Number)
	}
	if block == nil {
		return common.Address{}, fmt.Errorf("missing block:%v", hn.String())
	}
	header := a.chain.GetBlockHeader(block)
	if header == nil {
		return common.Address{}, fmt.Errorf("missing block:%v", hn.String())
	}
	return a.dagpoa.Author(header)
}
