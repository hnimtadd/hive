package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
)

// NewOllamaClient creates a new anthropic client that implements Eino's model.ChatModel interface.
// This replaces the custom llm.Client interface with Eino's standard interface.
func NewOllamaClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AI.Ollama == nil {
		return nil, errors.New("Ollama configuration is empty")
	}

	return NewOllamaClientWithConfig(cfg.AI.Ollama)
}

// NewOllamaClientWithConfig creates a new anthropic client with provided config.
// Returns Eino's model.ChatModel interface instead of custom wrapper.
func NewOllamaClientWithConfig(cfg *config.OllamaConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("Ollama configuration is empty")
	}

	anthropicConfig, err := prepareOllamaConfig(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare Ollama configuration: %w", err)
	}

	// Return Eino's ChatModel directly - no wrapper needed
	chatModel, err := ollama.NewChatModel(context.Background(), anthropicConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama model: %w", err)
	}

	return chatModel, nil
}

// NewOllamaToolCallingClient creates a anthropic client that supports tool calling.
// This returns the more advanced ToolCallingChatModel interface.
func NewOllamaToolCallingClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AI.Ollama == nil {
		return nil, errors.New("Ollama configuration is empty")
	}

	return NewOllamaToolCallingClientWithConfig(cfg.AI.Ollama)
}

// NewOllamaToolCallingClientWithConfig creates a tool-calling Ollama client with config.
func NewOllamaToolCallingClientWithConfig(cfg *config.OllamaConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("Ollama configuration is empty")
	}

	config, err := prepareOllamaConfig(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare Ollama configuration: %w", err)
	}

	// Ollama's ChatModel implements both ChatModel and ToolCallingChatModel
	chatModel, err := ollama.NewChatModel(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Ollama model: %w", err)
	}

	return chatModel, nil
}

func prepareOllamaConfig(conf config.OllamaConfig) (*ollama.ChatModelConfig, error) {
	return &ollama.ChatModelConfig{
		BaseURL: conf.BaseURL,
		Model:   conf.Model,
	}, nil
}
