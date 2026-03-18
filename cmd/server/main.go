package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	llm, err := llm.NewLLMToolCallingClient()
	if err != nil {
		log.Fatalf("faield to create llm: %v", err)
	}

	// Create agent registry
	registry, err := agent.NewAgentResitry(appConfig)
	if err != nil {
		log.Fatalf("failed to init registry: %s", err)
	}

	// Start the Hive server
	hiveServer, err := server.NewHiveServer(redisClient, llm, registry)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	if err = hiveServer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Server execution failed: %v", err)
	}

	log.Println("Agent worker stopped gracefully")
}
