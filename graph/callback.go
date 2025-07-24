package graph

import (
	"golang.org/x/net/context"
)

type NodeStartHandler interface {
	NodeStart(ctx context.Context, node string, state State)
}

type NodeEndHandler interface {
	NodeEnd(ctx context.Context, node string, state State)
}

type EdgeEntryHandler interface {
	EdgeEntry(ctx context.Context, edge string, state State)
}

type EdgeExitHandler interface {
	EdgeExit(ctx context.Context, edge string, state State, output string)
}
