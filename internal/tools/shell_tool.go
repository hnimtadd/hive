package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ExecuteShellTool executes shell commands
type ExecuteShellTool struct {
	workingDir string
	timeout    time.Duration
}

// Info implements [tool.InvokableTool].
func (t *ExecuteShellTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_shell",
		Desc: "Execute a shell command. Use this for git operations, file system operations, running tests, builds, etc. The command runs in a shell environment with access to all standard tools.",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"command": {
					Type:     schema.String,
					Desc:     "The shell command to execute (e.g., 'git clone repo.git', 'ls -la', 'npm install')",
					Required: true,
				},
				"working_dir": {
					Type:     schema.String,
					Desc:     "Working directory for the command (optional, defaults to current directory)",
					Required: false,
				},
			},
		),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (t *ExecuteShellTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Command    string  `json:"command"`
		WorkingDir *string `json:"working_dir,omitempty"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// Determine working directory
	workDir := t.workingDir
	if args.WorkingDir != nil && *args.WorkingDir != "" {
		workDir = *args.WorkingDir
	}

	// Create command context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	log.Println("Executing", args.Command)

	// Execute command in shell
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", args.Command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	if err != nil {
		// Include both error and output for debugging
		return fmt.Sprintf("command failed: %s\nOutput: %s", err, outputStr), nil
	}

	// Return output as result
	result, err := json.Marshal(map[string]any{
		"status":      "success",
		"output":      outputStr,
		"command":     args.Command,
		"working_dir": workDir,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

// NewExecuteShellTool creates a new shell execution tool
func NewExecuteShellTool(workingDir string) tool.InvokableTool {
	return &ExecuteShellTool{
		workingDir: workingDir,
		timeout:    5 * time.Minute, // Default 5 minute timeout
	}
}

// NewExecuteShellToolWithTimeout creates a new shell execution tool with custom timeout
func NewExecuteShellToolWithTimeout(workingDir string, timeout time.Duration) tool.InvokableTool {
	return &ExecuteShellTool{
		workingDir: workingDir,
		timeout:    timeout,
	}
}
