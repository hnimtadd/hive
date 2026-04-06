package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/bee/react"
	"github.com/hnimtadd/hive/internal/trace"
	hiveerrors "github.com/hnimtadd/hive/pkg/errors"
	hiveutils "github.com/hnimtadd/hive/pkg/utils"
)

type ThoroughnessLevel string

const (
	ThoroughnessQuick        ThoroughnessLevel = "quick"
	ThoroughnessMedium       ThoroughnessLevel = "medium"
	ThoroughnessVeryThorough ThoroughnessLevel = "very thorough"
)

type ExploreInput struct {
	Task         string            `json:"task"                   jsonschema:"The exploration task or question"`
	Context      string            `json:"context,omitempty"      jsonschema:"Optional context about what you're looking for"`
	Thoroughness ThoroughnessLevel `json:"thoroughness,omitempty" jsonschema:"Search thoroughness: 'quick', 'medium', or 'very thorough' (default: quick)"`
}

// ExploreOutput defines output from the Explore agent.
type ExploreOutput struct {
	Summary      string          `json:"summary"                 jsonschema:"Brief summary of findings"`
	KeyFiles     []FileReference `json:"key_files"               jsonschema:"List of relevant files with descriptions"`
	Patterns     string          `json:"patterns,omitempty"      jsonschema:"Identified patterns or structures"`
	CodeSnippets []CodeSnippet   `json:"code_snippets,omitempty" jsonschema:"Relevant code excerpts"`
	Notes        string          `json:"notes,omitempty"         jsonschema:"Additional observations or architectural notes"`
}

// FileReference represents a file with location info.
type FileReference struct {
	Path        string `json:"path"                  jsonschema:"File path"`
	LineNumber  int    `json:"line_number,omitempty" jsonschema:"Optional line number"`
	Description string `json:"description"           jsonschema:"What this file contains or does"`
}

// CodeSnippet represents a code excerpt.
type CodeSnippet struct {
	File     string `json:"file"               jsonschema:"Source file path"`
	Language string `json:"language,omitempty" jsonschema:"Programming language"`
	Code     string `json:"code"               jsonschema:"Code content"`
}

// ExploreBee is a specialized read-only agent for codebase exploration.
type ExploreBee interface {
	Explore(ctx context.Context, input *ExploreInput) (*ExploreOutput, error)
}

type exploreBee struct {
	id              string
	description     string
	agent           *react.Agent
	config          *bee.Config
	outputValidator *jsonschema.Resolved
}

// NewExploreBee creates a new Explore agent with read-only tools.
func NewExploreBee(config *bee.Config, agentOpts ...react.AgentOption) (ExploreBee, error) {
	// Filter tools to only allow read operations
	readOnlyTools := filterReadOnlyTools(config.Tools)
	if len(readOnlyTools) == 0 {
		return nil, errors.New("no read-only tools available")
	}

	systemPrompt := getExploreSystemPrompt()

	reactAgent, err := react.NewWithSystemPrompt(
		config.ID,
		config.LLM,
		readOnlyTools,
		systemPrompt,
		config.MaxSteps,
		agentOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init explore ReACT agent: %w", err)
	}

	schema, err := jsonschema.For[ExploreOutput](nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build explore output schema: %w", err)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve explore schema: %w", err)
	}

	return &exploreBee{
		id:              config.ID,
		agent:           reactAgent,
		description:     "Fast read-only agent optimized for searching and analyzing codebases",
		config:          config,
		outputValidator: resolved,
	}, nil
}

// filterReadOnlyTools filters tools to only include safe read-only operations.
func filterReadOnlyTools(tools []tool.InvokableTool) []tool.InvokableTool {
	readOnlyToolNames := map[string]bool{
		"glob":      true,
		"grep":      true,
		"file_read": true,
	}

	filtered := make([]tool.InvokableTool, 0)
	for _, t := range tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		if readOnlyToolNames[info.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// getExploreSystemPrompt returns the system prompt for the Explore agent.
func getExploreSystemPrompt() string {
	inputDesc, _ := hiveutils.DescribeJSONSchema[ExploreInput]()
	outputDesc, _ := hiveutils.DescribeJSONSchema[ExploreOutput]()

	return fmt.Sprintf(`You are a specialized codebase exploration agent optimized for fast, read-only analysis.

## Your Mission
Search and analyze codebases efficiently to answer questions about:
- File locations and structure
- Code patterns and implementations
- Architecture and dependencies
- Function/class definitions
- API endpoints and interfaces
- Configuration files

## Available Tools
You have **read-only** access to:
1. **glob** - Find files by pattern (wildcards: *.go, **/*.ts)
2. **grep** - Search file contents (regex, recursive, case-insensitive options)
3. **file_read** - Read file contents (can read partial content for large files)

## Thoroughness Levels
Adjust your search strategy based on the thoroughness level:

**Quick** (default): Targeted searches, limit results (20-30), read key files only
**Medium**: Broader patterns, more results (50-100), read multiple related files
**Very Thorough**: Comprehensive search, maximum results (100+), deep analysis

## Search Strategy
1. **Start with glob** to find relevant files by name/pattern
2. **Use grep** to search content and find patterns
3. **Read files** to analyze and extract details
4. **Limit results**: Don't read 100+ files, summarize patterns instead

## Important Constraints
- **READ ONLY**: You cannot write, edit, execute, or use bash
- **ANALYSIS ONLY**: Your output should be information, not actions
- **SPEED MATTERS**: Be efficient, leverage your fast model
- **CONTEXT AWARE**: Parse the thoroughness level from input

## Output Format
Provide structured findings:
- Summary: Brief overview
- Key Files: List with paths, line numbers, descriptions
- Patterns: Identified structures or conventions
- Code Snippets: Relevant excerpts with context
- Notes: High-level architectural observations

========= INPUT FORMAT ========
%s

========= OUTPUT FORMAT ========
YOU MUST RESPOND WITH A RAW JSON THAT FOLLOWS THIS SCHEMA:
%s

Remember: You are an explorer, not a builder. Discover, analyze, and report—never modify or execute.`, inputDesc, outputDesc)
}

// Explore performs codebase exploration.
func (e *exploreBee) Explore(ctx context.Context, input *ExploreInput) (*ExploreOutput, error) {
	logger := trace.Logger(ctx)

	retryConfig := hiveerrors.RetryConfig{
		MaxAttempts:   e.config.MaxSteps,
		InitialDelay:  500,
		BackoffFactor: 2.0,
		MaxDelay:      5000,
	}

	taskDescription, err := json.Marshal(input)
	if err != nil {
		logger.ErrorContext(ctx, "failed to marshal explore input", slog.Any("error", err))
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(e.config.TimeoutInSec)*time.Second)
	defer cancel()

	handler := hiveerrors.NewErrorHandler[*ExploreOutput]()
	msgs := []*schema.Message{schema.UserMessage(string(taskDescription))}

	output, err := handler.WithRetry(ctx, retryConfig, func(ctx context.Context) (*ExploreOutput, error) {
		trace.Logger(ctx).Debug("explore agent executing", slog.String("agent_id", e.id))

		result, execErr := e.agent.ExecuteWithMessages(ctx, msgs)
		if execErr != nil {
			trace.Logger(ctx).Error("explore ReACT execution failed", slog.Any("error", execErr))
			return nil, execErr
		}

		// Extract content from result
		content := func() string {
			if len(result.MultiContent) != 0 {
				for content := range slices.Values(result.MultiContent) {
					switch content.Type {
					case schema.ChatMessagePartTypeText:
						return content.Text
					default:
						continue
					}
				}
			}
			return result.Content
		}()

		trace.Logger(ctx).Debug("explore output received", slog.Int("content_length", len(content)))
		msgs = append(msgs, result)

		// Parse JSON output
		content, err = hiveutils.HeristicallyExtractJSONString(content)
		if err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("output is not valid JSON: %s", err)))
			return nil, hiveerrors.NewHiveError(hiveerrors.ErrTypeValidation, "failed to parse explore output", err)
		}

		var output map[string]any
		if err = json.Unmarshal([]byte(content), &output); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, hiveerrors.NewHiveError(hiveerrors.ErrTypeValidation, "failed to parse explore output", err)
		}

		if err = e.outputValidator.Validate(output); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("output does not follow JSON schema: %s", err)))
			return nil, hiveerrors.NewHiveError(hiveerrors.ErrTypeValidation, "failed to validate explore output schema", err)
		}

		exploreOutput := ExploreOutput{}
		if err = json.Unmarshal([]byte(content), &exploreOutput); err != nil {
			msgs = append(msgs, schema.UserMessage(fmt.Sprintf("invalid JSON output: %s", err)))
			return nil, hiveerrors.NewHiveError(hiveerrors.ErrTypeValidation, "failed to parse explore output", err)
		}

		return &exploreOutput, nil
	})

	if err != nil {
		logger.ErrorContext(ctx, "explore execution failed",
			slog.String("agent_id", e.id),
			slog.Any("error", err),
		)
	} else {
		logger.InfoContext(ctx, "explore execution completed",
			slog.String("agent_id", e.id),
			slog.Int("key_files_found", len(output.KeyFiles)),
			slog.Int("code_snippets", len(output.CodeSnippets)),
		)
	}

	return output, err
}

func Explore(ctx context.Context, input *ExploreInput) (*schema.ToolResult, error) {
	logger := trace.Logger(ctx)

	logger.InfoContext(ctx,
		"explore execution started",
		slog.String("task", input.Task),
		slog.String("thoroughness", string(input.Thoroughness)),
	)
	// Set default thoroughness if not specified
	if input.Thoroughness == "" {
		input.Thoroughness = ThoroughnessQuick
	}

	// Validate thoroughness level
	validThoroughness := []ThoroughnessLevel{ThoroughnessQuick, ThoroughnessMedium, ThoroughnessVeryThorough}
	if !slices.Contains(validThoroughness, input.Thoroughness) {
		input.Thoroughness = ThoroughnessQuick
	}
	agent, err := NewExploreBee(&bee.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to init explore agent: %w", err)
	}
	output, err := agent.Explore(ctx, input)
	if err != nil {
		return &schema.ToolResult{Parts: []schema.ToolOutputPart{
			{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("failed to explore: %s", err)}},
		}, nil
	}
	outputBytes, err := json.Marshal(output)
	if err != nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("failed to explore: failed to marshal output %s", err)},
			}}, nil
	}
	return &schema.ToolResult{Parts: []schema.ToolOutputPart{
		{Type: schema.ToolPartTypeText, Text: string(outputBytes)},
	}}, nil
}

func ExploreTool() (tool.InvokableTool, error) {
	return utils.InferTool("explore", "Fast read-only agent tool optimized for searching and analyzing codebases", Explore)
}
