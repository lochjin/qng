package graph

import (
	"context"
	"testing"
)

type TestClass struct {
	t *testing.T
}

func (tc *TestClass) NodeStart(ctx context.Context, node string, state State) {
	tc.t.Logf("At %s:%v", node, state)
}

func TestCallback(t *testing.T) {
	g := NewGraph(WithNodeStartHandler(&TestClass{t: t}))

	g.AddNode("node_0", func(_ context.Context, name string, state State) (State, error) {
		text := "I am node 0"
		t.Log(text)
		state["node_0"] = text
		return state, nil
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
