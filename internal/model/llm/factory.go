package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/internal/observability"
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
	ModelPool(tier Tier) func() model.ToolCallingChatModel
}

type modelPool struct {
	clients []model.ToolCallingChatModel
	current int
	mu      sync.Mutex
}

func (p *modelPool) GetModel() model.ToolCallingChatModel {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.clients) == 0 {
		return nil
	}
	m := p.clients[p.current]
	p.current = (p.current + 1) % len(p.clients)
	return m
}

type provider struct {
	cfg    *config.AIConfig
	models map[Tier]*modelPool
}

// ModelPool implements [Provider].
func (p *provider) ModelPool(tier Tier) func() model.ToolCallingChatModel {
	pool, isDefined := p.models[tier]
	if !isDefined {
		pool = p.models[TierDefault]
	}
	return pool.GetModel
}

// GetModel implements [Provider].
func (p *provider) GetModel(ctx context.Context, tier Tier) (model.ToolCallingChatModel, bool) {
	logger := observability.Logger(ctx)
	pool, isDefined := p.models[tier]
	if !isDefined {
		logger.InfoContext(ctx, "Model with tier is not defined, use default tier", slog.String("tier", string(tier)))
		pool, isDefined = p.models[TierDefault]
	}
	if !isDefined || pool == nil {
		logger.InfoContext(ctx, "Model with tier is not defined")
		return nil, false
	}
	return pool.GetModel(), true
}

func NewLLMProvider(cfg *config.AIConfig) (Provider, error) {
	pools := map[Tier]*modelPool{}
	for tier, tierConfig := range cfg.Tiers {
		switch t := Tier(tier); t {
		case TierDefault, TierFast, TierSmart:
			pool, err := newModelPool(tierConfig, cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to init llm pool for tier %s: %w", tier, err)
			}
			pools[t] = pool
		default:
			return nil, fmt.Errorf("unsupported tier: %s", tier)
		}
	}
	if len(pools) == 0 {
		return nil, errors.New("no LLM tier available")
	}
	return &provider{
		cfg:    cfg,
		models: pools,
	}, nil
}

func chunkBy[T any](slice []T, size int) [][]T {
	var result [][]T
	for i := 0; i < len(slice); i += size {
		result = append(result, slice[i:min(i+size, len(slice))])
	}
	return result
}

func newModelPool(tierConfig config.ModelTier, cfg *config.AIConfig) (*modelPool, error) {
	var clients []model.ToolCallingChatModel

	switch tierConfig.Provider {
	case "anthropic":
		log.Println("initializing anthropic llm")
		if cfg.Anthropic == nil {
			return nil, errors.New("anthropic configuration is required when provider is 'anthropic'")
		}
		client, err := newAnthropicToolCallingClientWithConfig(tierConfig.Model, cfg.Anthropic)
		if err != nil {
			return nil, err
		}
		clients = []model.ToolCallingChatModel{client}
	case "openai":
		log.Println("initializing openai llm")
		if cfg.OpenAI == nil {
			return nil, errors.New("openai configuration is required when provider is 'openai'")
		}
		client, err := newOpenAIToolCallingClientWithConfig(tierConfig.Model, cfg.OpenAI)
		if err != nil {
			return nil, err
		}
		clients = []model.ToolCallingChatModel{client}
	case "ollama":
		log.Println("initializing ollama llm")
		if cfg.Ollama == nil {
			return nil, errors.New("ollama configuration is required when provider is 'ollama'")
		}
		client, err := newOllamaToolCallingClientWithConfig(tierConfig.Model, cfg.Ollama)
		if err != nil {
			return nil, err
		}
		clients = []model.ToolCallingChatModel{client}
	case "openrouter":
		log.Println("initializing openrouter llm with model pool")
		if cfg.OpenAI == nil {
			return nil, errors.New("openrouter configuration is required when provider is 'openai'")
		}
		models := tierConfig.Models
		if len(models) == 0 {
			models = []string{tierConfig.Model}
		}
		for _, group := range chunkBy(models, 3) {
			client, err := newOpenRouterToolCallingClientWithConfig(group, cfg.OpenAI)
			if err != nil {
				return nil, err
			}
			clients = append(clients, client)
		}
		log.Printf("created model pool with %d model groups (max 3 models each)", len(clients))
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", tierConfig.Provider)
	}

	return &modelPool{clients: clients}, nil
}
