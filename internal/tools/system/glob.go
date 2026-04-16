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
	// Validate input
	if input == nil {
		return &GlobOutput{
			Matches: []string{"ERROR: glob received nil input"},
		}, nil
	}
	if input.Query == "" {
		return &GlobOutput{
			Matches: []string{"ERROR: query pattern is required"},
		}, nil
	}

	matches, err := filepath.Glob(input.Query)
	if err != nil {
		return &GlobOutput{
			Matches: []string{fmt.Sprintf("ERROR: Invalid glob pattern '%s': %s", input.Query, err.Error())},
		}, nil
	}

	// Load .gitignore patterns if file exists
	wd, err := os.Getwd()
	if err != nil {
		// Continue without gitignore filtering if we can't get working directory
		return &GlobOutput{
			Matches: matches,
		}, nil
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
