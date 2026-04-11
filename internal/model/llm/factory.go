package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/config"
)

type Tier string

const (
	TierFast    Tier = "fast"
	TierSmart   Tier = "smart"
	TierDefault Tier = "default"
)

type Provider interface {
	GetModel(ctx context.Context, tier Tier) (model.ToolCallingChatModel, bool)
}

type provider struct {
	cfg    *config.AIConfig
	models map[Tier]model.ToolCallingChatModel
}

// GetModel implements [Provider].
func (p *provider) GetModel(ctx context.Context, tier Tier) (model.ToolCallingChatModel, bool) {
	logger := trace.Logger(ctx)
	model, isDefined := p.models[tier]
	if !isDefined {
		logger.InfoContext(ctx, "Model with tier is not defined, use default tier", slog.String("tier", string(tier)))
		model, isDefined = p.models[TierDefault]
	}
	if !isDefined {
		logger.InfoContext(ctx, "Model with tier is not defined")
		return nil, false
	}
	return model, true
}

func NewLLMProvider(cfg *config.AIConfig) (Provider, error) {
	models := map[Tier]model.ToolCallingChatModel{}
	for tier, tierConfig := range cfg.Tiers {
		switch t := Tier(tier); t {
		case TierDefault, TierFast, TierSmart:
			model, err := newLLMToolCallingClientWithConfig(tierConfig.Provider, tierConfig.Model, cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to init llm with tier %s: %w", tier, err)
			}
			models[t] = model
		default:
			return nil, fmt.Errorf("unsupported tier: %s", tier)
		}
	}
	if len(models) == 0 {
		return nil, errors.New("no LLM tier available")
	}
	return &provider{
		cfg:    cfg,
		models: models,
	}, nil
}

// newLLMToolCallingClientWithConfig creates LLM client with provided config.
func newLLMToolCallingClientWithConfig(provider string, model string, cfg *config.AIConfig) (model.ToolCallingChatModel, error) {
	switch provider {
	case "anthropic":
		log.Println("initializing anthropic llm")
		if cfg.Anthropic == nil {
			return nil, errors.New("anthropic configuration is required when provider is 'anthropic'")
		}
		return newAnthropicToolCallingClientWithConfig(model, cfg.Anthropic)
	case "openai":
		log.Println("initializing openai llm")
		if cfg.OpenAI == nil {
			return nil, errors.New("openai configuration is required when provider is 'openai'")
		}
		return newOpenAIToolCallingClientWithConfig(model, cfg.OpenAI)
	case "ollama":
		log.Println("initializing ollama llm")
		if cfg.Ollama == nil {
			return nil, errors.New("ollama configuration is required when provider is 'ollama'")
		}
		return newOllamaToolCallingClientWithConfig(model, cfg.Ollama)
	case "openrouter":
		log.Println("initializing openrouter llm")
		if cfg.OpenAI == nil {
			return nil, errors.New("openrouter configuration is required when provider is 'openai'")
		}
		return newOpenRouterToolCallingClientWithConfig(model, cfg.OpenAI)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}
}
