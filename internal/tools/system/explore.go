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

// ExploreOutput defines output from the Explore agent.
type ExploreOutput struct {
	Summary      string          `json:"summary"                 jsonschema:"Brief summary of findings"`
	KeyFiles     []FileReference `json:"key_files,omitempty"     jsonschema:"List of relevant files with descriptions"`
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
		model, _ := provider.GetModel(ctx, llm.TierFast)

		bee, err := bee.NewCustomBee[ExploreInput, ExploreOutput](&bee.Config{
			ID:           uuid.String(),
			Tools:        readTools,
			Persona:      getExploreSystemPrompt(),
			LLM:          model,
			MaxSteps:     30,
			TimeoutInSec: 60,
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
