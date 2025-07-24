package graph

import (
	"golang.org/x/net/context"
)

type GraphCallback interface {
	HandleNodeStart(ctx context.Context, node string, initialState State)
	HandleNodeEnd(ctx context.Context, node string, finalState State)
	HandleEdgeEntry(ctx context.Context, edge string, initialState State)
	HandleEdgeExit(ctx context.Context, edge string, finalState State, output string)
}

type BaseCallback struct{}

func (callback *BaseCallback) HandleNodeStart(ctx context.Context, node string, initialState State) {
}
func (callback *BaseCallback) HandleNodeEnd(ctx context.Context, node string, finalState State) {
}
func (callback *BaseCallback) HandleEdgeEntry(ctx context.Context, edge string, initialState State) {
}
func (callback *BaseCallback) HandleEdgeExit(ctx context.Context, edge string, finalState State, output string) {
}
