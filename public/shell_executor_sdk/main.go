package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hnimtadd/hive/pkg/hive"
)

type ShellExecutorInput struct {
	Command    string `json:"command"     jsonschema:"description=Shell command to execute"`
	WorkingDir string `json:"working_dir" jsonschema:"description=Working directory for command execution (optional, defaults to current dir)"`
	Timeout    int    `json:"timeout"     jsonschema:"description=Timeout in seconds (default 300, max 600)"`
}

type ShellExecutorOutput struct {
	Stdout     string `json:"stdout"      jsonschema:"description=Standard output from command"`
	Stderr     string `json:"stderr"      jsonschema:"description=Standard error from command"`
	ExitCode   int    `json:"exit_code"   jsonschema:"description=Command exit code (0 = success)"`
	Success    bool   `json:"success"     jsonschema:"description=Whether command succeeded (exit code 0)"`
	WorkingDir string `json:"working_dir" jsonschema:"description=Actual working directory used"`
	Duration   string `json:"duration"    jsonschema:"description=Command execution duration"`
}

// Safety check for dangerous commands.
func isDangerousCommand(cmd string) (bool, string) {
	dangerous := []string{
		"rm -rf /",
		"dd if=/dev/zero",
		":(){ :|:& };:", // fork bomb
		"mkfs.",
		"chmod -R 777 /",
		"> /dev/sda",
	}

	cmdLower := strings.ToLower(strings.TrimSpace(cmd))
	for _, danger := range dangerous {
		if strings.Contains(cmdLower, danger) {
			return true, fmt.Sprintf("Command contains dangerous pattern: %s", danger)
		}
	}

	// Check for suspicious sudo usage
	if strings.HasPrefix(cmdLower, "sudo ") &&
		(strings.Contains(cmdLower, "rm") || strings.Contains(cmdLower, "dd")) {
		return true, "Suspicious sudo command detected"
	}

	return false, ""
}

func shellExecute(ctx context.Context, input ShellExecutorInput) (ShellExecutorOutput, error) {
	startTime := time.Now()

	// Set defaults
	if input.Timeout == 0 {
		input.Timeout = 300 // 5 minutes default
	}
	if input.Timeout > 600 {
		input.Timeout = 600 // 10 minutes max
	}

	// Resolve working directory
	workingDir := input.WorkingDir
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return ShellExecutorOutput{}, fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		absPath, err := filepath.Abs(workingDir)
		if err != nil {
			return ShellExecutorOutput{}, fmt.Errorf("failed to resolve working directory: %w", err)
		}
		workingDir = absPath

		// Verify directory exists
		if _, err := os.Stat(workingDir); os.IsNotExist(err) {
			return ShellExecutorOutput{}, fmt.Errorf("working directory does not exist: %s", workingDir)
		}
	}

	// Safety check
	if dangerous, reason := isDangerousCommand(input.Command); dangerous {
		return ShellExecutorOutput{
			Stderr:     reason,
			ExitCode:   1,
			Success:    false,
			WorkingDir: workingDir,
			Duration:   "0s",
		}, fmt.Errorf("dangerous command rejected: %s", reason)
	}

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(input.Timeout)*time.Second)
	defer cancel()

	// Execute command via bash
	cmd := exec.CommandContext(cmdCtx, "bash", "-c", input.Command)
	cmd.Dir = workingDir

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	duration := time.Since(startTime)
	exitCode := 0
	success := true

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
		success = false
	}

	output := ShellExecutorOutput{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		Success:    success,
		WorkingDir: workingDir,
		Duration:   duration.String(),
	}

	// Return output even on command failure (non-zero exit code)
	// This allows the agent to see error messages
	return output, nil
}

func main() {
	tool, err := hive.NewTool(
		"shell_execute",
		"Execute shell commands with safety checks and timeout",
		shellExecute,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create tool: %v\n", err)
		os.Exit(1)
	}

	tool.Serve()
}
