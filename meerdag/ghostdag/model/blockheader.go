package model

import (
	"github.com/Qitmeer/qng/consensus/engine/pow"
)

type BlockHeader interface {
	Bits() uint32
	Pow() pow.IPow
}
