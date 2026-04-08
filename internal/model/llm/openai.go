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

func prepareOpenAIConfig(model string, cfg *config.OpenAIConfig) (*openai.ChatModelConfig, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}

	openaiConfig := &openai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: cfg.BaseURL,
	}

	// Add extra fields for Anthropic models
	if cfg.ExtraFields != nil {
		openaiConfig.ExtraFields = cfg.ExtraFields
	}
	return openaiConfig, nil
}

// newOpenAIToolCallingClientWithConfig creates a tool-calling OpenAI client with config.
func newOpenAIToolCallingClientWithConfig(model string, cfg *config.OpenAIConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("openai configuration is empty")
	}
	openaiConfig, err := prepareOpenAIConfig(model, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare openai configuration: %w", err)
	}

	chatModel, err := openai.NewChatModel(context.Background(), openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI model: %w", err)
	}

	return chatModel, nil
}
