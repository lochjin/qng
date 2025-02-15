package engine

import "fmt"

type EngineType byte

const (
	PoWEngineType EngineType = 100

	PoAEngineType EngineType = 101

	PoSEngineType EngineType = 102

	DPoSEngineType EngineType = 103

	BFTEngineType EngineType = 104
)

var etStrings = map[EngineType]string{
	PoWEngineType:  "PoWEngineType",
	PoAEngineType:  "PoAEngineType",
	PoSEngineType:  "PoSEngineType",
	DPoSEngineType: "DPoSEngineType",
	BFTEngineType:  "BFTEngineType",
}

func (et EngineType) String() string {
	ret := et
	if ret.IsPoW() {
		ret = PoWEngineType
	}
	if s, ok := etStrings[ret]; ok {
		return s
	}
	return fmt.Sprintf("Unknown EngineType (%d)", byte(et))
}

func (et EngineType) IsPoW() bool {
	return et <= PoWEngineType
}

func (et EngineType) IsPoA() bool {
	return et == PoAEngineType
}
