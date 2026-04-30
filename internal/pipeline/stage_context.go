package pipeline

import (
	"context"

	"github.com/hnimtadd/hive/internal/budget"
	"github.com/hnimtadd/hive/internal/middleware"
	"github.com/hnimtadd/hive/internal/middleware/system"
	"github.com/hnimtadd/hive/internal/observability"
)

type ContextStage struct {
	deps *PipelineDependencies
}

var _ Stage = &ContextStage{}

func NewContextStage() *ContextStage {
	return &ContextStage{}
}

// Execute implements [Stage].
// ContextStage populates context into the pipeline state
func (c *ContextStage) Execute(ctx context.Context, state *PipelineState) (StageResult, error) {
	ctx = observability.ContextWithTraceContext(ctx, observability.NewRootTraceContext())

	eventBusTopic := c.deps.EventBus.Publish(state.Task.SessionID)
	ctx = middleware.ContextWithMiddleware(ctx, middleware.JointMiddleware(
		system.EventStreamMiddleware(eventBusTopic),
		observability.NewTraceMiddleware(c.deps.SessionLogger),
	))
	ctx = budget.ContextWithBudget(ctx, budget.NewContextBudget(c.deps.Config.AI.Context))

	state.Ctx = ctx
	return StageNext, nil
}

// Name implements [Stage].
func (c *ContextStage) Name() string { return "context" }
