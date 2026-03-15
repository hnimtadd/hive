package agent

import (
	"maps"
	"slices"
)

// Registry manages available agents in the system.
type Registry interface {
	// ListAgents returns all registered agents
	ListAgents() []HiveAgent
}

type agentRegistry struct {
	agents map[string]HiveAgent
}

// ListAgents implements [Registry].
func (a *agentRegistry) ListAgents() []HiveAgent {
	return slices.Collect(maps.Values(a.agents))
}

func NewAgentResitry() Registry {
	return &agentRegistry{
		agents: make(map[string]HiveAgent),
	}
}
