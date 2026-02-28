package agent

import (
	"context"
	"maps"
	"slices"

	"github.com/hnimtadd/hive/pkg/types"
)

// HiveAgent defines the interface that all Hive agents must implement
// This interface enables pluggable agents that can be moved to gRPC plugins later
type HiveAgent interface {
	// GetID returns the unique identifier for this agent instance
	GetID() string

	// GetType returns the type/category of this agent (e.g., "code_editor", "test_runner", "deployer")
	GetType() string

	// CanHandle determines if this agent can process the given task
	CanHandle(task *types.HiveTask) bool

	// Execute performs the main work of the task
	// Returns an error if execution fails, nil if successful
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

// AgentConfig holds configuration for agent initialization
type AgentConfig struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	MaxTasks     int               `json:"max_tasks"`
	Timeout      int               `json:"timeout_seconds"`
	Environment  map[string]string `json:"environment"`
	Capabilities []string          `json:"capabilities"`
}

// AgentRegistry manages available agents in the system
type AgentRegistry interface {
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

// FindAgent implements [AgentRegistry].
func (a *agentRegistry) FindAgent(task *types.HiveTask) (HiveAgent, error) {
	return a.agents["id"], nil
}

// GetAgent implements [AgentRegistry].
func (a *agentRegistry) GetAgent(agentID string) (HiveAgent, error) {
	return a.agents[agentID], nil
}

// GetAgentsByType implements [AgentRegistry].
func (a *agentRegistry) GetAgentsByType(agentType string) []HiveAgent {
	panic("Not implemented")
}

// ListAgents implements [AgentRegistry].
func (a *agentRegistry) ListAgents() []HiveAgent {
	return slices.Collect(maps.Values(a.agents))
}

// RegisterAgent implements [AgentRegistry].
func (a *agentRegistry) RegisterAgent(agent HiveAgent) error {
	id := agent.GetID()
	a.agents[id] = agent
	return nil
}

// UnregisterAgent implements [AgentRegistry].
func (a *agentRegistry) UnregisterAgent(agentID string) error {
	delete(a.agents, agentID)
	return nil
}

func NewAgentResitry() AgentRegistry {
	return &agentRegistry{
		agents: make(map[string]HiveAgent),
	}
}

// AgentManager handles the lifecycle of agents
type AgentManager interface {
	// StartAgent initializes and starts an agent
	StartAgent(ctx context.Context, config *AgentConfig) (HiveAgent, error)

	// StopAgent gracefully shuts down an agent
	StopAgent(ctx context.Context, agentID string) error

	// RestartAgent restarts a failed or stuck agent
	RestartAgent(ctx context.Context, agentID string) error

	// MonitorAgents continuously monitors agent health
	MonitorAgents(ctx context.Context) error
}

// FeedbackChannel represents a communication channel for human-in-the-loop feedback
type FeedbackChannel interface {
	// SendRequest sends a feedback request to the human operator
	SendRequest(ctx context.Context, taskID, message string) error

	// WaitForResponse waits for human response with timeout
	WaitForResponse(ctx context.Context, taskID string) (string, error)

	// Close closes the feedback channel
	Close() error
}

