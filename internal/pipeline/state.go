package pipeline

import (
	"context"
	"time"

	"github.com/hnimtadd/hive/pkg/types"
)

// PipelineState is the shared mutabled state for the whole pipeline execution.
type PipelineState struct {
	Session *types.Session

	// Ctx holds enriched context after ContextStage. This context already have
	// task identity information injected, so inner agent could read from this.
	Ctx context.Context

	// Global state
	Iteration int
	ExitCode  StageResult
	RunID     string
}

// NewPipelineState creates a PipelineState with identity fields available.
func NewPipelineState(ctx context.Context, session *types.Session) *PipelineState {
	return &PipelineState{
		Session: session,
		Ctx:     ctx,
		RunID:   session.ID,
	}
}

type PipelineResult struct {
	RunID    string
	Output   string
	Duration time.Duration
}

func (p PipelineState) Result() *PipelineResult {
	return &PipelineResult{
		RunID: p.RunID,
	}
}
