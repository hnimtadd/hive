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
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/tools/system"
	"github.com/hnimtadd/hive/internal/trace"
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
	Execute(ctx context.Context, task *types.HiveTask) (*QueenOutput, error)
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
	}, nil
}

// Execute implements [QueenBee].
func (s *queen) Execute(ctx context.Context, task *types.HiveTask) (*QueenOutput, error) {
	logger := trace.Logger(ctx)
	logger.InfoContext(ctx, "queen execution started",
		slog.String("queen_id", s.id),
		slog.String("task_id", task.ID),
		slog.String("task_status", string(task.Status)),
	)

	retryConfig := errors.RetryConfig{
		MaxAttempts:   s.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}
	taskDescription, err := task.JSONString()
	if err != nil {
		logger.ErrorContext(ctx, "failed to serialize task to JSON", slog.Any("error", err))
		return nil, fmt.Errorf("task could be tranlsated to JSON: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.config.TimeoutInSec)*time.Second)
	defer cancel()

	handler := errors.NewErrorHandler[*QueenOutput]()

	var reactAgent *react.Agent
	msgs := []*schema.Message{schema.UserMessage(taskDescription)}
	msg, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*QueenOutput, error) {
		trace.Logger(ctx).Debug("queen executing", slog.Int("message_count", len(msgs)))
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
			trace.Logger(ctx).Error("queen ReACT execution failed", slog.Any("error", execErr))
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

		content, err = hiveutils.HeristicallyExtractJSONString(content)
		if err != nil {
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not a valid JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output to agent output schema", err).
				WithContext("raw_output", result.Content)
		}

		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to map output to an object"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema: failed to parse output to agent output schema", err).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		if err = s.outputValidator.Validate(output); err != nil {
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, output is not follow schema"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err).
				WithContext("parsed_output", output).
				WithContext("extracted_content", content).
				WithContext("raw_output", result.Content)
		}

		agentOutput := QueenOutput{}
		if err = json.Unmarshal([]byte(content), &agentOutput); err != nil {
			msgs = append(msgs, schema.SystemMessage("invalid agent output schema, failed to parse output JSON"))
			return nil, errors.NewHiveError(errors.ErrTypeValidation, "invalid agent output schema", err).
				WithContext("content", content)
		}

		if len(result.ReasoningContent) > 0 {
			agentOutput.Thought += fmt.Sprintf("system thought: %s", result.ReasoningContent)
		}

		switch agentOutput.Status {
		case types.TaskStatusCompleted,
			types.TaskStatusFailed,
			types.TaskStatusInProgress,
			types.TaskStatusPaused:
			task.Status = agentOutput.Status

		default:
			msgs = append(
				msgs,
				schema.UserMessage(
					fmt.Sprintf(
						"invalid task output status: %s. Valid statuses are: 'in_progress' (need another execution cycle), 'paused' (need user input), 'completed' (task done), 'failed' (cannot complete)",
						agentOutput.Status,
					),
				),
			)
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

Status Selection Guidelines - Choose the appropriate status for each response:

	1. "in_progress": Use this when you completed one execution cycle but need to continue in the next cycle.
	   - You delegated to an agent and received results, but need to delegate to another agent or do more work
	   - You gathered some information but need additional steps to complete the task
	   - You made progress toward the goal but it's not yet complete
	   - Set "content" to describe what you just accomplished (e.g., "Received search results from agent X, now analyzing...")
	   - The system will immediately call you again to continue - your next invocation will have access to the tool results from this cycle
	   - DO NOT use this when you need user input - use "paused" instead

	2. "paused": Use this ONLY when you need information or clarification from the user before you can proceed.
	   - The task requirements are ambiguous and you cannot proceed without clarification
	   - You need the user to make a decision between multiple valid approaches
	   - You require additional context that only the user can provide (not available through any agent)
	   - Set "content" to your question for the user
	   - The system will WAIT for user feedback, then call you again with their response

	3. "completed": Use this when the user's goal is fully achieved.
	   - All task requirements have been met and no further work is needed
	   - Set "content" to a summary of what was accomplished and the final results

	4. "failed": Use this when the task cannot be completed.
	   - Available agents lack the necessary capabilities to fulfill the request
	   - A logical dead-end is reached and there's no path forward
	   - Set "content" to explain why the task cannot be completed

Constraint: Do not perform the task yourself. Your only tools are delegation and synthesis.
`
	_ = agents // TODO: Include agent descriptions in persona
	return persona
}
