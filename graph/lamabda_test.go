package graph

import (
	"context"
	"testing"
)

func TestLamabda(t *testing.T) {
	g := NewGraph[State, State]()

	g.AddLambdaNode("node_0", func() error {
		text := "I am lambda node"
		t.Log(text)
		return nil
	})
	g.AddNode(END, func(_ context.Context, name string, state State) (State, error) {
		t.Log("I am end")
		return state, nil
	})
	g.AddEdge("node_0", END)
	g.SetEntryPoint("node_0")

	runnable, err := g.Compile()
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	input := State{"input": "hello world!"}
	output, err := runnable.Invoke(context.Background(), input)
	if err != nil {
		t.Fatalf("invoke error: %v", err)
	}
	t.Log(output)
}
