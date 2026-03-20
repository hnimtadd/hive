package server

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type HiveServer struct {
	agentv1.AgentServiceServer

	registry   agent.Registry
	supervisor agent.SupervisorAgent
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(llm model.ToolCallingChatModel, registry agent.Registry) (*HiveServer, error) {
	persona, err := getSupervisorPersona(registry)
	if err != nil {
		return nil, err
	}
	log.Println("persona", persona)
	supervisor, err := agent.NewSupervisorAgent(&agent.Config{
		ID:          uuid.New().String(),
		Description: persona,
		MaxSteps:    3,
		LLM:         llm,
		Tools: []tool.InvokableTool{
			deletegateTool(registry),
		},
	})
	if err != nil {
		return nil, err
	}
	return &HiveServer{
		registry:   registry,
		supervisor: supervisor,
	}, nil
}

func (s *HiveServer) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	grpcServer := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(grpcServer, s)
	if err = grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (s *HiveServer) ExecuteTask(req *agentv1.ExecuteTaskRequest, srv grpc.ServerStreamingServer[agentv1.TaskUpdate]) error {
	task := types.NewHiveTask(req.GetGlobalGoal())
	maps.Copy(task.Artifacts, req.GetInitialArtifacts())

	ctx := context.Background()
loop:
	for {
		msg, err := s.supervisor.Execute(ctx, task)
		if err != nil {
			return err
		}
		switch msg.Status {
		case types.TaskStatusCompleted:
			update := &agentv1.TaskUpdate{}
			update.Payload = &agentv1.TaskUpdate_Result{
				Result: &agentv1.FinalResult{
					Content:        msg.Content,
					CompletionTime: time.Now().String(),
				},
			}
			if err = srv.Send(update); err != nil {
				log.Println("failed to feedback to user", err)
				return err
			}
			log.Println("finished", msg.Content)
			break loop
		case types.TaskStatusFailed:
			update := &agentv1.TaskUpdate{}
			update.Payload = &agentv1.TaskUpdate_Error{
				Error: &agentv1.ErrorNotice{
					Message: msg.Content,
				},
			}
			if err = srv.Send(update); err != nil {
				log.Println("failed to feedback to user", err)
				return err
			}
			log.Println("failed", msg.Content)
			break loop
		case types.TaskStatusPaused, types.TaskStatusInProgress:
			log.Println(msg.Status, msg.Content)
			break loop
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

This is the output that your response must follow this schema only, don't return anything except the raw JSON without any formatting.
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
	outputDescription, err := utils.DescribeJSONSchema[agent.SupervisorOutput]()
	if err != nil {
		return "", fmt.Errorf("failed to describe JSON schema: %w", err)
	}
	return fmt.Sprintf(persona, taskDescription, outputDescription, string(yamlBytes)), nil
}
