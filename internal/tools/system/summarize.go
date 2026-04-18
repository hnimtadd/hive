package system

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/pkg/types"
)

// SummarizeTaskHistory uses an LLM to create a concise summary of task message history.
// It uses the Fast tier model for cost efficiency.
//
// Parameters:
//   - ctx: Context for the LLM call
//   - provider: LLM provider to use for summarization
//   - messages: Message history to summarize
//   - targetTokens: Desired summary length in tokens
//
// Returns:
//   - summary: Condensed summary string
//   - error: Any error encountered during summarization
func SummarizeTaskHistory(
	ctx context.Context,
	provider llm.Provider,
	messages []types.Message,
	targetTokens int,
) (string, error) {
	logger := observability.Logger(ctx)

	// Skip if too few messages
	if len(messages) < 3 {
		logger.DebugContext(ctx, "skipping summarization: too few messages",
			slog.Int("message_count", len(messages)),
		)
		return "", nil
	}

	// Get fast tier model for cost efficiency
	model, ok := provider.GetModel(ctx, llm.TierFast)
	if !ok {
		return "", fmt.Errorf("no fast tier model available for summarization")
	}

	// Format messages into readable text
	historyText := formatMessagesForSummarization(messages)

	// Estimate input tokens
	inputTokens := len(historyText) / 4
	logger.InfoContext(ctx, "starting task history summarization",
		slog.Int("message_count", len(messages)),
		slog.Int("estimated_input_tokens", inputTokens),
		slog.Int("target_summary_tokens", targetTokens),
	)

	// Create prompt
	prompt := fmt.Sprintf(`You are a technical summarization expert. Summarize this task execution history concisely.

Focus on:
1. What was accomplished (concrete results, not attempts)
2. Key decisions made (architectural choices, approach selected)
3. Current blockers or issues
4. Important artifacts created or modified

History to summarize:
%s

Provide a 3-4 sentence summary in past tense. Target length: ~%d tokens.`, historyText, targetTokens)

	// Call LLM
	llmMessages := []*schema.Message{
		schema.SystemMessage("You are a concise technical summarizer focused on task execution history."),
		schema.UserMessage(prompt),
	}

	result, err := model.Generate(ctx, llmMessages)
	if err != nil {
		logger.ErrorContext(ctx, "summarization LLM call failed",
			slog.Any("error", err),
		)
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	summary := result.Content
	outputTokens := len(summary) / 4
	tokenSavings := inputTokens - outputTokens

	logger.InfoContext(ctx, "task history summarized successfully",
		slog.Int("original_tokens", inputTokens),
		slog.Int("summary_tokens", outputTokens),
		slog.Int("tokens_saved", tokenSavings),
		slog.Float64("compression_ratio", float64(tokenSavings)/float64(inputTokens)),
	)

	return summary, nil
}

// formatMessagesForSummarization converts messages into a readable format for LLM summarization.
func formatMessagesForSummarization(messages []types.Message) string {
	var builder strings.Builder

	for i, msg := range messages {
		builder.WriteString(fmt.Sprintf("[Message %d - %s]\n", i+1, msg.Role))
		builder.WriteString(msg.Content)
		builder.WriteString("\n\n")
	}

	return builder.String()
}
