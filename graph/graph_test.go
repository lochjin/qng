package graph_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/graph/llamago"
	"os"
	"testing"

	"github.com/Qitmeer/qng/graph"
	"github.com/tmc/langchaingo/llms"
)

func TestExampleMessageGraph(t *testing.T) {
	var llamagoModel string
	if llamagoModel = os.Getenv("LLAMAGO_TEST_MODEL"); llamagoModel == "" {
		t.Skip("LLAMAGO_TEST_MODEL not set")
		return
	}
	model, err := llamago.New(llamago.WithModel(os.Getenv(llamagoModel)))
	if err != nil {
		fmt.Println(err)
		return
	}

	g := graph.NewMessageGraph()

	g.AddNode("oracle", func(ctx context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		r, err := model.GenerateContent(ctx, state, llms.WithTemperature(0.0))
		if err != nil {
			return nil, err
		}
		return append(state,
			llms.TextParts(llms.ChatMessageTypeAI, r.Choices[0].Content),
		), nil
	})

	g.AddNode(graph.END, func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		return state, nil
	})

	g.AddEdge("oracle", graph.END)
	g.SetEntryPoint("oracle")

	runnable, err := g.Compile()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	// Let's run it!
	res, err := runnable.Invoke(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "What is 1 + 1?"),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(res)

	// Output:
	// [{human [What is 1 + 1?]} {ai [1 + 1 equals 2.]}]
}

func TestMessageGraph(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		buildGraph     func() *graph.MessageGraph
		inputMessages  []llms.MessageContent
		expectedOutput []llms.MessageContent
		expectedError  error
	}{
		{
			name: "Simple graph",
			buildGraph: func() *graph.MessageGraph {
				g := graph.NewMessageGraph()
				g.AddNode("node1", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
					return append(state, llms.TextParts(llms.ChatMessageTypeAI, "Node 1")), nil
				})
				g.AddNode("node2", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
					return append(state, llms.TextParts(llms.ChatMessageTypeAI, "Node 2")), nil
				})
				g.AddEdge("node1", "node2")
				g.AddEdge("node2", graph.END)
				g.SetEntryPoint("node1")
				return g
			},
			inputMessages: []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "Input")},
			expectedOutput: []llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeHuman, "Input"),
				llms.TextParts(llms.ChatMessageTypeAI, "Node 1"),
				llms.TextParts(llms.ChatMessageTypeAI, "Node 2"),
			},
			expectedError: nil,
		},
		{
			name: "Entry point not set",
			buildGraph: func() *graph.MessageGraph {
				g := graph.NewMessageGraph()
				g.AddNode("node1", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
					return state, nil
				})
				return g
			},
			expectedError: graph.ErrEntryPointNotSet,
		},
		{
			name: "Node not found",
			buildGraph: func() *graph.MessageGraph {
				g := graph.NewMessageGraph()
				g.AddNode("node1", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
					return state, nil
				})
				g.AddEdge("node1", "node2")
				g.SetEntryPoint("node1")
				return g
			},
			expectedError: fmt.Errorf("%w: node2", graph.ErrNodeNotFound),
		},
		{
			name: "No outgoing edge",
			buildGraph: func() *graph.MessageGraph {
				g := graph.NewMessageGraph()
				g.AddNode("node1", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
					return state, nil
				})
				g.SetEntryPoint("node1")
				return g
			},
			expectedError: fmt.Errorf("%w: node1", graph.ErrNoOutgoingEdge),
		},
		{
			name: "Error in node function",
			buildGraph: func() *graph.MessageGraph {
				g := graph.NewMessageGraph()
				g.AddNode("node1", func(_ context.Context, _ []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
					return nil, errors.New("node error")
				})
				g.AddEdge("node1", graph.END)
				g.SetEntryPoint("node1")
				return g
			},
			expectedError: errors.New("error in node node1: node error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := tc.buildGraph()
			runnable, err := g.Compile()
			if err != nil {
				if tc.expectedError == nil || !errors.Is(err, tc.expectedError) {
					t.Fatalf("unexpected compile error: %v", err)
				}
				return
			}

			output, err := runnable.Invoke(context.Background(), tc.inputMessages)
			if err != nil {
				if tc.expectedError == nil || err.Error() != tc.expectedError.Error() {
					t.Fatalf("unexpected invoke error: '%v', expected '%v'", err, tc.expectedError)
				}
				return
			}

			if tc.expectedError != nil {
				t.Fatalf("expected error %v, but got nil", tc.expectedError)
			}

			if len(output) != len(tc.expectedOutput) {
				t.Fatalf("expected output length %d, but got %d", len(tc.expectedOutput), len(output))
			}

			for i, msg := range output {
				got := fmt.Sprint(msg)
				expected := fmt.Sprint(tc.expectedOutput[i])
				if got != expected {
					t.Errorf("expected output[%d] content %q, but got %q", i, expected, got)
				}
			}
		})
	}
}

func TestConditionalEdge(t *testing.T) {
	g := graph.NewMessageGraph()

	g.AddNode("node_0", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		text := "I am node 0"
		t.Log(text)
		return append(state, llms.TextParts(llms.ChatMessageTypeAI, text)), nil
	})
	g.AddNode("node_1", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		text := "I am node 1"
		t.Log(text)
		return append(state, llms.TextParts(llms.ChatMessageTypeAI, text)), nil
	})
	g.AddNode("node_2", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		text := "I am node 2"
		t.Log(text)
		return append(state, llms.TextParts(llms.ChatMessageTypeAI, text)), nil
	})
	g.AddNode("node_3", func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		text := "I am node 3"
		t.Log(text)
		return append(state, llms.TextParts(llms.ChatMessageTypeAI, text)), nil
	})
	g.AddNode(graph.END, func(_ context.Context, state []llms.MessageContent, opts graph.Options) ([]llms.MessageContent, error) {
		t.Log("I am end")
		return state, nil
	})

	g.AddEdge("node_0", "node_1")
	g.AddConditionalEdge("node_1", func(_ context.Context, state []llms.MessageContent, _ graph.Options) string {
		if len(state) > 0 {
			for _, s := range state {
				if s.Parts[0].(llms.TextContent).String() == "node 2 branch" {
					return "node_2"
				}
			}
		}
		return "node_3"
	})

	g.AddEdge("node_2", graph.END)
	g.AddEdge("node_3", graph.END)

	g.SetEntryPoint("node_0")

	runnable, err := g.Compile()
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	t.Run("node_2 branch", func(t *testing.T) {
		input := []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, "node 2 branch")}
		output, err := runnable.Invoke(context.Background(), input)
		if err != nil {
			t.Fatalf("invoke error: %v", err)
		}
		if output[3].Parts[0].(llms.TextContent).Text != "I am node 2" {
			t.Errorf("expected no branch, got %v", output[1])
		}
		t.Log(output)
	})

	t.Run("node_3 branch", func(t *testing.T) {
		input := []llms.MessageContent{}
		output, err := runnable.Invoke(context.Background(), input)
		if err != nil {
			t.Fatalf("invoke error: %v", err)
		}
		if output[2].Parts[0].(llms.TextContent).Text != "I am node 3" {
			t.Errorf("expected no branch, got %v", output[1])
		}
		t.Log(output)
	})
}
