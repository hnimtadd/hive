package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hnimtadd/hive/internal/bee/queen"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/pkg/types"
)

type ExecuteStage struct {
	deps PipelineDependencies
}

var _ Stage = &ExecuteStage{}

func NewExecuteStage() *ExecuteStage {
	return &ExecuteStage{}
}

// Execute implements [Stage].
func (e *ExecuteStage) Execute(ctx context.Context, state *PipelineState) (StageResult, error) {
	logger := observability.Logger(state.Ctx)
	supervisor, err := queen.NewQueenBee(state.Task.ID, 10, e.deps.Registry, e.deps.Config.Server.MaxTimeout, e.deps.Provider)
	// publisher := e.deps.EventBus.Publish(state.Task.ID)

	// Execute supervisor loop
	for {
		select {
		case <-state.Ctx.Done():
		case <-ctx.Done():
			logger.Info("context cancelled during execution")
			return StageAbort, fmt.Errorf("context cancelled during execution: %w", ctx.Err())

		default:
			// Execute supervisor iteration
			var output *queen.QueenOutput
			output, err = supervisor.Execute(ctx, state.Task)
			if err != nil {
				logger.Error("supervisor execution failed", slog.Any("error", err))
				return StageAbort, fmt.Errorf("failed to execute supervisor: %w", err)
			}

			// TODO: update state state based on output

			// Send update to client
			switch output.Status {
			case types.TaskStatusCompleted:
				// publisher <-  Task Response Success event
				logger.Info("task completed")
				return StageNext, nil

			case types.TaskStatusFailed:
				// publisher <-  Task Response Success event
				logger.Info("task failed")
				return StageNext, nil

			case types.TaskStatusPaused:
				// publisher <- Task Feedback required, and wait for feedback resposne from user
				// update state message with feedback response
				continue
			case types.TaskStatusInProgress:
				// publisher <- task update
				logger.Info("task in progress")
				continue
				// Continue to next iteration
			}
		}
	}

}

// Name implements [Stage].
func (e *ExecuteStage) Name() string { return "execute" }
