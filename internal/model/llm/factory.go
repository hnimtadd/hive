package llm

import (
	"errors"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
)

// NewLLMClient creates the appropriate LLM client based on configuration.
// This is the main entry point that handles client selection at config surface level.
func NewLLMClient() (model.ToolCallingChatModel, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return NewLLMToolCallingClientWithConfig(&cfg.AI)
}

// NewLLMClientWithConfig creates LLM client with provided config.
func NewLLMClientWithConfig(cfg *config.AIConfig) (model.ToolCallingChatModel, error) {
	switch cfg.Provider {
	case "claude":
		if cfg.Claude == nil {
			return nil, errors.New("claude configuration is required when provider is 'claude'")
		}
		return NewClaudeClientWithConfig(cfg.Claude)
	case "openai":
		if cfg.OpenAI == nil {
			return nil, errors.New("openai configuration is required when provider is 'openai'")
		}
		return NewOpenAIClientWithConfig(cfg.OpenAI)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}

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
	case "claude":
		log.Println("initializing claude llm")
		if cfg.Claude == nil {
			return nil, errors.New("claude configuration is required when provider is 'claude'")
		}
		return NewClaudeToolCallingClientWithConfig(cfg.Claude)
	case "openai":
		log.Println("initializing openai llm")
		if cfg.OpenAI == nil {
			return nil, errors.New("openai configuration is required when provider is 'openai'")
		}
		return NewOpenAIToolCallingClientWithConfig(cfg.OpenAI)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}
