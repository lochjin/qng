package graph

import (
	"context"
	"testing"
)

type TestClass struct {
	BaseCallback
	t *testing.T
}

func (tc TestClass) HandleNodeStart(ctx context.Context, node string, initialState State) {
	tc.t.Logf("At %s:%v", node, initialState)
}

func TestCallback(t *testing.T) {
	g := NewGraph(WithCallback(TestClass{BaseCallback: BaseCallback{}, t: t}))

	g.AddNode("node_0", func(_ context.Context, state State, opts Options) (State, error) {
		text := "I am node 0"
		t.Log(text)
		state["node_0"] = text
		return state, nil
	})
	g.AddNode(END, func(_ context.Context, state State, opts Options) (State, error) {
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
