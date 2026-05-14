package queen

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/bee/react"
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/budget"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/tools/system"
	"github.com/hnimtadd/hive/pkg/errors"
	"github.com/hnimtadd/hive/pkg/types"
	hiveutils "github.com/hnimtadd/hive/pkg/utils"
)

type QueenBee interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// Description return a short self-description about agent capabilities.
	Description() string

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *types.Session) (*QueenOutput, error)
}

type QueenOutput struct {
	Status     types.Status `json:"status"                jsonschema:"Task status - 'in_progress': completed this cycle, need another cycle to continue; 'paused': need user input before next cycle; 'completed': task finished successfully; 'failed': task cannot be completed"`
	Content    string       `json:"content"               jsonschema:"For in_progress: what you accomplished this cycle. For paused: question for user. For completed: final summary of results. For failed: reason for failure"`
	NextAction string       `json:"next_action,omitempty" jsonschema:"What you plan to do in the next execution cycle (primarily for in_progress status)"`
	Thought    string       `json:"-"`
}

type queen struct {
	id string

	outputValidator *jsonschema.Resolved

	config   *bee.Config
	registry registry.Registry
	provider llm.Provider
}

func NewQueenBee(
	agentID string,
	maxStep int,
	registry registry.Registry,
	timeout time.Duration,
	provider llm.Provider,
) (QueenBee, error) {
	// Create delegate and explore tools
	delegateTool, err := system.DelegateTool()
	if err != nil {
		return nil, fmt.Errorf("worker: failed to create delegate tool: %w", err)
	}

	exploreTool, err := system.ExploreTool(provider)
	if err != nil {
		return nil, fmt.Errorf("worker: failed to create explore tool: %w", err)
	}

	schema, err := jsonschema.For[QueenOutput](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build agent output schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve agent schema: %w", err)
	}

	config := &bee.Config{
		ID:           agentID,
		MaxSteps:     maxStep,
		TimeoutInSec: int(timeout.Seconds()),
		ModelPool:    provider.ModelPool(llm.TierSmart),
		Tools:        []tool.InvokableTool{delegateTool, exploreTool},
		Persona:      buildPersona(registry),
	}

	return &queen{
		id:              agentID,
		outputValidator: resolved,
		config:          config,
		registry:        registry,
		provider:        provider,
	}, nil
}

// Execute implements [QueenBee].
func (s *queen) Execute(ctx context.Context, task *types.Session) (*QueenOutput, error) {
	logger := observability.Logger(ctx)
	logger.InfoContext(ctx, "queen execution started",
		slog.String("queen_id", s.id),
		slog.String("task_id", task.ID),
		slog.String("task_status", string(task.Status)),
	)

	// Check context budget and trigger summarization if needed
	if budget, ok := budget.BudgetFromContext(ctx); ok {
		if budget.ShouldTriggerSummary(task) {
			logger.InfoContext(ctx, "context budget exceeded, triggering summarization",
				slog.Int("message_count", len(task.Messages)),
				slog.Int("threshold", budget.SummaryTriggerThreshold),
			)

			// Summarize all but last 3 messages
			messagesToSummarize := task.Messages
			recentMessages := []types.Message{}
			if len(task.Messages) > 3 {
				messagesToSummarize = task.Messages[:len(task.Messages)-3]
				recentMessages = task.Messages[len(task.Messages)-3:]
			}

			summary, err := system.SummarizeTaskHistory(
				ctx,
				s.provider,
				messagesToSummarize,
				budget.SummaryTargetTokens,
			)

			if err != nil {
				logger.WarnContext(ctx, "summarization failed, continuing without summary",
					slog.Any("error", err),
				)
			} else {
				// Update task with summary
				task.Summary = summary
				task.Messages = recentMessages

				logger.InfoContext(ctx, "context summarized successfully",
					slog.Int("original_message_count", len(messagesToSummarize)),
					slog.Int("remaining_message_count", len(recentMessages)),
					slog.Int("summary_length", len(summary)),
				)
			}
		}
	}

	retryConfig := errors.RetryConfig{
		MaxAttempts:   s.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := task.CompactJSONString()
	if err != nil {
		logger.ErrorContext(ctx, "failed to serialize task to JSON", slog.Any("error", err))
		return nil, fmt.Errorf("task could be tranlsated to JSON: %w", err)
	}

	// Log context size metrics
	estimatedTokens := len(taskDescription) / 4
	logger.InfoContext(ctx, "context_size_estimate",
		slog.Int("message_count", len(task.Messages)),
		slog.Int("task_json_bytes", len(taskDescription)),
		slog.Int("estimated_tokens", estimatedTokens),
		slog.Bool("has_summary", task.Summary != ""),
	)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.config.TimeoutInSec)*time.Second)
	defer cancel()

	handler := errors.NewErrorHandler[*QueenOutput]()

	var reactAgent *react.Agent
	msgs := []*schema.Message{schema.UserMessage(taskDescription)}
	msg, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*QueenOutput, error) {
		observability.Logger(ctx).Debug("queen executing", slog.Int("message_count", len(msgs)))
		reactAgent, err = react.New(react.Config{
			ID:           s.config.ID,
			ChatModel:    s.config.ModelPool(),
			Tools:        s.config.Tools,
			SystemPrompt: s.config.Persona,
			MaxStep:      s.config.MaxSteps,
		})
		if err != nil {
			return nil, fmt.Errorf("queen: failed to init ReACT agent: %w", err)
		}
		// Execute the task using the ReACT agent
		result, execErr := reactAgent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
			observability.Logger(ctx).Error("queen ReACT execution failed", slog.Any("error", execErr))
			return nil, execErr
		}

		content := func() string {
			if len(result.MultiContent) != 0 {
				for content := range slices.Values(result.MultiContent) {
					switch content.Type {
					case schema.ChatMessagePartTypeText:
						return content.Text
						// process agent output update
					default:
						continue
					}
				}
			}
			return result.Content
		}()
		msgs = append(msgs, result)

		content, err = hiveutils.HeuristicallyExtractJSONString(content)
		if err != nil {
			errorMsg := fmt.Sprintf(`ERROR: Your output is not valid JSON. You must respond with ONLY a JSON object, nothing else.

Your output was:
%s

Required format:
{"status": "in_progress|paused|completed|failed", "content": "description", "next_action": "optional next step"}

Example valid response:
{"status": "in_progress", "content": "Delegated search to explore agent", "next_action": "Will analyze results"}

Please try again with ONLY the JSON object, no markdown, no explanations.`, result.Content)
			msgs = append(msgs, schema.SystemMessage(errorMsg))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output to agent output schema", err).
				WithContext("raw_output", result.Content)
		}

		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			errorMsg := fmt.Sprintf(`ERROR: Your JSON is malformed and cannot be parsed.

Your JSON:
%s

Error: %s

Required format:
{"status": "in_progress", "content": "your message", "next_action": "optional"}

Please ensure:
- All strings are in double quotes
- No trailing commas
- Proper JSON syntax`, content, err.Error())
			msgs = append(msgs, schema.SystemMessage(errorMsg))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output to agent output schema", err).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		if err = s.outputValidator.Validate(output); err != nil {
			errorMsg := fmt.Sprintf(`ERROR: Your JSON doesn't match the required schema.

Your output:
%s

Validation error: %s

Required fields:
- "status": Must be one of: "in_progress", "paused", "completed", "failed"
- "content": String describing what happened or what you need (required)
- "next_action": String describing next step (optional, recommended for "in_progress")

Example valid outputs:
{"status": "in_progress", "content": "Retrieved data from API", "next_action": "Will process and analyze"}
{"status": "completed", "content": "Task completed successfully. Created 3 files and ran tests."}
{"status": "paused", "content": "Which approach should I use: A or B?"}
{"status": "failed", "content": "Cannot proceed: missing required API credentials"}

Please try again with the correct format.`, content, err.Error())
			msgs = append(msgs, schema.SystemMessage(errorMsg))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err).
				WithContext("parsed_output", output).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		agentOutput := QueenOutput{}
		if err = json.Unmarshal([]byte(content), &agentOutput); err != nil {
			errorMsg := fmt.Sprintf(`ERROR: JSON structure is valid but field types are incorrect.

Your JSON:
%s

Error: %s

Correct field types:
- "status": string (one of: "in_progress", "paused", "completed", "failed")
- "content": string (not number, not boolean)
- "next_action": string or omit (not required)

Ensure all values are properly quoted strings.`, content, err.Error())
			msgs = append(msgs, schema.SystemMessage(errorMsg))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err).
				WithContext("content", content)
		}

		if len(result.ReasoningContent) > 0 {
			agentOutput.Thought += fmt.Sprintf("system thought: %s", result.ReasoningContent)
		}

		switch agentOutput.Status {
		case types.SessionStatusCompleted,
			types.SessionStatusFailed,
			types.SessionStatusInProgress,
			types.SessionStatusPaused:
			task.Status = agentOutput.Status

		default:
			errorMsg := fmt.Sprintf(`ERROR: Invalid status value "%s"

Valid status values are:
- "in_progress": You completed this cycle but need to continue in the next cycle
- "paused": You need user input or clarification before proceeding
- "completed": The task is fully done and successful
- "failed": The task cannot be completed

Your current output:
{"status": "%s", "content": "%s"}

Please respond with one of the valid status values.`, agentOutput.Status, agentOutput.Status, agentOutput.Content)
			msgs = append(msgs, schema.SystemMessage(errorMsg))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid task status", nil)
		}
		return &agentOutput, nil
	})

	if err != nil {
		logger.ErrorContext(ctx, "queen execution failed",
			slog.String("task_id", task.ID),
			slog.Any("error", err),
		)
	} else {
		logger.InfoContext(ctx, "queen execution completed",
			slog.String("task_id", task.ID),
			slog.String("status", string(msg.Status)),
		)
	}

	return msg, err
}

// Description implements [QueenBee].
func (s *queen) Description() string {
	return s.config.Description
}

// GetID implements [QueenBee].
func (s *queen) GetID() string {
	return s.id
}

// buildPersona builds the system prompt for the supervisor.
// This is copied from server.go - should be extracted to a shared function.
func buildPersona(reg registry.Registry) string {
	agents := reg.ListAgents()
	persona := `
Role: You are the Central Orchestrator for a multi-agent swarm. Your goal is to navigate a complex task to completion by delegating to specialized workers.

Core Responsibilities:
	- Analyze State: Review the task's "message" field which contains the full conversation history, including your previous progress updates and any user feedback. Identify what has been achieved and what is still missing.
    - Prevent Redundancy: If a supervisee has already failed at a specific approach, do not assign them the same task again without new instructions.
    - Evaluate Capabilities: Match the requirements of the next step against the specific tools and expertise of the available agents.
	- Delegate and coordinate: Use available tools to delegate work to specialized agents.
	- Context Awareness: Always check the "message" field in the task to see what was previously accomplished and what the user has said. This helps you avoid repeating work or asking the same questions.

CRITICAL OUTPUT FORMAT REQUIREMENT:
You MUST respond with ONLY a valid JSON object. NO markdown, NO explanations, NO extra text before or after the JSON.

Required JSON Schema:
{
  "status": "in_progress" | "paused" | "completed" | "failed",
  "content": "string describing what happened or what you need",
  "next_action": "string describing what you'll do next (optional, mainly for in_progress)"
}

Status Selection Guidelines:

	1. "in_progress": Use this when you completed one execution cycle but need to continue in the next cycle.
	   - You delegated to an agent and received results, but need to delegate to another agent or do more work
	   - You gathered some information but need additional steps to complete the task
	   - You made progress toward the goal but it's not yet complete
	   - Set "content" to describe what you just accomplished (e.g., "Received search results from agent X, now analyzing...")
	   - The system will immediately call you again to continue - your next invocation will have access to the tool results from this cycle
	   - DO NOT use this when you need user input - use "paused" instead
	   Example: {"status": "in_progress", "content": "Delegated file search to explore agent, received 15 matching files", "next_action": "Will analyze the files and identify the bug location"}

	2. "paused": Use this ONLY when you need information or clarification from the user before you can proceed.
	   - The task requirements are ambiguous and you cannot proceed without clarification
	   - You need the user to make a decision between multiple valid approaches
	   - You require additional context that only the user can provide (not available through any agent)
	   - Set "content" to your question for the user
	   - The system will WAIT for user feedback, then call you again with their response
	   Example: {"status": "paused", "content": "Should I use approach A (faster but less reliable) or approach B (slower but more thorough)?"}

	3. "completed": Use this when the user's goal is fully achieved.
	   - All task requirements have been met and no further work is needed
	   - Set "content" to a summary of what was accomplished and the final results
	   Example: {"status": "completed", "content": "Successfully fixed the bug in authentication.go:42. The issue was a null pointer dereference. Applied fix and verified with tests."}

	4. "failed": Use this when the task cannot be completed.
	   - Available agents lack the necessary capabilities to fulfill the request
	   - A logical dead-end is reached and there's no path forward
	   - Set "content" to explain why the task cannot be completed
	   Example: {"status": "failed", "content": "Cannot proceed: no agents available with database access capabilities, and this task requires direct database queries."}

Constraint: Do not perform the task yourself. Your only tools are delegation and synthesis.

REMINDER: Output ONLY the JSON object. Do not include markdown code blocks, explanations, or any other text.
`
	_ = agents // TODO: Include agent descriptions in persona
	return persona
}
