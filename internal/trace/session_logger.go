package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hnimtadd/hive/pkg/config"
)

// SessionLogger handles session/agent execution logging.
type SessionLogger struct {
	config  *config.SessionLogConfig
	logFile *os.File
}

// NewSessionLogger creates a new session logger based on config.
func NewSessionLogger(cfg *config.SessionLogConfig) (*SessionLogger, error) {
	if cfg == nil || !cfg.Enabled {
		return &SessionLogger{config: cfg}, nil
	}

	// Ensure log directory exists
	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session log directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(cfg.Dir, fmt.Sprintf("session_%s.jsonl", timestamp))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create session log file: %w", err)
	}

	return &SessionLogger{
		config:  cfg,
		logFile: logFile,
	}, nil
}

// LogLLMRequest logs an LLM request.
func (l *SessionLogger) LogLLMRequest(_ context.Context, req *LLMRequestLog) {
	if l == nil || l.logFile == nil || !l.config.LogRequests {
		return
	}
	l.writeEntry("LLM_REQUEST", req)
}

// LogLLMResponse logs an LLM response.
func (l *SessionLogger) LogLLMResponse(_ context.Context, resp *LLMResponseLog) {
	if l == nil || l.logFile == nil || !l.config.LogResponses {
		return
	}
	l.writeEntry("LLM_RESPONSE", resp)
}

// LogToolCall logs a tool invocation
func (l *SessionLogger) LogToolCall(ctx context.Context, tool *ToolCallLog) {
	if l == nil || l.logFile == nil || !l.config.LogTools {
		return
	}
	l.writeEntry("TOOL_CALL", tool)
}

// LogError logs an error event
func (l *SessionLogger) LogError(ctx context.Context, err *ErrorLog) {
	if l == nil || l.logFile == nil {
		return
	}
	l.writeEntry("ERROR", err)
}

func (l *SessionLogger) writeEntry(eventType string, data any) {
	entry := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"event":     eventType,
		"data":      data,
	}

	line, _ := json.Marshal(entry)
	_, _ = l.logFile.Write(append(line, '\n'))
	_ = l.logFile.Sync()
}

// Close closes the log file.
func (l *SessionLogger) Close() error {
	if l == nil || l.logFile == nil {
		return nil
	}
	return l.logFile.Close()
}

// IsEnabled returns true if session logging is active
func (l *SessionLogger) IsEnabled() bool {
	return l != nil && l.config != nil && l.config.Enabled && l.logFile != nil
}

// TruncateContent truncates content if configured.
func (l *SessionLogger) truncateContent(content string) string {
	if l.config != nil && l.config.MaxContentLength > 0 && len(content) > l.config.MaxContentLength {
		return content[:l.config.MaxContentLength] + "... [truncated]"
	}
	return content
}
