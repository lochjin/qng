package engine

type Engine interface {
	Type() EngineType
	Digest() []byte
	Bytes() []byte
}
