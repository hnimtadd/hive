package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/internal/server"
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
	registry := agent.NewAgentResitry()
	if err = registry.RegisterAgent(codeAgent); err != nil {
		log.Fatalf("Failed to register code agent: %v", err)
	}

	server := server.NewHiveServer(redisClient, registry)
	if err = server.Start(ctx); err != nil && errors.Is(err, context.Canceled) {
		log.Fatalf("Server execution failed: %v", err)
	}

	log.Println("Agent worker stopped gracefully")
}
