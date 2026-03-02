package llm

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

// NewClaudeClient creates a new Claude client that implements Eino's model.ChatModel interface.
// This replaces the custom llm.Client interface with Eino's standard interface.
func NewClaudeClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewClaudeClientWithConfig(&cfg.AI)
}

// NewClaudeClientWithConfig creates a new Claude client with provided config.
// Returns Eino's model.ChatModel interface instead of custom wrapper.
func NewClaudeClientWithConfig(cfg *config.AIConfig) (*claude.ChatModel, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}

	claudeConfig := &claude.Config{
		APIKey: apiKey,
		Model:  cfg.Model,
	}

	if cfg.BaseURL != "" {
		claudeConfig.BaseURL = types.Ptr(cfg.BaseURL)
	}

	// Return Eino's ChatModel directly - no wrapper needed
	chatModel, err := claude.NewChatModel(context.Background(), claudeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude model: %w", err)
	}

	return chatModel, nil
}

// NewClaudeToolCallingClient creates a Claude client that supports tool calling.
// This returns the more advanced ToolCallingChatModel interface.
func NewClaudeToolCallingClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewClaudeToolCallingClientWithConfig(&cfg.AI)
}

// NewClaudeToolCallingClientWithConfig creates a tool-calling Claude client with config.
func NewClaudeToolCallingClientWithConfig(cfg *config.AIConfig) (model.ToolCallingChatModel, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}

	claudeConfig := &claude.Config{
		APIKey: apiKey,
		Model:  cfg.Model,
	}

	if cfg.BaseURL != "" {
		claudeConfig.BaseURL = types.Ptr(cfg.BaseURL)
	}

	// Claude's ChatModel implements both ChatModel and ToolCallingChatModel
	chatModel, err := claude.NewChatModel(context.Background(), claudeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude model: %w", err)
	}

	return chatModel, nil
}
