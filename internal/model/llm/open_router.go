package llm

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cloudwego/eino-ext/components/model/openrouter"
	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
)

func newOpenRouterToolCallingClientWithConfig(models []string, cfg *config.OpenAIConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("openrouter configuration is empty")
	}
	openRouterConfig, err := prepareOpenRouterConfig(models, *cfg)
	if err != nil {
		return nil, errors.New("failed to prepare openrouter configuration")
	}
	return openrouter.NewChatModel(context.Background(), openRouterConfig)
}

func prepareOpenRouterConfig(models []string, cfg config.OpenAIConfig) (*openrouter.Config, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", cfg.APIKeyEnv)
	}
	openRouterConfig := &openrouter.Config{
		APIKey: apiKey,
		Models: models,
	}
	if cfg.BaseURL != "" {
		openRouterConfig.BaseURL = cfg.BaseURL
	}
	return openRouterConfig, nil
}
