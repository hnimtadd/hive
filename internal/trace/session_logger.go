package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/internal/types"
	"github.com/hnimtadd/hive/pkg/config"
)

// SessionLogger handles session/agent execution logging
type SessionLogger struct {
	config  *config.SessionLogConfig
	logFile *os.File
}

// NewSessionLogger creates a new session logger based on config
func NewSessionLogger(cfg *config.SessionLogConfig) (*SessionLogger, error) {
	if cfg == nil || !cfg.Enabled {
		return &SessionLogger{config: cfg}, nil
	}

	// Ensure log directory exists
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
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

// LogLLMRequest logs an LLM request
func (l *SessionLogger) LogLLMRequest(ctx context.Context, req *LLMRequestLog) {
	if l == nil || l.logFile == nil || !l.config.LogRequests {
		return
	}
	l.writeEntry("LLM_REQUEST", req)
}

// LogLLMResponse logs an LLM response
func (l *SessionLogger) LogLLMResponse(ctx context.Context, resp *LLMResponseLog) {
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

// LogAgentCycle logs a complete agent execution cycle
func (l *SessionLogger) LogAgentCycle(ctx context.Context, cycle *types.AgentCycleLog) {
	if l == nil || l.logFile == nil {
		return
	}
	l.writeEntry("AGENT_CYCLE", cycle)
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

// LLMRequestLog represents a logged LLM request
type LLMRequestLog struct {
	AgentID      string   `json:"agent_id"`
	CallID       string   `json:"call_id"`
	Model        string   `json:"model"`
	MessageCount int      `json:"message_count"`
	Messages     []string `json:"messages"` // truncated content of each message
	Tools        []string `json:"tools,omitempty"`
}

// NewLLMRequestLog creates a LLMRequestLog
func NewLLMRequestLog(agentID, callID, model string, messages []*schema.Message, tools []string, logger *SessionLogger) *LLMRequestLog {
	msgContents := make([]string, len(messages))
	for i, msg := range messages {
		content := msg.Content
		if logger != nil {
			content = logger.truncateContent(content)
		}
		msgContents[i] = fmt.Sprintf("[%s] %s", msg.Role, content)
	}

	return &LLMRequestLog{
		AgentID:      agentID,
		CallID:       callID,
		Model:        model,
		MessageCount: len(messages),
		Messages:     msgContents,
		Tools:        tools,
	}
}

// LLMResponseLog represents a logged LLM response
type LLMResponseLog struct {
	AgentID      string    `json:"agent_id"`
	CallID       string    `json:"call_id"`
	Content      string    `json:"content"`
	FinishReason string    `json:"finish_reason,omitempty"`
	Usage        *UsageLog `json:"usage,omitempty"`
}

// UsageLog represents token usage
type UsageLog struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewLLMResponseLog creates a LLMResponseLog
func NewLLMResponseLog(agentID, callID string, resp *schema.Message, logger *SessionLogger) *LLMResponseLog {
	content := ""
	finishReason := ""
	var usage *UsageLog

	if resp != nil {
		content = resp.Content
		if logger != nil {
			content = logger.truncateContent(content)
		}
		if resp.ResponseMeta != nil {
			finishReason = resp.ResponseMeta.FinishReason
			if resp.ResponseMeta.Usage != nil {
				usage = &UsageLog{
					PromptTokens:     resp.ResponseMeta.Usage.PromptTokens,
					CompletionTokens: resp.ResponseMeta.Usage.CompletionTokens,
					TotalTokens:      resp.ResponseMeta.Usage.TotalTokens,
				}
			}
		}
	}

	return &LLMResponseLog{
		AgentID:      agentID,
		CallID:       callID,
		Content:      content,
		FinishReason: finishReason,
		Usage:        usage,
	}
}

// ToolCallLog represents a logged tool call
type ToolCallLog struct {
	AgentID  string `json:"agent_id"`
	CallID   string `json:"call_id"`
	ToolName string `json:"tool_name"`
	Input    string `json:"input"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

// NewToolCallLog creates a ToolCallLog
func NewToolCallLog(agentID, callID, toolName, input, output string, err error, logger *SessionLogger) *ToolCallLog {
	log := &ToolCallLog{
		AgentID:  agentID,
		CallID:   callID,
		ToolName: toolName,
		Input:    logger.truncateContent(input),
	}

	if err == nil {
		log.Output = logger.truncateContent(output)
	} else {
		log.Error = err.Error()
	}

	return log
}

// ErrorLog represents a logged error
type ErrorLog struct {
	AgentID string `json:"agent_id"`
	CallID  string `json:"call_id"`
	Error   string `json:"error"`
	Context string `json:"context,omitempty"`
}

// NewErrorLog creates an ErrorLog
func NewErrorLog(agentID, callID, errMsg, context string) *ErrorLog {
	return &ErrorLog{
		AgentID: agentID,
		CallID:  callID,
		Error:   errMsg,
		Context: context,
	}
}
