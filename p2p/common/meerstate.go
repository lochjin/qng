package common

import (
	v2 "github.com/Qitmeer/qng/p2p/proto/v2"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type MeerState struct {
	Id     enode.ID
	Number uint64
	Enode  string
	ENR    string
}

func NewMeerState(ms *v2.MeerState) *MeerState {
	var id enode.ID
	copy(id[:], ms.Id.Hash)

	return &MeerState{
		Id:     id,
		Number: ms.Number,
		Enode:  string(ms.Enode),
		ENR:    string(ms.Enr),
	}
}
