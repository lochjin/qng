package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type LoopCondition func(ctx context.Context, state []llms.MessageContent, iteration int) (bool, error)

type LoopConfig struct {
	MaxIterations int

	Timeout time.Duration

	DelayBetweenIterations time.Duration

	Condition LoopCondition

	OnIterationStart func(ctx context.Context, iteration int, state []llms.MessageContent)

	OnIterationEnd func(ctx context.Context, iteration int, state []llms.MessageContent, err error)

	OnLoopComplete func(ctx context.Context, totalIterations int, finalState []llms.MessageContent)
}

type LoopOption func(*LoopConfig)

func WithMaxIterations(max int) LoopOption {
	return func(config *LoopConfig) {
		config.MaxIterations = max
	}
}

func WithTimeout(timeout time.Duration) LoopOption {
	return func(config *LoopConfig) {
		config.Timeout = timeout
	}
}

func WithDelay(delay time.Duration) LoopOption {
	return func(config *LoopConfig) {
		config.DelayBetweenIterations = delay
	}
}

func WithCondition(condition LoopCondition) LoopOption {
	return func(config *LoopConfig) {
		config.Condition = condition
	}
}

func WithIterationCallbacks(
	onStart func(ctx context.Context, iteration int, state []llms.MessageContent),
	onEnd func(ctx context.Context, iteration int, state []llms.MessageContent, err error),
) LoopOption {
	return func(config *LoopConfig) {
		config.OnIterationStart = onStart
		config.OnIterationEnd = onEnd
	}
}

func WithLoopCompleteCallback(callback func(ctx context.Context, totalIterations int, finalState []llms.MessageContent)) LoopOption {
	return func(config *LoopConfig) {
		config.OnLoopComplete = callback
	}
}

type AgentLoop struct {
	config LoopConfig
}

func NewAgentLoop(opts ...LoopOption) *AgentLoop {
	config := LoopConfig{
		MaxIterations:          0,
		Timeout:                0,
		DelayBetweenIterations: 0,
	}

	for _, opt := range opts {
		opt(&config)
	}

	return &AgentLoop{
		config: config,
	}
}

type LoopResult struct {
	TotalIterations int
	FinalState      []llms.MessageContent
	Error           error
	Duration        time.Duration
}

func (al *AgentLoop) Execute(
	ctx context.Context,
	initialState []llms.MessageContent,
	agentFunc func(ctx context.Context, state []llms.MessageContent) ([]llms.MessageContent, error),
) *LoopResult {
	startTime := time.Now()

	if al.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, al.config.Timeout)
		defer cancel()
	}

	state := initialState
	iteration := 0

	for {
		if al.config.MaxIterations > 0 && iteration >= al.config.MaxIterations {
			break
		}

		select {
		case <-ctx.Done():
			return &LoopResult{
				TotalIterations: iteration,
				FinalState:      state,
				Error:           ctx.Err(),
				Duration:        time.Since(startTime),
			}
		default:
		}

		if al.config.OnIterationStart != nil {
			al.config.OnIterationStart(ctx, iteration, state)
		}

		var err error
		state, err = agentFunc(ctx, state)

		if al.config.OnIterationEnd != nil {
			al.config.OnIterationEnd(ctx, iteration, state, err)
		}

		if err != nil {
			return &LoopResult{
				TotalIterations: iteration,
				FinalState:      state,
				Error:           fmt.Errorf("iteration %d failed: %w", iteration, err),
				Duration:        time.Since(startTime),
			}
		}

		iteration++

		if al.config.Condition != nil {
			shouldContinue, err := al.config.Condition(ctx, state, iteration)
			if err != nil {
				return &LoopResult{
					TotalIterations: iteration,
					FinalState:      state,
					Error:           fmt.Errorf("condition check failed: %w", err),
					Duration:        time.Since(startTime),
				}
			}
			if !shouldContinue {
				break
			}
		}

		if al.config.DelayBetweenIterations > 0 {
			select {
			case <-time.After(al.config.DelayBetweenIterations):
			case <-ctx.Done():
				return &LoopResult{
					TotalIterations: iteration,
					FinalState:      state,
					Error:           ctx.Err(),
					Duration:        time.Since(startTime),
				}
			}
		}
	}

	result := &LoopResult{
		TotalIterations: iteration,
		FinalState:      state,
		Duration:        time.Since(startTime),
	}

	if al.config.OnLoopComplete != nil {
		al.config.OnLoopComplete(ctx, iteration, state)
	}

	return result
}

func ContinueUntilMaxIterations(maxIterations int) LoopCondition {
	return func(ctx context.Context, state []llms.MessageContent, iteration int) (bool, error) {
		return iteration < maxIterations, nil
	}
}

func ContinueUntilCondition(condition func([]llms.MessageContent) bool) LoopCondition {
	return func(ctx context.Context, state []llms.MessageContent, iteration int) (bool, error) {
		return condition(state), nil
	}
}

func ContinueUntilTimeout(duration time.Duration) LoopCondition {
	startTime := time.Now()
	return func(ctx context.Context, state []llms.MessageContent, iteration int) (bool, error) {
		return time.Since(startTime) < duration, nil
	}
}

func ContinueUntilMessageContains(target string) LoopCondition {
	return func(ctx context.Context, state []llms.MessageContent, iteration int) (bool, error) {
		for _, msg := range state {
			for _, part := range msg.Parts {
				if text, ok := part.(llms.TextContent); ok {
					if strings.Contains(text.Text, target) {
						return false, nil
					}
				}
			}
		}
		return true, nil
	}
}

func ContinueWhileMessageContains(target string) LoopCondition {
	return func(ctx context.Context, state []llms.MessageContent, iteration int) (bool, error) {
		for _, msg := range state {
			for _, part := range msg.Parts {
				if text, ok := part.(llms.TextContent); ok {
					if strings.Contains(text.Text, target) {
						return true, nil
					}
				}
			}
		}
		return false, nil
	}
}
