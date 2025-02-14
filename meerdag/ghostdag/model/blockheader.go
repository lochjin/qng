package model

import (
	"github.com/Qitmeer/qng/consensus/pow"
)

type BlockHeader interface {
	Bits() uint32
	Pow() pow.IPow
}
