// Integration example for Analyzer agent with the server component
package main

import (
	"context"
	"log"

	"github.com/hnimtadd/hive/agents/analyzer"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

func SetupAgentWorkflow() (agent.WorkerAgent, error) {
	// Create LLM model (assuming Claude or OpenAI)
	chatModel, err := llm.NewLLMClient()
	if err != nil {
		return nil, err
	}

	// Analyzer agent will auto-discover Jira config if enabled
	appConfig, err := config.LoadConfig()
	if err != nil {
		// Use minimal config without Jira if config file not found
		panic("failed to load config")
	}

	// Create Analyzer agent (task analyzer and router)
	AnalyzerAgent, err := analyzer.NewAnalyzerAgent(chatModel, appConfig)
	if err != nil {
		return nil, err
	}
	return AnalyzerAgent, nil
}

func main() {
	// Set up the workflow
	agent, err := SetupAgentWorkflow()
	if err != nil {
		log.Fatal("Failed to setup agent workflow:", err)
	}

	// Example task that would benefit from analysis
	task := &types.HiveTask{
		ID:     "task-123",
		JiraID: "T6-1274",
	}

	// The task will be automatically routed through Analyzer agent first
	// 1. Analyzer agent analyzes the task:
	//    - Type: coding (contains "implement", "authentication")
	//    - Complexity: complex (multiple systems: JWT + rate limiting)
	//    - Required Skills: [go_programming, api_development, security, authentication]
	//    - Prerequisites: [repository_access, database_connection, api_credentials]
	//    - Risk Factors: [security_considerations, performance_impact]
	//    - Estimated Steps: 10
	//
	// 2. Analyzer agent enhances task with analysis context
	// 3. Analyzer agent delegates to coder agent with enriched context
	// 4. Coder agent executes with better understanding of requirements

	if err := agent.Execute(context.Background(), task); err != nil {
		log.Panic(err)
	}
}
