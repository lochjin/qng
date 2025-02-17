package config

import "github.com/Qitmeer/qng/consensus/engine"

type Config interface {
	Type() engine.EngineType
	Check() error
}
