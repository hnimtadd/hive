// Integration example for coder agent with the server component
package main

import (
	"context"
	"log"

	"github.com/hnimtadd/hive/agents/coder"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

func SetupAgentWorkflow() (agent.HiveAgent, error) {
	// Create LLM model (assuming Claude or OpenAI)
	chatModel, err := llm.NewLLMClient()
	if err != nil {
		return nil, err
	}

	// coder agent will auto-discover Jira config if enabled
	appConfig, err := config.LoadConfig()
	if err != nil {
		// Use minimal config without Jira if config file not found
		panic("failed to load config")
	}

	// Create coder agent (task analyzer and router)
	coderAgent, err := coder.NewAgent(chatModel, appConfig)
	if err != nil {
		return nil, err
	}
	return coderAgent, nil
}

func main() {
	// Set up the workflow
	agent, err := SetupAgentWorkflow()
	if err != nil {
		log.Fatal("Failed to setup agent workflow:", err)
	}

	// Example task that would benefit from analysis
	task := &types.HiveTask{
		ID:                "task-123",
		JiraID:            "T6-1274",
		Description:       "I want to add the README file to dat.nguyen/something repo, the content of the README is how to write helloworld program in 3 different programming languages, let make MR after you complete",
		GitlabProjectPath: "dat.nguyen/something",
		SourceBranch:      "main",
		TargetBranch:      "feature",
	}

	if err = agent.Execute(context.Background(), task); err != nil {
		log.Panic(err)
	}
}
