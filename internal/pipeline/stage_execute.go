package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hnimtadd/hive/internal/bee/queen"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/pkg/types"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
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
	publisher := e.deps.EventBus.Publish(state.Task.ID)

	// Execute supervisor loop
	for {
		select {
		case <-state.Ctx.Done():
			return StageAbort, state.Ctx.Err()
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
				logger.Info("task completed")
				publisher <- agentv1.NewSessionEventTurnResponse(
					state.RunID,
					agentv1.NewTurnResponseSuccess(state.Task.ConversationID, state.RunID, state.RunID, output.Content),
				)
				return StageNext, nil

			case types.TaskStatusFailed:
				logger.Info("task failed")
				publisher <- agentv1.NewSessionEventTurnResponse(
					state.RunID,
					agentv1.NewTurnResponseFailed(state.Task.ConversationID, state.RunID, state.RunID, output.Content),
				)
				return StageNext, nil

			case types.TaskStatusPaused:
				// TODO: create a feedback coordinator here
				// IDEA:
				// - pipeline have a generic submit command
				// feedback submit will be Submit(shared.TypeFeedbackInput, Feedback{correlation_id, answer})
				publisher <- agentv1.NewSessionEventInputRequired(
					state.RunID,
					agentv1.NewInputRequired(state.Task.ConversationID, state.RunID, output.Content),
				)
				// wait for the feedback response
				// update state message with feedback response
				continue
			case types.TaskStatusInProgress:
				publisher <- agentv1.NewSessionEventTurnResponse(
					state.RunID,
					agentv1.NewTurnResponseUpdate(state.Task.ConversationID, state.RunID, state.RunID, output.Content),
				)
				continue
				// Continue to next iteration
			}
		}
	}

}

// Name implements [Stage].
func (e *ExecuteStage) Name() string { return "execute" }
