package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	gitignore "github.com/sabhiram/go-gitignore"
)

func GlobTool() (tool.InvokableTool, error) {
	return utils.InferTool("glob", "a pattern-matching utility used to find files and directories whose names match a specific pattern", glob)
}

type GlobInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"Search keyword"`
}

type GlobOutput struct {
	Matches []string `json:"matches" jsonschema:"Matches files"`
}

func glob(_ context.Context, input *GlobInput) (*GlobOutput, error) {
	matches, err := filepath.Glob(input.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to run glob: %w", err)
	}

	// Load .gitignore patterns if file exists
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	gitignorePath := filepath.Join(wd, ".gitignore")
	var ignorer *gitignore.GitIgnore
	if _, err := os.Stat(gitignorePath); err == nil {
		ignorer, err = gitignore.CompileIgnoreFile(gitignorePath)
		if err != nil {
			// Continue without filtering if .gitignore parsing fails
			ignorer = nil
		}
	}

	// Filter matches against gitignore patterns
	filtered := make([]string, 0, len(matches))
	for _, match := range matches {
		relPath, err := filepath.Rel(wd, match)
		if err != nil {
			relPath = match
		}

		// Include file if no ignorer or if not ignored
		if ignorer == nil || !ignorer.MatchesPath(relPath) {
			filtered = append(filtered, match)
		}
	}

	return &GlobOutput{
		Matches: filtered,
	}, nil
}
