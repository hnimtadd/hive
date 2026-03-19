package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
	"gopkg.in/yaml.v3"
)

type HiveServer struct {
	registry    agent.Registry
	redisClient *redis.Client
	supervisor  agent.SupervisorAgent
}

func NewHiveServer(redisClient *redis.Client, llm model.ToolCallingChatModel, registry agent.Registry) (*HiveServer, error) {
	persona, err := getSupervisorPersona(registry)
	if err != nil {
		return nil, err
	}
	supervisor, err := agent.NewSupervisorAgent(&agent.Config{
		ID:          uuid.New().String(),
		Description: persona,
		MaxSteps:    3,
		LLM:         llm,
		Tools: []tool.InvokableTool{
			agentLookupTool(registry),
		},
	})
	if err != nil {
		return nil, err
	}
	return &HiveServer{
		registry:    registry,
		redisClient: redisClient,
		supervisor:  supervisor,
	}, nil
}

func (s *HiveServer) Start(ctx context.Context) error {
	log.Println("Starting Hive Server")

	// Main task processing loop
	for {
		select {
		case <-ctx.Done():
			log.Printf("Server shutting down")
			return ctx.Err()
		default:
		}

		// Get next task from queue
		task, err := s.redisClient.GetNextTask(ctx)
		if err != nil {
			log.Printf("Failed to get next task: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if task == nil {
			// No tasks available, wait a bit
			time.Sleep(1 * time.Second)
			continue
		}

		// Process task through agent pipeline
		if err = s.processTask(ctx, task); err != nil {
			log.Printf("Task pipeline failed: %v", err)
			// _ = task.MarkFailed(ctx, err.Error())
			_ = s.redisClient.UpdateTask(ctx, task)
		}
	}
}

// processTask processes a task through the agent pipeline
// 1. Find the appropriate execution agent based on analysis
// 2. Execute the task with the selected agent.
func (s *HiveServer) processTask(ctx context.Context, task *types.HiveTask) error {
	ctx = types.ContextWithTask(ctx, task)
	for task.Status != types.TaskStatusCompleted || task.Status != types.TaskStatusFailed {
		msg, err := s.supervisor.Execute(ctx, task)
		if err != nil {
			return err
		}
		switch msg.Status {
		case "FINISHED":
			fmt.Println("finished", msg.Content)
		case "INTERRUPT":
			fmt.Println("interrupt", msg.Content)
		case "FAILED":
			fmt.Println("failed", msg.Content)
		default:
			// Handle unexpected "stuck" states
		}
	}
	return nil
}

func getSupervisorPersona(registry agent.Registry) (string, error) {
	agents := registry.ListAgents()
	persona := `
Role: You are the Central Orchestrator for a multi-agent swarm. Your goal is to navigate a complex task to completion by delegating to specialized workers.
Core Responsibilities:
	- Analyze State: Review the conversation history. Identify what has been achieved and what is still missing.
    - Prevent Redundancy: If a supervisee has already failed at a specific approach, do not assign them the same task again without new instructions.
    - Evaluate Capabilities: Match the requirements of the next step against the specific tools and expertise of the available agents.
	- Terminate with Purpose:
		* Output FINISH if the user's goal is met along with the information
        * Output FAILED if the available agents lack the capabilities to proceed or if a logical dead-end is reached.
Constraint: Do not perform the task yourself. Your only tools are delegation and synthesis.
This is the task state, which is your input that you and your team are working with:
%s
%This is the output that you have to return:
%s
Available Agents:
%s
`
	prompts := map[string]string{}
	for _, agent := range agents {
		prompts[agent.GetID()] = agent.Description()
	}
	yamlBytes, err := yaml.Marshal(prompts)
	if err != nil {
		return "", fmt.Errorf("failed to build system prompt: %w", err)
	}
	taskDescription, err := utils.DescribeJSONSchema[types.HiveTask]()
	if err != nil {
		return "", fmt.Errorf("failed to describe JSON schema: %w", err)
	}
	return fmt.Sprintf(persona, taskDescription, string(yamlBytes)), nil
}
