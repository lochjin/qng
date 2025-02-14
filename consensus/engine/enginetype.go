package engine

import "fmt"

type EngineType byte

const (
	POWEngineType EngineType = 100

	POAEngineType EngineType = 101
)

var etStrings = map[EngineType]string{
	POWEngineType: "POWEngineType",
	POAEngineType: "POAEngineType",
}

func (et EngineType) String() string {
	if s, ok := etStrings[et]; ok {
		return s
	}
	return fmt.Sprintf("Unknown EngineType (%d)", byte(et))
}

func (et EngineType) IsPOW() bool {
	return et == POWEngineType
}

func (et EngineType) IsPOA() bool {
	return et == POAEngineType
}
