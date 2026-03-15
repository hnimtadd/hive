package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hnimtadd/hive/agents/coder"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/internal/server"
	"github.com/hnimtadd/hive/pkg/config"
)

func main() {
	log.Println("Starting Hive Server Worker...")
	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	// Initialize LLM client for agents
	llmClient, err := llm.NewLLMToolCallingClient()
	if err != nil {
		log.Fatalf("Failed to initialize LLM client: %v", err)
	}

	// Create agent registry
	registry := agent.NewAgentResitry()

	// Create and register Coder Agent
	coderAgent, err := coder.NewAgent(llmClient, appConfig)
	if err != nil {
		log.Fatalf("Failed to create coder agent: %v", err)
	}

	// Start the Hive server
	hiveServer := server.NewHiveServer(redisClient, registry)
	if err = hiveServer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Server execution failed: %v", err)
	}

	log.Println("Agent worker stopped gracefully")
}
