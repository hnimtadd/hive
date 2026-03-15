package server

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/pkg/types"
)

type HiveServer struct {
	registry    agent.Registry
	redisClient *redis.Client
}

func NewHiveServer(redisClient *redis.Client, registry agent.Registry) *HiveServer {
	return &HiveServer{
		registry:    registry,
		redisClient: redisClient,
	}
}

func (s *HiveServer) Start(ctx context.Context) error {
	log.Println("Starting Hive Server")
	if err := s.startAgents(ctx); err != nil {
		log.Printf("failed to start agent: %v", err)
		return err
	}

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
		task.RecordState = s.recordTaskStateHandler

		// Process task through agent pipeline
		if err = s.processTask(ctx, task); err != nil {
			log.Printf("Task pipeline failed: %v", err)
			_ = task.MarkFailed(ctx, err.Error())
			_ = s.redisClient.UpdateTask(ctx, task)
		}
	}
}

func (s *HiveServer) startAgents(ctx context.Context) error {
	agents := s.registry.ListAgents()
	for a := range slices.Values(agents) {
		// Register agent
		if err := s.redisClient.RegisterAgent(ctx, a.GetID(), a.GetType()); err != nil {
			return fmt.Errorf("failed to register agent: %w", err)
		}
	}

	// Start heartbeat goroutine
	go func() {
		if err := s.doAgentHeartbeat(ctx, agents); err != nil {
			log.Printf("failed to do agent heartbeat: %v", err)
		}
	}()
	return nil
}

func (s *HiveServer) doAgentHeartbeat(ctx context.Context, agents []agent.HiveAgent) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			for a := range slices.Values(agents) {
				if err := a.Heartbeat(); err != nil {
					return fmt.Errorf("heartbeat failed: %w", err)
				}
			}
		}
	}
}

// WaitForResponse implements [agent.FeedbackChannel].
func (s *HiveServer) WaitForResponse(ctx context.Context, taskID string) (string, error) {
	return s.redisClient.WaitForFeedback(ctx, taskID)
}

func (s *HiveServer) recordTaskStateHandler(ctx context.Context, task *types.HiveTask) error {
	return s.redisClient.UpdateTask(ctx, task)
}

// processTask processes a task through the agent pipeline
// 1. Find the appropriate execution agent based on analysis
// 2. Execute the task with the selected agent
func (s *HiveServer) processTask(ctx context.Context, task *types.HiveTask) error {
	log.Printf("Processing task %s: %s", task.ID, task.Description)
	for task.Status != types.TaskStatusCompleted || task.Status != types.TaskStatusFailed {
		// Step 4: Find the appropriate execution agent
		agent, err := s.registry.FindAgent(task)
		if err != nil {
			return fmt.Errorf("failed to find execution agent: %w", err)
		}

		log.Printf("Selected agent: %s (%s)", agent.GetID(), agent.GetType())

		// Step 2: Validate task
		if err = agent.Validate(task); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		// Step 3: Execute task
		log.Printf("Executing task with %s agent...", agent.GetType())
		if err = agent.Execute(ctx, task); err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}
	}
	log.Printf("Task %s completed successfully", task.ID)
	return nil

}
