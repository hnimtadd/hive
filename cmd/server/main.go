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

	// Initialize LLM client for enhanced agent
	llmClient, err := llm.NewLLMToolCallingClient()
	if err != nil {
		log.Fatalf("Failed to initialize LLM client: %v", err)
	}

	// Create and start the enhanced coder agent
	coderAgent, err := coder.NewCoderAgent(llmClient)
	if err != nil {
		log.Fatalf("Failed to create enhanced coder agent: %v", err)
	}
	log.Printf("Coder Agent %s started", coderAgent.GetID())

	// TODO: handle this part automatically
	registry := agent.NewAgentResitry()
	if err = registry.RegisterAgent(coderAgent); err != nil {
		log.Fatalf("Failed to register enhanced coder agent: %v", err)
	}

	server := server.NewHiveServer(redisClient, registry)
	if err = server.Start(ctx); err != nil && errors.Is(err, context.Canceled) {
		log.Fatalf("Server execution failed: %v", err)
	}

	log.Println("Agent worker stopped gracefully")
}
