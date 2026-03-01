package agent

import (
	"errors"
	"maps"
	"slices"

	"github.com/hnimtadd/hive/pkg/types"
)

// Registry manages available agents in the system.
type Registry interface {
	// RegisterAgent adds an agent to the registry
	RegisterAgent(agent HiveAgent) error

	// UnregisterAgent removes an agent from the registry
	UnregisterAgent(agentID string) error

	// FindAgent returns the best agent for a given task
	FindAgent(task *types.HiveTask) (HiveAgent, error)

	// ListAgents returns all registered agents
	ListAgents() []HiveAgent

	// GetAgent returns a specific agent by ID
	GetAgent(agentID string) (HiveAgent, error)

	// GetAgentsByType returns all agents of a specific type
	GetAgentsByType(agentType string) []HiveAgent
}

type agentRegistry struct {
	agents map[string]HiveAgent
}

// FindAgent implements [Registry].
func (a *agentRegistry) FindAgent(task *types.HiveTask) (HiveAgent, error) {
	for _, agent := range a.agents {
		if agent.CanHandle(task) {
			return agent, nil
		}
	}
	return nil, errors.New("failed to find agent for task")
}

// GetAgent implements [Registry].
func (a *agentRegistry) GetAgent(agentID string) (HiveAgent, error) {
	return a.agents[agentID], nil
}

// GetAgentsByType implements [Registry].
func (a *agentRegistry) GetAgentsByType(_ string) []HiveAgent {
	panic("Not implemented")
}

// ListAgents implements [Registry].
func (a *agentRegistry) ListAgents() []HiveAgent {
	return slices.Collect(maps.Values(a.agents))
}

// RegisterAgent implements [Registry].
func (a *agentRegistry) RegisterAgent(agent HiveAgent) error {
	id := agent.GetID()
	a.agents[id] = agent
	return nil
}

// UnregisterAgent implements [Registry].
func (a *agentRegistry) UnregisterAgent(agentID string) error {
	delete(a.agents, agentID)
	return nil
}

func NewAgentResitry() Registry {
	return &agentRegistry{
		agents: make(map[string]HiveAgent),
	}
}
