package system

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/observability"
)

// ShellSession manages a persistent Shell session.
type ShellSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
}

var (
	// Map of trace_id -> ShellSession for isolated sessions per request.
	sessions  = make(map[string]*ShellSession)
	sessionMu sync.Mutex
)

// allowedCommands of commands that are considered safe.
var allowedCommands = map[string]bool{
	// File operations
	"ls":    true,
	"cat":   true,
	"head":  true,
	"tail":  true,
	"less":  true,
	"more":  true,
	"file":  true,
	"stat":  true,
	"wc":    true,
	"du":    true,
	"df":    true,
	"pwd":   true,
	"cd":    true,
	"mkdir": true,
	"touch": true,
	"cp":    true,
	"mv":    true,
	"rm":    true,
	"rmdir": true,
	"chmod": true,
	"chown": true,
	"ln":    true,
	// Search and text processing
	"grep":  true,
	"find":  true,
	"sort":  true,
	"uniq":  true,
	"cut":   true,
	"awk":   true,
	"sed":   true,
	"tr":    true,
	"diff":  true,
	"patch": true,
	// Archive operations
	"tar":    true,
	"gzip":   true,
	"gunzip": true,
	"zip":    true,
	"unzip":  true,
	// Development tools
	"git":       true,
	"make":      true,
	"npm":       true,
	"yarn":      true,
	"pip":       true,
	"python":    true,
	"python3":   true,
	"node":      true,
	"go":        true,
	"cargo":     true,
	"rustc":     true,
	"javac":     true,
	"java":      true,
	"gcc":       true,
	"g++":       true,
	"clang":     true,
	"docker":    true,
	"kubectl":   true,
	"terraform": true,
	// Process and system info
	"ps":       true,
	"top":      true,
	"kill":     true,
	"killall":  true,
	"which":    true,
	"whereis":  true,
	"type":     true,
	"env":      true,
	"export":   true,
	"echo":     true,
	"printf":   true,
	"date":     true,
	"uptime":   true,
	"uname":    true,
	"hostname": true,
	// Network tools
	"curl":    true,
	"wget":    true,
	"ping":    true,
	"netstat": true,
	"ss":      true,
	"telnet":  true,
	"nc":      true,
	// Package managers
	"apt":     true,
	"apt-get": true,
	"yum":     true,
	"dnf":     true,
	"brew":    true,
	"pacman":  true,
}

// ShellOperators that should be blocked to prevent command chaining bypasses.
var ShellOperators = map[string]bool{
	"&&": true,
	"||": true,
	"|":  true,
	";":  true,
	"&":  true,
	">":  true,
	"<":  true,
	">>": true,
	"<<": true,
}

// CleanupShellSession closes and removes the Shell session for a specific trace_id
func CleanupShellSession(traceID string) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if session, exists := sessions[traceID]; exists {
		if err := session.Close(); err != nil {
			return fmt.Errorf("failed to close session: %w", err)
		}
		delete(sessions, traceID)
	}
	return nil
}

// CleanupAllShellSessions closes all active Shell sessions (useful for shutdown)
func CleanupAllShellSessions() error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	var errs []error
	for traceID, session := range sessions {
		if err := session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close session for trace %s: %w", traceID, err))
		}
		delete(sessions, traceID)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errs)
	}
	return nil
}

// GetActiveShellSessionCount returns the number of active Shell sessions
func GetActiveShellSessionCount() int {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	return len(sessions)
}

func ShellTool() (tool.InvokableTool, error) {
	return utils.InferTool("Shell", "execute Shell shell commands in a persistent session", Shell)
}

type ShellInput struct {
	Command string `json:"command" jsonschema_description:"The Shell command to execute"`
	Restart bool   `json:"restart" jsonschema_description:"Set to true to restart the shell session"`
	Timeout int    `json:"timeout" jsonschema_description:"Command timeout in seconds (default: 30, max: 300)"`
}

// simpleTokenize provides basic shell tokenization for command validation
// This is a simplified version that handles spaces and basic quoting.
func simpleTokenize(command string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, char := range command {
		switch char {
		case '"', '\'':
			switch {
			case !inQuote:
				inQuote = true
				quoteChar = char
			case char == quoteChar:
				inQuote = false
				quoteChar = 0
			default:
				current.WriteRune(char)
			}
		case ' ', '\t', '\n':
			if inQuote {
				current.WriteRune(char)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// validateCommand checks if a command is safe to execute.
func validateCommand(command string) (bool, string) {
	// Parse the command
	tokens := simpleTokenize(command)

	if len(tokens) == 0 {
		return false, "Empty command"
	}

	// Extract the executable
	executable := tokens[0]

	// Check if it's in the allowlist
	if !allowedCommands[executable] {
		return false, fmt.Sprintf("Command '%s' is not in the allowlist", executable)
	}

	// Check for dangerous patterns in all tokens
	for _, token := range tokens {
		// Block shell operators
		if ShellOperators[token] {
			return false, fmt.Sprintf("Shell operator '%s' is not allowed", token)
		}

		// Block command substitution
		if strings.Contains(token, "$") || strings.Contains(token, "`") {
			return false, "Command substitution is not allowed"
		}

		// Block dangerous rm patterns
		if executable == "rm" {
			if strings.Contains(command, "rm -rf /") || strings.Contains(command, "rm -rf /*") {
				return false, "Dangerous rm command pattern detected"
			}
		}
	}

	return true, ""
}

// NewShellSession creates a new persistent Shell session.
func NewShellSession() (*ShellSession, error) {
	cmd := exec.Command("/bin/Shell")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	return &ShellSession{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

// Execute runs a command in the Shell session.
func (s *ShellSession) Execute(command string, timeout time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add a unique marker to detect end of output
	marker := fmt.Sprintf("__CMD_END_%d__", time.Now().UnixNano())
	fullCommand := fmt.Sprintf("%s\necho %s\n", command, marker)

	// Write command to stdin
	if _, err := s.stdin.Write([]byte(fullCommand)); err != nil {
		return "", fmt.Errorf("failed to write command: %w", err)
	}

	// Read output with timeout
	outputChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		var output strings.Builder
		buf := make([]byte, 4096)

		for {
			n, err := s.stdout.Read(buf)
			if err != nil {
				errorChan <- err
				return
			}

			chunk := string(buf[:n])
			output.WriteString(chunk)

			// Check if we've received the marker
			if strings.Contains(output.String(), marker) {
				// Remove the marker from output
				result := strings.Replace(output.String(), marker+"\n", "", 1)
				result = strings.TrimSuffix(result, marker)
				outputChan <- result
				return
			}
		}
	}()

	// Wait for output or timeout
	select {
	case output := <-outputChan:
		return output, nil
	case err := <-errorChan:
		return "", fmt.Errorf("failed to read output: %w", err)
	case <-time.After(timeout):
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
}

// Close terminates the Shell session
func (s *ShellSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

func Shell(ctx context.Context, input *ShellInput) (*schema.ToolResult, error) {
	// Validate input
	if input == nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "ERROR: shell received nil input"},
			},
		}, nil
	}

	// Extract trace ID from context
	traceCtx, ok := observability.TraceContextFromContext(ctx)
	if !ok {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "Error: trace_id not found in context"},
			},
		}, nil
	}
	traceID := traceCtx.TraceID

	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Handle session restart
	if input.Restart {
		// Close existing session for this trace_id if it exists
		if session, exists := sessions[traceID]; exists {
			session.Close()
			delete(sessions, traceID)
		}

		// Create new session
		newSession, err := NewShellSession()
		if err != nil {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to restart shell session: %s", err)},
				},
			}, nil
		}
		sessions[traceID] = newSession

		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "Shell session restarted successfully"},
			},
		}, nil
	}

	// Validate command is provided
	if input.Command == "" {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "Error: command is required (or set restart=true to restart session)"},
			},
		}, nil
	}

	// Validate command safety
	valid, reason := validateCommand(input.Command)
	if !valid {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Command rejected: %s", reason)},
			},
		}, nil
	}

	// Get or create session for this trace_id
	session, exists := sessions[traceID]
	if !exists {
		var err error
		session, err = NewShellSession()
		if err != nil {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to create shell session: %s", err)},
				},
			}, nil
		}
		sessions[traceID] = session
	}

	// Set timeout (default 30s, max 300s)
	timeout := time.Duration(input.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	// Execute command with context timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		output, err := session.Execute(input.Command, timeout)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- output
	}()

	// Wait for result or context cancellation
	select {
	case output := <-resultChan:
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: output},
			},
		}, nil
	case err := <-errorChan:
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Execution error: %s", err)},
			},
		}, nil
	case <-ctx.Done():
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Command cancelled: %s", ctx.Err())},
			},
		}, nil
	}
}
