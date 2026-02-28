package agent

import (
	"context"
	"maps"
	"slices"

	"github.com/hnimtadd/hive/pkg/types"
)

// HiveAgent defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to hashicorp gRPC plugins later.
type HiveAgent interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// GetType returns the type/category of this agent (e.g., "code_editor", "test_runner", "deployer")
	GetType() string

	// CanHandle determines if this agent can process the given task
	CanHandle(task *types.HiveTask) bool

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
	// For success task, markCompleted will be automatically call with the
	// summary by the agent, so the caller don't have to handle this manually.
	Execute(ctx context.Context, task *types.HiveTask) error

	// ReportStatus provides real-time status updates during execution
	// This is called periodically to update task progress
	ReportStatus(ctx context.Context, task *types.HiveTask) error

	// RequestFeedback pauses execution and requests human input
	// The agent should wait for feedback before continuing
	RequestFeedback(ctx context.Context, task *types.HiveTask, message string) (string, error)

	// Validate performs pre-execution validation of the task
	// Returns error if task cannot be executed due to invalid parameters
	Validate(task *types.HiveTask) error

	// Cleanup performs any necessary cleanup after task completion or failure
	Cleanup(ctx context.Context, task *types.HiveTask) error

	// GetCapabilities returns a list of capabilities this agent supports
	GetCapabilities() []string

	// Heartbeat indicates the agent is alive and ready to accept work
	// Used for agent discovery and health monitoring
	Heartbeat() error
}

// Config holds configuration for agent initialization.
type Config struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	MaxTasks     int               `json:"max_tasks"`
	Timeout      int               `json:"timeout_seconds"`
	Environment  map[string]string `json:"environment"`
	Capabilities []string          `json:"capabilities"`
}

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
func (a *agentRegistry) FindAgent(_ *types.HiveTask) (HiveAgent, error) {
	return a.agents["id"], nil
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
