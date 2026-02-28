package server

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/redis"
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
			log.Printf("%v", err)
		}
	}()

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

		a, err := s.registry.FindAgent(task)
		if err != nil {
			log.Printf("failed to find agent for the task: %v", err)

			if err = s.redisClient.SubmitTask(ctx, task); err != nil {
				log.Printf("Failed to requeue task: %v", err)
			}
			continue
		}

		// Validate and execute task
		if err = a.Validate(task); err != nil {
			task.MarkFailed(fmt.Sprintf("Validation failed: %v", err))
			_ = s.redisClient.UpdateTask(ctx, task)
			continue
		}

		// Execute task
		if err = a.Execute(ctx, task); err != nil {
			log.Printf("Task execution failed: %v", err)
			task.MarkFailed(err.Error())
			_ = s.redisClient.UpdateTask(ctx, task)
		}

		// Cleanup regardless of success or failure
		if err = a.Cleanup(ctx, task); err != nil {
			log.Printf("Cleanup failed: %v", err)
		}
	}
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
