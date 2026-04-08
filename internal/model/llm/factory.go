package llm

import (
	"errors"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
)

// NewLLMToolCallingClient creates the appropriate LLM client based on configuration.
// This is the main entry point that handles client selection at config surface level.
func NewLLMToolCallingClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewLLMToolCallingClientWithConfig(&cfg.AI)
}

// NewLLMToolCallingClientWithConfig creates LLM client with provided config.
func NewLLMToolCallingClientWithConfig(cfg *config.AIConfig) (model.ToolCallingChatModel, error) {
	switch cfg.Provider {
	case "anthropic":
		log.Println("initializing anthropic llm")
		if cfg.Anthropic == nil {
			return nil, errors.New("anthropic configuration is required when provider is 'anthropic'")
		}
		return NewAnthropicToolCallingClientWithConfig(cfg.Anthropic)
	case "openai":
		log.Println("initializing openai llm")
		if cfg.OpenAI == nil {
			return nil, errors.New("openai configuration is required when provider is 'openai'")
		}
		return NewOpenAIToolCallingClientWithConfig(cfg.OpenAI)
	case "ollama":
		log.Println("initializing ollama llm")
		if cfg.Ollama == nil {
			return nil, errors.New("ollama configuration is required when provider is 'ollama'")
		}
		return NewOllamaToolCallingClientWithConfig(cfg.Ollama)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}
