package agent

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/andygrunwald/go-jira"
	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/errors"
)

// Registry manages available agents in the system.
type Registry interface {
	// ListAgents returns all registered agents
	ListAgents() []WorkerAgent
	// GetByID get agent by agent ID
	GetByID(id string) (WorkerAgent, bool)
}

type registry struct {
	agents map[string]WorkerAgent
	tools  map[string]tool.InvokableTool
}

// ListAgents implements [Registry].
func (a *registry) ListAgents() []WorkerAgent {
	return slices.Collect(maps.Values(a.agents))
}

func (a *registry) GetByID(id string) (WorkerAgent, bool) {
	agent, ok := a.agents[id]

	return agent, ok
}

// scan: TODO: scan the agent folder and create agent with different persona and
// discovery tool registered also.
func (a *registry) scan(cfg *config.Config) ([]WorkerAgent, error) {
	entries, err := os.ReadDir(cfg.Agents.Dir)
	if err != nil {
		log.Printf("failed to read agents home: %s\n", err)
		return []WorkerAgent{}, nil
	}
	agents := []WorkerAgent{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mdPath := filepath.Join(cfg.Agents.Dir, entry.Name(), "agent.md")
		config, err := LoadAgentConfig(mdPath) //nolint: govet// ignore lint
		if err != nil {
			log.Printf("failed to load agent configuration from :%s, err: %s\n", mdPath, err)
		}

		llm, err := llm.NewLLMToolCallingClientWithConfig(&cfg.AI)
		if err != nil {
			return nil, fmt.Errorf("failed to init llm: %w", err)
		}
		tools := []tool.InvokableTool{}
		for _, required := range config.RequiredTools {
			tool, ok := a.tools[required]
			if !ok {
				log.Printf("tools not found, let install it first: %s\n", required)
				continue
			}
			log.Printf("attached tool: %s", required)
			tools = append(tools, tool)
		}
		config.ID = entry.Name() + "-" + config.ID
		config.Tools = tools
		config.LLM = llm
		workerAgent, err := NewWorkerAgent(config)
		if err != nil {
			log.Printf("failed to init worker agent: %s", err)
			continue
		}
		agents = append(agents, workerAgent)
	}
	log.Printf("successfully scanned %d agents\n", len(agents))
	return agents, nil
}

func NewAgentResitry(appConfig *config.Config) (Registry, error) {
	agentTools := map[string]tool.InvokableTool{}
	if appConfig.Jira.Enabled {
		// Get API token from environment
		apiToken := os.Getenv(appConfig.Jira.APITokenEnv)
		if apiToken == "" {
			return nil, fmt.Errorf("jira API token not found in environment variable %s", appConfig.Jira.APITokenEnv)
		}

		tp := jira.BasicAuthTransport{
			Username: appConfig.Jira.UserName,
			Password: apiToken,
		}
		// Create Jira client and wrap it in a tool
		jiraClient, err := jira.NewClient(tp.Client(), appConfig.Jira.BaseURL)
		if err != nil {
			return nil, errors.ErrInternal("failed to create jira client", err)
		}
		jiraTool := tools.NewJiraTool(jiraClient, appConfig.Jira.CustomFields)
		info, _ := jiraTool.Info(context.Background())
		agentTools[info.Name] = jiraTool

		log.Printf("Jira integration enabled for analyzer agent: %s\n", appConfig.Jira.BaseURL)
	}
	{
		fsTool := tools.NewListFilesTool(appConfig.WorkspaceDir)
		info, _ := fsTool.Info(context.Background())
		agentTools[info.Name] = fsTool
	}
	log.Println("available tools", agentTools)
	reg := &registry{
		agents: make(map[string]WorkerAgent),
		tools:  agentTools,
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
