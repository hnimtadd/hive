package bee

import (
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/pkg/config"
)

// Registry manages available agents in the system.
type Registry interface {
	// ListAgents returns all registered agents
	ListAgents() []WorkerBee
	// GetByID get agent by agent ID
	GetByID(id string) (WorkerBee, bool)
}

type registry struct {
	agents map[string]WorkerBee
	tools  map[string]tool.InvokableTool
	path   string
}

// ListAgents implements [Registry].
func (a *registry) ListAgents() []WorkerBee {
	return slices.Collect(maps.Values(a.agents))
}

func (a *registry) GetByID(id string) (WorkerBee, bool) {
	agent, ok := a.agents[id]

	return agent, ok
}

// scan: TODO: scan the agent folder and create agent with different persona and
// discovery tool registered also.
func (a *registry) scan(cfg *config.Config) ([]WorkerBee, error) {
	entries, err := os.ReadDir(a.path)
	if err != nil {
		log.Printf("failed to read agents home: %s\n", err)
		return []WorkerBee{}, nil
	}

	// Use server timeout as default for agents, with a reasonable max cap (2x default)
	defaultTimeoutSec := int(cfg.Server.Timeout.Seconds())

	agents := []WorkerBee{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mdPath := filepath.Join(a.path, entry.Name(), "agent.md")
		beeConfig, err := LoadAgentConfig(mdPath) //nolint: govet// ignore lint
		if err != nil {
			log.Printf("failed to load agent configuration from :%s, err: %s\n", mdPath, err)
			continue
		}

		// Apply default timeout if not specified, cap at max
		if beeConfig.TimeoutInSec <= 0 {
			beeConfig.TimeoutInSec = defaultTimeoutSec
			log.Printf("Agent %s: using default timeout %ds", entry.Name(), defaultTimeoutSec)
		} else if beeConfig.TimeoutInSec > defaultTimeoutSec {
			log.Printf("Agent %s: timeout %ds exceeds max %ds, capping",
				entry.Name(), beeConfig.TimeoutInSec, defaultTimeoutSec)
			beeConfig.TimeoutInSec = defaultTimeoutSec
		}
		llm, err := llm.NewLLMToolCallingClientWithConfig(&cfg.AI)
		if err != nil {
			return nil, fmt.Errorf("failed to init llm: %w", err)
		}
		tools := []tool.InvokableTool{}
		for _, required := range beeConfig.RequiredTools {
			tool, ok := a.tools[required]
			if !ok {
				log.Printf("tools not found, let install it first: %s\n", required)
				continue
			}
			log.Printf("attached tool: %s", required)
			tools = append(tools, tool)
		}
		beeConfig.ID = entry.Name() + "-" + beeConfig.ID
		beeConfig.Tools = tools
		beeConfig.LLM = llm
		workerAgent, err := NewWorkerBee(beeConfig)
		if err != nil {
			log.Printf("failed to init worker agent: %s", err)
			continue
		}
		agents = append(agents, workerAgent)
	}
	log.Printf("successfully scanned %d agents\n", len(agents))
	return agents, nil
}

func NewBeeResitry(appConfig *config.Config, tools tools.Registry) (Registry, error) {
	agentTools := tools.ListTools()
	log.Println("available tools", agentTools)
	reg := &registry{
		agents: make(map[string]WorkerBee),
		tools:  agentTools,
		path:   appConfig.BeesDir,
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
