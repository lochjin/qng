package graph

import "context"

type LambdaFunction func() error

type LambdaNode struct {
	*BaseNode
	Function LambdaFunction
}

func (g *Graph[I, O]) AddLambdaNode(name string, fn LambdaFunction) {
	function := func(ctx context.Context, name string, state State) (State, error) {
		return state, fn()
	}
	g.nodes[name] = LambdaNode{
		BaseNode: &BaseNode{
			Name:     name,
			Function: function},
	}
}
