package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/schema"
)

// Info implements [tool.InvokableTool].
func Info() (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_files",
		Desc: "List files and directories in a given path. Returns file names, sizes, and types (file/directory).",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"path": {
					Type:     schema.String,
					Desc:     "Path to list (relative or absolute). Use '.' for current directory",
					Required: true,
				},
				"recursive": {
					Type:     schema.Boolean,
					Desc:     "List files recursively (default: false)",
					Required: false,
				},
			},
		),
	}, nil
}

func InvokableRun(argumentsInJSON string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive *bool  `json:"recursive,omitempty"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Path == "" {
		return "", errors.New("path is required")
	}

	recursive := false
	if args.Recursive != nil {
		recursive = *args.Recursive
	}

	// Resolve path
	fullPath := args.Path
	var entries []map[string]any

	if recursive {
		// Recursive listing
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if path == fullPath {
				return nil
			}

			relPath, err := filepath.Rel(fullPath, path)
			if err != nil {
				return err
			}

			entryType := "file"
			if info.IsDir() {
				entryType = "directory"
			}

			entries = append(entries, map[string]any{
				"name": relPath,
				"type": entryType,
				"size": info.Size(),
			})

			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Non-recursive listing
		dirEntries, err := os.ReadDir(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range dirEntries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			entryType := "file"
			if entry.IsDir() {
				entryType = "directory"
			}

			entries = append(entries, map[string]any{
				"name": entry.Name(),
				"type": entryType,
				"size": info.Size(),
			})
		}
	}

	result, err := json.Marshal(map[string]any{
		"status":    "success",
		"path":      args.Path,
		"full_path": fullPath,
		"count":     len(entries),
		"entries":   entries,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

func main() {
	stdinBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s", err)
	}
	output, err := InvokableRun(string(stdinBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to invoke tool: %s", err)
	}
	fmt.Fprint(os.Stdout, output)
	fmt.Fprintln(os.Stderr, "success")
}
