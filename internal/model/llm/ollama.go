package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
)

// newOllamaToolCallingClientWithConfig creates a tool-calling Ollama client with config.
func newOllamaToolCallingClientWithConfig(model string, cfg *config.OllamaConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("ollama configuration is empty")
	}

	config, err := prepareOllamaConfig(model, *cfg)
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

func prepareOllamaConfig(model string, conf config.OllamaConfig) (*ollama.ChatModelConfig, error) { //nolint:unparam // this is just our convention
	return &ollama.ChatModelConfig{
		BaseURL: conf.BaseURL,
		Model:   model,
	}, nil
}
