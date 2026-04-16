package system

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func GrepTool() (tool.InvokableTool, error) {
	return utils.InferTool("grep", "search for patterns in file contents using regular expressions", grep)
}

type GrepInput struct {
	Pattern     string `json:"pattern"      jsonschema:"required" jsonschema_description:"Regular expression pattern to search for"`
	Path        string `json:"path"         jsonschema:"required" jsonschema_description:"File or directory path to search in"`
	Recursive   bool   `json:"recursive"                          jsonschema_description:"Search recursively in directories (default: false)"`
	IgnoreCase  bool   `json:"ignore_case"                        jsonschema_description:"Case insensitive search (default: false)"`
	LineNumbers bool   `json:"line_numbers"                       jsonschema_description:"Show line numbers in output (default: true)"`
	MaxResults  int    `json:"max_results"                        jsonschema_description:"Maximum number of results to return (default: 100)"`
}

type GrepOutput struct {
	Matches []string `json:"matches"`
}

func grep(_ context.Context, input *GrepInput) (*GrepOutput, error) {
	// Set defaults
	if input.MaxResults == 0 {
		input.MaxResults = 100
	}
	if !input.LineNumbers {
		input.LineNumbers = true
	}

	// Compile regex pattern
	var re *regexp.Regexp
	var err error
	if input.IgnoreCase {
		re, err = regexp.Compile("(?i)" + input.Pattern)
	} else {
		re, err = regexp.Compile(input.Pattern)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []string
	resultCount := 0

	// Check if path is file or directory
	info, err := os.Stat(input.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to access path: %w", err)
	}

	var filesToSearch []string
	if info.IsDir() {
		if input.Recursive {
			err = filepath.Walk(input.Path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip files with errors
				}
				if !info.IsDir() {
					filesToSearch = append(filesToSearch, path)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to walk directory: %w", err)
			}
		} else {
			return nil, errors.New("path is a directory. Set recursive=true to search recursively.")

		}
	} else {
		filesToSearch = append(filesToSearch, input.Path)
	}

	// Search in files
	for _, filePath := range filesToSearch {
		if resultCount >= input.MaxResults {
			results = append(results, fmt.Sprintf("\n... (results truncated at %d matches)", input.MaxResults))
			break
		}

		file, err := os.Open(filePath)
		if err != nil {
			continue // Skip files that can't be opened
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() && resultCount < input.MaxResults {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				if input.LineNumbers {
					if len(filesToSearch) > 1 {
						results = append(results, fmt.Sprintf("%s:%d:%s", filePath, lineNum, line))
					} else {
						results = append(results, fmt.Sprintf("%d:%s", lineNum, line))
					}
				} else {
					if len(filesToSearch) > 1 {
						results = append(results, fmt.Sprintf("%s:%s", filePath, line))
					} else {
						results = append(results, line)
					}
				}
				resultCount++
			}
		}
		file.Close()

		if err := scanner.Err(); err != nil {
			// Skip files with scan errors (e.g., binary files)
			continue
		}
	}

	return &GrepOutput{
		Matches: results,
	}, nil
}
