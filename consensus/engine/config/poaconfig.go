package config

import "github.com/Qitmeer/qng/consensus/engine"

type POAConfig struct {
	Period uint64 // Number of seconds between blocks to enforce
	Epoch  uint64 // Epoch length to reset votes and checkpoint
}

func (c *POAConfig) Type() engine.EngineType {
	return engine.POAEngineType
}

func (c *POAConfig) Check() error {
	return nil
}
