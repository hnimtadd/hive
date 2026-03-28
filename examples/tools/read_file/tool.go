package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Info implements [tool.InvokableTool].
// func Info() (*schema.ToolInfo, error) {
// 	return &schema.ToolInfo{
// 		Name: "read_local_file",
// 		Desc: "Read the contents of a local file from the filesystem. Returns the complete file content.",
// 		ParamsOneOf: schema.NewParamsOneOfByParams(
// 			map[string]*schema.ParameterInfo{
// 				"path": {
// 					Type:     schema.String,
// 					Desc:     "File path to read (relative or absolute)",
// 					Required: true,
// 				},
// 			},
// 		),
// 	}, nil
// }

// InvokableRun implements [tool.InvokableTool].
func InvokableRun(argumentsInJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Path == "" {
		return "", errors.New("path is required")
	}

	// Resolve path
	content, err := os.ReadFile(args.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", args.Path)
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status":    "success",
		"path":      args.Path,
		"full_path": args.Path,
		"content":   string(content),
		"size":      len(content),
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
		return
	}
	output, err := InvokableRun(string(stdinBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to invoke tool: %s", err)
		return
	}
	fmt.Fprint(os.Stdout, output)
	fmt.Fprintf(os.Stderr, "success")
}
