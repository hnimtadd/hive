package registry

import (
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/adrg/frontmatter"
	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/model/llm"
	toolRegistry "github.com/hnimtadd/hive/internal/tools/registry"
	"github.com/hnimtadd/hive/pkg/config"
)

// Registry manages available agents in the system.
type Registry interface {
	// ListAgents returns all registered agents
	ListAgents() []bee.CustomBee[bee.WorkerInput, bee.WorkerOutput]
	// GetByID get agent by agent ID
	GetByID(id string) (bee.CustomBee[bee.WorkerInput, bee.WorkerOutput], bool)
}

type registry struct {
	bees  map[string]bee.CustomBee[bee.WorkerInput, bee.WorkerOutput]
	tools map[string]tool.InvokableTool
	path  string
}

// ListAgents implements [Registry].
func (a *registry) ListAgents() []bee.CustomBee[bee.WorkerInput, bee.WorkerOutput] {
	return slices.Collect(maps.Values(a.bees))
}

func (a *registry) GetByID(id string) (bee.CustomBee[bee.WorkerInput, bee.WorkerOutput], bool) {
	agent, ok := a.bees[id]

	return agent, ok
}

// scan: TODO: scan the agent folder and create agent with different persona and
// discovery tool registered also.
func (a *registry) scan(cfg *config.Config) ([]bee.CustomBee[bee.WorkerInput, bee.WorkerOutput], error) {
	entries, err := os.ReadDir(a.path)
	if err != nil {
		log.Printf("failed to read agents home: %s\n", err)
		return []bee.CustomBee[bee.WorkerInput, bee.WorkerOutput]{}, nil
	}

	// Use server timeout as default for agents, with a reasonable max cap (2x default)
	defaultTimeoutSec := int(cfg.Bees.DefaultTimeout.Seconds())

	agents := []bee.CustomBee[bee.WorkerInput, bee.WorkerOutput]{}
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
		workerAgent, err := bee.NewCustomBee[bee.WorkerInput, bee.WorkerOutput](beeConfig)
		if err != nil {
			log.Printf("failed to init worker agent: %s", err)
			continue
		}
		agents = append(agents, workerAgent)
	}
	log.Printf("successfully scanned %d agents\n", len(agents))
	return agents, nil
}

func NewBeeResitry(appConfig *config.Config, tools toolRegistry.Registry) (Registry, error) {
	agentTools := tools.ListTools()
	log.Println("available tools", agentTools)
	reg := &registry{
		bees:  make(map[string]bee.CustomBee[bee.WorkerInput, bee.WorkerOutput]),
		tools: agentTools,
		path:  appConfig.Bees.Dir,
	}
	agents, err := reg.scan(appConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to scan agents: %w", err)
	}
	for _, agent := range agents {
		reg.bees[agent.GetID()] = agent
	}
	return reg, nil
}

func LoadAgentConfig(path string) (*bee.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg bee.Config
	// This reads the --- YAML --- block into 'cfg'
	// and returns the remaining Markdown as 'persona'
	persona, err := frontmatter.Parse(f, &cfg)
	if err != nil {
		return nil, err
	}

	// Set the Markdown body as the Description/Persona
	cfg.Persona = string(persona)

	return &cfg, nil
}
