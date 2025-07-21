package agent

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
)

func TestBasicLoop(t *testing.T) {
	loop := NewAgentLoop(
		WithMaxIterations(5),
		WithDelay(1*time.Second),
		WithIterationCallbacks(
			func(ctx context.Context, iteration int, state []llms.MessageContent) {
				log.Printf("Start %d iteration", iteration+1)
			},
			func(ctx context.Context, iteration int, state []llms.MessageContent, err error) {
				if err != nil {
					log.Printf("Fail %d iteration: %v", iteration+1, err)
				} else {
					log.Printf("Success %d iteration", iteration+1)
				}
			},
		),
		WithLoopCompleteCallback(func(ctx context.Context, totalIterations int, finalState []llms.MessageContent) {
			log.Printf("Finish total %d iteration", totalIterations)
		}),
	)

	initialState := []llms.MessageContent{
		{
			Role: "user",
			Parts: []llms.ContentPart{
				llms.TextContent{Text: "start task"},
			},
		},
	}

	agentFunc := func(ctx context.Context, state []llms.MessageContent) ([]llms.MessageContent, error) {
		newMessage := llms.MessageContent{
			Role: "assistant",
			Parts: []llms.ContentPart{
				llms.TextContent{Text: fmt.Sprintf("Processing message, current time: %s", time.Now().Format("15:04:05"))},
			},
		}

		state = append(state, newMessage)
		return state, nil
	}

	ctx := context.Background()
	result := loop.Execute(ctx, initialState, agentFunc)

	if result.Error != nil {
		log.Printf("Loop execution failed: %v", result.Error)
	} else {
		log.Printf("Loop execution successful, time-consuming: %v", result.Duration)
		for i, msg := range result.FinalState {
			log.Printf("Message %d: %s", i+1, msg.Parts[0].(llms.TextContent).Text)
		}
	}
}
