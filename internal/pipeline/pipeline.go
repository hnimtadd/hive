package pipeline

import (
	"context"
	"fmt"
	"time"
)

type Pipeline struct {
	pre       []Stage
	iteration []Stage

	deps PipelineDependencies
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		pre:       []Stage{NewContextStage()},
		iteration: []Stage{NewExecuteStage()},
	}
}

// Execute executes the full pipeline for a single agent run.
func (p *Pipeline) Execute(ctx context.Context, state *PipelineState) (*PipelineResult, error) {
	start := time.Now()
	for _, stage := range p.pre {
		stage.Execute(ctx, state)
	}

	// Propagate enriched context from pre stages
	if state.Ctx != nil {
		ctx = state.Ctx
	}

	// 2. Iteration loop.
	// StageContinue: Continue the current stage iteration, (in iteration context, this mean, continue the iteration)
	// StageNext: go to next stage
	// StageAbort: Stop entire run
	for state.Iteration = 0; state.Iteration < p.deps.Config.AI.MaxStep; state.Iteration++ {
		for _, stage := range p.iteration {
			result, err := stage.Execute(ctx, state)
			if err != nil {
				return nil, fmt.Errorf("iter %d %s: %w", state.Iteration, stage.Name(), err)
			}

			switch result {
			case StageAbort:
				state.ExitCode = StageAbort
				break
			case StageNext:
				state.ExitCode = StageNext
				break
			case StageContinue:
				continue
			}
		}

		// Check if we need to exit the execution stages.
		if state.ExitCode == StageAbort || state.ExitCode == StageNext {
			break
		}

		// If the context is cancelled, then we cancel the pipeline also
		if ctx.Err() != nil {
			state.ExitCode = StageAbort
			break
		}
	}

	result := state.Result()
	result.Duration = time.Since(start)
	return result, nil
}
