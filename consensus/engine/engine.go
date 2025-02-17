package engine

type Engine interface {
	Type() EngineType
	Name() string
	Digest() []byte
	Bytes() []byte
}
