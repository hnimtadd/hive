package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"

	anthropic "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

// newAnthropicToolCallingClientWithConfig creates a tool-calling Anthropic client with config.
func newAnthropicToolCallingClientWithConfig(model string, cfg *config.AnthropicConfig) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, errors.New("Anthropic configuration is empty")
	}

	anthropicConfig, err := prepareAnthropicConfig(model, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare Anthropic configuration: %w", err)
	}
	log.Println("using model", anthropicConfig.Model)

	// Anthropic's ChatModel implements both ChatModel and ToolCallingChatModel
	chatModel, err := anthropic.NewChatModel(context.Background(), anthropicConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anthropic model: %w", err)
	}

	return chatModel, nil
}

func prepareAnthropicConfig(model string, conf config.AnthropicConfig) (*anthropic.Config, error) {
	apiKey := os.Getenv(conf.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", conf.APIKeyEnv)
	}

	AnthropicConfig := &anthropic.Config{
		APIKey: apiKey,
		Model:  model,
	}

	if conf.BaseURL != "" {
		AnthropicConfig.BaseURL = types.Ptr(conf.BaseURL)
	}

	// Apply Anthropic-specific configuration
	switch conf.Provider {
	case config.AnthropicIntegrationTypeAPI:
		AnthropicConfig.AdditionalHeaderFields = make(map[string]string)
		maps.Copy(AnthropicConfig.AdditionalHeaderFields, conf.Headers)

	case config.AnthropicIntegrationTypeBedrock:
		AnthropicConfig.ByBedrock = true
		// Region is only make sense if using with bedrock
		if conf.Region != "" {
			AnthropicConfig.Region = conf.Region
		}
	default:
		return nil, fmt.Errorf("unsupported Anthropic integration type: %s", conf.Provider)
	}

	return AnthropicConfig, nil
}
