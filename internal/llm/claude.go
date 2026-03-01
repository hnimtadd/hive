package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/schema"
	"github.com/hnimtadd/hive/pkg/config"
)

// LLMClient represents a generic Large Language Model client interface
type LLMClient interface {
	Generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error)
	Close() error
}

// ClaudeClient is a Claude-specific implementation of LLMClient
type ClaudeClient struct {
	model *claude.ChatModel
}

// NewClaudeClient creates a new Claude client
func NewClaudeClient() (*ClaudeClient, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewClaudeClientWithConfig(&cfg.AI)
}

// NewClaudeClientWithConfig creates a new Claude client with provided config
func NewClaudeClientWithConfig(cfg *config.AIConfig) (*ClaudeClient, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}

	claudeConfig := &claude.Config{
		APIKey: apiKey,
		Model:  cfg.Model,
	}

	if cfg.BaseURL != "" {
		baseURL := cfg.BaseURL
		claudeConfig.BaseURL = &baseURL
	}

	model, err := claude.NewChatModel(context.Background(), claudeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude model: %w", err)
	}

	return &ClaudeClient{model: model}, nil
}

// Generate sends a message to Claude and returns the response
func (c *ClaudeClient) Generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	response, err := c.model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("claude generation failed: %w", err)
	}

	return response, nil
}

// Close closes the Claude client (no-op for Claude)
func (c *ClaudeClient) Close() error {
	// Claude client doesn't need explicit closing
	return nil
}
