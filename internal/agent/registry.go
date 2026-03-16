package agent

import (
	"fmt"
	"maps"
	"slices"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/pkg/config"
)

// Registry manages available agents in the system.
type Registry interface {
	// ListAgents returns all registered agents
	ListAgents() []HiveAgent
	// GetByID get agent by agent ID
	GetByID(id string) (HiveAgent, bool)
}

type registry struct {
	agents map[string]HiveAgent
	tools  map[string]tool.InvokableTool
}

// ListAgents implements [Registry].
func (a *registry) ListAgents() []HiveAgent {
	return slices.Collect(maps.Values(a.agents))
}

func (a *registry) GetByID(id string) (HiveAgent, bool) {
	agent, ok := a.agents[id]
	return agent, ok

}

// scan: TODO: scan the agent folder and create agent with different persona and
// discovery tool registered also.
func (a *registry) scan(llm model.ToolCallingChatModel, _ *config.Config) ([]HiveAgent, error) {
	config := &Config{
		ID:          uuid.New().String(),
		Description: "You are an assistant agent",
		MaxSteps:    30,
		Timeout:     10,
		MaxTasks:    10,
		LLM:         llm,
		Tools:       []tool.InvokableTool{},
	}
	agent, err := NewAgent(config)
	if err != nil {
		return nil, err
	}
	return []HiveAgent{agent}, nil
}

func NewAgentResitry(appConfig *config.Config) (Registry, error) {
	reg := &registry{
		agents: make(map[string]HiveAgent),
	}
	llm, err := llm.NewLLMToolCallingClientWithConfig(&appConfig.AI)
	if err != nil {
		return nil, fmt.Errorf("failed to init llm")
	}
	agents, err := reg.scan(llm, appConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to scan agents: %w", err)
	}
	for _, agent := range agents {
		reg.agents[agent.GetID()] = agent
	}
	return reg, nil
}
