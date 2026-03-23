package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"net"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/mapper"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/hnimtadd/hive/pkg/utils"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type HiveServer struct {
	agentv1.AgentServiceServer

	registry   agent.Registry
	supervisor agent.SupervisorAgent

	grpcServer *grpc.Server
}

var _ agentv1.AgentServiceServer = &HiveServer{}

func NewHiveServer(llm model.ToolCallingChatModel, registry agent.Registry) (*HiveServer, error) {
	persona, err := getSupervisorPersona(registry)
	if err != nil {
		return nil, err
	}
	supervisor, err := agent.NewSupervisorAgent(&agent.Config{
		ID:          uuid.New().String(),
		Description: persona,
		MaxSteps:    3,
		LLM:         llm,
		Tools:       []tool.InvokableTool{agent.DelegateTool(registry)},
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
	s.grpcServer = grpcServer
	if err = grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

func (s *HiveServer) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

func (s *HiveServer) ExecuteTask(srv grpc.BidiStreamingServer[agentv1.ClientMessage, agentv1.ServerMessage]) error {
	msg, err := srv.Recv()
	if err != nil {
		return err
	}
	req := msg.GetRequest()
	if req == nil {
		return errors.New("first message must be a task request")
	}
	task := types.NewHiveTask(req.GetGlobalGoal())
	maps.Copy(task.Artifacts, req.GetInitialArtifacts())

	ctx := context.Background()
loop:
	for {
		output, err := s.supervisor.Execute(ctx, task)
		if err != nil {
			return err
		}
		switch output.Status {
		case types.TaskStatusCompleted:
			update := mapper.ToTaskUpdateSuccess(output)
			if err = srv.Send(update); err != nil {
				log.Println("failed to feedback to user", err)
				return err
			}
			log.Println("finished", output.Content)
			break loop

		case types.TaskStatusFailed:
			update := mapper.ToTaskUpdateFailed(output)
			if err = srv.Send(update); err != nil {
				log.Println("failed to feedback to user", err)
				return err
			}
			log.Println("failed", output.Content)
			break loop

		case types.TaskStatusPaused:
			update := mapper.ToTaskUpdateRequireFeedback(output)
			if err = srv.Send(update); err != nil {
				log.Printf("failed to require feedback from user: %v\n", err)
				return err
			}
			for {
				var msg *agentv1.ClientMessage
				msg, err = srv.Recv()
				if err != nil {
					log.Printf("failed to receive message from user: %s\n", err)
					return err
				}
				feedback := msg.GetFeedback()
				if feedback == nil {
					log.Printf("feedback user is required")
					continue
				}
				// TODO: feed feedback to model
				log.Println("user feedback", feedback.String())
				break
			}

		// TODO: mock the event stream to the internal model so client could have a visibility on tool calling and thoughts here
		case types.TaskStatusInProgress:
			update := mapper.ToTaskUpdateInProgress(output)
			if err = srv.Send(update); err != nil {
				log.Printf("failed to sent in progress from user: %v\n", err)
				return err
			}

			log.Println(output.Status, output.Content)
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
