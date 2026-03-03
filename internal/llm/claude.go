package llm

import (
	"context"
	"errors"
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
	if cfg.AI.Claude != nil {
		return nil, fmt.Errorf("claude configuration is empty")
	}

	return NewClaudeClientWithConfig(cfg.AI.Claude)
}

// NewClaudeClientWithConfig creates a new Claude client with provided config.
// Returns Eino's model.ChatModel interface instead of custom wrapper.
func NewClaudeClientWithConfig(cfg *config.ClaudeConfig) (*claude.ChatModel, error) {
	if cfg == nil {
		return nil, errors.New("claude configuration is empty")
	}
	claudeConfig, err := prepareClaudeConfig(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare claude configuration: %w", err)
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
	if cfg.AI.Claude != nil {
		return nil, fmt.Errorf("claude configuration is empty")
	}

	return NewClaudeToolCallingClientWithConfig(cfg.AI.Claude)
}

func prepareClaudeConfig(conf config.ClaudeConfig) (*claude.Config, error) {
	apiKey := os.Getenv(conf.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", conf.APIKeyEnv)
	}

	claudeConfig := &claude.Config{
		APIKey: apiKey,
		Model:  conf.Model,
	}

	if conf.BaseURL != "" {
		claudeConfig.BaseURL = types.Ptr(conf.BaseURL)
	}

	switch conf.ClaudeIntegrationType {
	case config.ClaudeIntegrationTypeAPI:
		claudeConfig.AdditionalHeaderFields = make(map[string]string)
		for key, value := range conf.Headers {
			claudeConfig.AdditionalHeaderFields[key] = value
		}
		// Add API key to headers if api-key header is configured
		if _, hasApiKey := conf.Headers["api-key"]; hasApiKey {
			claudeConfig.AdditionalHeaderFields["api-key"] = apiKey
		}

	case config.ClaudeIntegrationTypeBedrock:
		claudeConfig.ByBedrock = true
		// Region is only make sense if using with bedrock
		if conf.Region != "" {
			claudeConfig.Region = conf.Region
		}
	default:
		return nil, fmt.Errorf("unsupported claude integration type: %s", conf.ClaudeIntegrationType)
		// unknow
	}
	return claudeConfig, nil
}

// NewClaudeToolCallingClientWithConfig creates a tool-calling Claude client with config.
func NewClaudeToolCallingClientWithConfig(cfg *config.ClaudeConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("claude configuration is empty")
	}
	claudeConfig, err := prepareClaudeConfig(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare claude configuration: %w", err)
	}

	// Claude's ChatModel implements both ChatModel and ToolCallingChatModel
	chatModel, err := claude.NewChatModel(context.Background(), claudeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude model: %w", err)
	}

	return chatModel, nil
}
