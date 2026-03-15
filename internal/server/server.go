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
	return nil
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
	panic("not implemented")
}
