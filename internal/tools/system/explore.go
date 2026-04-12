package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/trace"
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

// ExploreOutput defines output from the Explore agent - kept simple for agent consumption.
type ExploreOutput struct {
	// Brief summary of what was found (most important)
	Summary string `json:"summary" jsonschema:"2-3 sentence summary of key findings"`

	// Key files discovered with brief descriptions (optional)
	RelevantFiles []FileInfo `json:"relevant_files,omitempty" jsonschema:"Important files found, include path and brief description"`

	// Additional context or patterns observed (optional)
	KeyInsights string `json:"key_insights,omitempty" jsonschema:"Notable patterns, architecture, or observations"`
}

// FileInfo represents a discovered file - minimal info for other agents.
type FileInfo struct {
	Path        string `json:"path"        jsonschema:"File path"`
	Description string `json:"description" jsonschema:"What this file contains or does"`
}

// filterReadOnlyTools filters tools to only include safe read-only operations.
func filterReadOnlyTools(tools map[string]tool.InvokableTool) []tool.InvokableTool {
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

// GetExploreSystemPrompt returns the system prompt for the Explore agent.
func GetExploreSystemPrompt() string {
	inputDesc, _ := hiveutils.DescribeJSONSchema[ExploreInput]()
	// Generate human-readable schema description instead of raw JSON schema
	// This prevents LLM from outputting schema definition instead of data
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

======== Output Format =======
CRITICAL: Return ONLY a raw JSON object that folow this JSON SCHEMA without markdown decorator, any output that not follow the convention will be rejected.
%s

========= INPUT FORMAT ========
%s`, outputDesc, inputDesc)
}

// Explore performs codebase exploration.
func explore(provider llm.Provider) func(ctx context.Context, input *ExploreInput) (*ExploreOutput, error) {
	return func(ctx context.Context, input *ExploreInput) (*ExploreOutput, error) {
		uuid, _ := uuid.NewUUID()
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

		systemTools, err := Tools()
		if err != nil {
			logger.ErrorContext(ctx, "failed to create tools", slog.String("err", err.Error()))
			return nil, errors.New("no read-only tools available")
		}
		readTools := filterReadOnlyTools(systemTools)
		logger.InfoContext(ctx, "read tools", slog.Int("num tools", len(readTools)))

		bee, err := bee.NewCustomBee[ExploreInput, ExploreOutput](&bee.Config{
			ID:           uuid.String(),
			Tools:        readTools,
			Persona:      GetExploreSystemPrompt(),
			ModelPool:    provider.ModelPool(llm.TierFast),
			MaxSteps:     100,
			TimeoutInSec: 200,
		})
		if err != nil {
			logger.ErrorContext(ctx, "failed to create explore agent", slog.String("err", err.Error()))
			return nil, err
		}
		return bee.Execute(ctx, input)
	}
}

func ExploreTool(llm llm.Provider) (tool.InvokableTool, error) {
	return utils.InferTool("explore", "Fast read-only agent tool optimized for searching and analyzing codebases", explore(llm))
}
