package agent

import (
	"fmt"
	"maps"
	"slices"

	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/pkg/config"
)

// Registry manages available agents in the system.
type Registry interface {
	// ListAgents returns all registered agents
	ListAgents() []HiveAgent
}

type registry struct {
	agents map[string]HiveAgent
	tools  map[string]tool.InvokableTool
}

// ListAgents implements [Registry].
func (a *registry) ListAgents() []HiveAgent {
	return slices.Collect(maps.Values(a.agents))
}

// scan: TODO: scan the agent folder and create agent with different persona and
// discovery tool registered also.
func (a *registry) scan(_ *config.Config) ([]HiveAgent, error) {
	return []HiveAgent{}, nil
}

func NewAgentResitry(appConfig *config.Config) (Registry, error) {
	reg := &registry{
		agents: make(map[string]HiveAgent),
	}
	agents, err := reg.scan(appConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to scan agents: %w", err)
	}
	for _, agent := range agents {
		reg.agents[agent.GetID()] = agent
	}
	return reg, nil
}
