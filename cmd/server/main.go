package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/redis"
)

func main() {
	log.Println("Starting Hive Server Worker...")

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping agent...")
		cancel()
	}()

	// Initialize Redis client
	redisClient, err := redis.NewClient()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Create and start the code editor agent
	codeAgent, err := agent.NewCodeEditorAgent(redisClient)
	if err != nil {
		log.Fatalf("Failed to create code editor agent: %v", err)
	}

	log.Printf("Code Editor Agent %s started and ready for tasks", codeAgent.GetID())

	// Start the agent (this blocks until context is cancelled)
	if err := codeAgent.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Agent execution failed: %v", err)
	}

	log.Println("Agent worker stopped gracefully")
}
