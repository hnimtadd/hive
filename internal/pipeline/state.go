package pipeline

import (
	"context"

	"github.com/hnimtadd/hive/pkg/types"
)

// PipelineState is the shared mutabled state for the whole pipeline execution.
type PipelineState struct {
	Task *types.HiveTask

	// Ctx holds enriched context after ContextStage. This context already have
	// task identity information injected, so inner agent could read from this.
	Ctx context.Context

	ExitCode StageResult
	RunID    string
}

// NewPipelineState creates a PipelineState with identity fields available.
func NewPipelineState(ctx context.Context, task *types.HiveTask) *PipelineState {
	return &PipelineState{
		Task:  task,
		Ctx:   ctx,
		RunID: task.ID,
	}
}
