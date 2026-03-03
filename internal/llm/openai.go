package llm

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
)

// NewOpenAIClient creates a new OpenAI client that implements Eino's model.ToolCallingChatModel interface.
func NewOpenAIClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AI.OpenAI == nil {
		return nil, errors.New("openai configuration is empty")
	}

	return NewOpenAIClientWithConfig(cfg.AI.OpenAI)
}

// NewOpenAIClientWithConfig creates a new OpenAI client with provided config.
func NewOpenAIClientWithConfig(cfg *config.OpenAIConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("openai configuration is empty")
	}

	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}

	openaiConfig := &openai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   cfg.Model,
		BaseURL: cfg.BaseURL,
	}

	// Add extra fields for Anthropic models
	if cfg.ExtraFields != nil {
		openaiConfig.ExtraFields = cfg.ExtraFields
	}

	chatModel, err := openai.NewChatModel(context.Background(), openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI model: %w", err)
	}

	return chatModel, nil
}

// NewOpenAIToolCallingClient creates an OpenAI client that supports tool calling.
func NewOpenAIToolCallingClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AI.OpenAI == nil {
		return nil, errors.New("openai configuration is empty")
	}

	return NewOpenAIToolCallingClientWithConfig(cfg.AI.OpenAI)
}

// NewOpenAIToolCallingClientWithConfig creates a tool-calling OpenAI client with config.
func NewOpenAIToolCallingClientWithConfig(cfg *config.OpenAIConfig) (model.ToolCallingChatModel, error) {
	return NewOpenAIClientWithConfig(cfg)
}