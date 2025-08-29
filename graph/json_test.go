package graph

import (
	"context"
	"errors"
	"fmt"
	"github.com/Qitmeer/qng/graph/llamago"
	"github.com/tmc/langchaingo/llms"
	"log"
	"os"
	"testing"
)

func TestJson(t *testing.T) {
	var llamagoModel string
	if llamagoModel = os.Getenv("LLAMAGO_TEST_MODEL"); llamagoModel == "" {
		t.Skip("LLAMAGO_TEST_MODEL not set")
		return
	}

	llm, err := llamago.New()
	if err != nil {
		fmt.Println(err)
		return
	}

	g := NewGraph()

	g.AddNode("oracle", func(ctx context.Context, name string, state State) (State, error) {
		if state == nil {
			return nil, errors.New("No state")
		}

		completion, err := llms.GenerateFromSinglePrompt(ctx,
			llm,
			state["input"].(string),
			llms.WithTemperature(0.0),
			llms.WithJSONMode(),
		)
		if err != nil {
			log.Fatal(err)
		}

		state["output"] = completion
		return state, nil
	})

	g.AddNode(END, func(_ context.Context, name string, state State) (State, error) {
		return state, nil
	})

	g.AddEdge("oracle", END)
	g.SetEntryPoint("oracle")

	runnable, err := g.Compile()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	res, err := runnable.Invoke(ctx, State{"input": "天空为什么是蓝的"})
	if err != nil {
		panic(err)
	}

	fmt.Println(res)
}
