package rawdb

import (
	"path/filepath"

	"github.com/ethereum/go-ethereum/ethdb"
)

// The list of table names of chain freezer.
const (
	ChainFreezerHeaderTable = "headers"

	ChainFreezerBlockTable = "blocks"

	ChainFreezerDAGBlockTable = "dagblocks"
)

// chainFreezerNoSnappy configures whether compression is disabled for the ancient-tables.
var chainFreezerNoSnappy = map[string]bool{
	ChainFreezerHeaderTable:   false,
	ChainFreezerBlockTable:    false,
	ChainFreezerDAGBlockTable: false,
}

const (
	// stateHistoryTableSize defines the maximum size of freezer data files.
	stateHistoryTableSize = 2 * 1000 * 1000 * 1000

	// stateHistoryAccountIndex indicates the name of the freezer state history table.
	stateHistoryMeta         = "history.meta"
	stateHistoryAccountIndex = "account.index"
	stateHistoryStorageIndex = "storage.index"
	stateHistoryAccountData  = "account.data"
	stateHistoryStorageData  = "storage.data"
)

// stateFreezerNoSnappy configures whether compression is disabled for the state freezer.
var stateFreezerNoSnappy = map[string]bool{
	stateHistoryMeta:         true,
	stateHistoryAccountIndex: false,
	stateHistoryStorageIndex: false,
	stateHistoryAccountData:  false,
	stateHistoryStorageData:  false,
}

// The list of identifiers of ancient stores.
var (
	ChainFreezerName       = "chain"        // the folder name of chain segment ancient store.
	MerkleStateFreezerName = "state"        // the folder name of state history ancient store.
	VerkleStateFreezerName = "state_verkle" // the folder name of state history ancient store.
)

// freezers the collections of all builtin freezers.
var freezers = []string{ChainFreezerName, MerkleStateFreezerName, VerkleStateFreezerName}

// NewStateFreezer initializes the ancient store for state history.
//
//   - if the empty directory is given, initializes the pure in-memory
//     state freezer (e.g. dev mode).
//   - if non-empty directory is given, initializes the regular file-based
//     state freezer.
func NewStateFreezer(ancientDir string, verkle bool, readOnly bool) (ethdb.ResettableAncientStore, error) {
	if ancientDir == "" {
		return NewMemoryFreezer(readOnly, stateFreezerNoSnappy), nil
	}
	var name string
	if verkle {
		name = filepath.Join(ancientDir, VerkleStateFreezerName)
	} else {
		name = filepath.Join(ancientDir, MerkleStateFreezerName)
	}
	return newResettableFreezer(name, "state", readOnly, stateHistoryTableSize, stateFreezerNoSnappy)
}
