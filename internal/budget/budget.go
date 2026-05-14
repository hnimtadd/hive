package budget

import (
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

// ContextBudget tracks context usage limits and determines when summarization is needed.
type ContextBudget struct {
	MaxMessages              int
	SummaryTriggerThreshold  int
	SummaryTargetTokens      int
	MaxTaskDescriptionTokens int
}

// NewContextBudget creates a new context budget from configuration.
func NewContextBudget(cfg config.ContextConfig) *ContextBudget {
	return &ContextBudget{
		MaxMessages:              cfg.MaxMessagesPerTask,
		SummaryTriggerThreshold:  cfg.SummaryTriggerThreshold,
		SummaryTargetTokens:      cfg.SummaryTargetTokens,
		MaxTaskDescriptionTokens: cfg.MaxTaskDescriptionTokens,
	}
}

// EstimateTokens estimates the token count for a slice of messages.
// Uses a heuristic: 1 token ≈ 4 characters (Claude's rough estimate).
func (cb *ContextBudget) EstimateTokens(msgs []types.Message) int {
	totalChars := 0
	for _, msg := range msgs {
		totalChars += len(msg.Content)
	}
	return totalChars / 4
}

// ShouldTriggerSummary determines if summarization should be triggered based on the task state.
// Returns true if:
//   - Message count >= SummaryTriggerThreshold
//   - Or estimated tokens exceed MaxTaskDescriptionTokens
func (cb *ContextBudget) ShouldTriggerSummary(task *types.Session) bool {
	if len(task.Messages) >= cb.SummaryTriggerThreshold {
		return true
	}

	estimatedTokens := cb.EstimateTokens(task.Messages)
	if estimatedTokens > cb.MaxTaskDescriptionTokens {
		return true
	}

	return false
}

// ExceedsTaskDescriptionLimit checks if a task JSON string exceeds the token limit.
func (cb *ContextBudget) ExceedsTaskDescriptionLimit(taskJSON string) bool {
	estimatedTokens := len(taskJSON) / 4
	return estimatedTokens > cb.MaxTaskDescriptionTokens
}
