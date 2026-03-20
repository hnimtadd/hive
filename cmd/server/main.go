package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/internal/server"
	"github.com/hnimtadd/hive/pkg/config"
)

func main() {
	log.Println("Starting Hive Server...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	llm, err := llm.NewLLMToolCallingClient()
	if err != nil {
		log.Fatalf("faield to create llm: %v", err)
	}

	// Create agent registry
	registry, err := agent.NewAgentResitry(cfg)
	if err != nil {
		log.Fatalf("failed to init registry: %s", err)
	}

	// Start the Hive server
	hiveServer, err := server.NewHiveServer(llm, registry)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Handle shutdown signals gracefully
	go func() {
		if err = hiveServer.Serve(cfg.Server.Addr()); err != nil {
			stop()
			log.Fatalln(err)
		}
	}()
	<-ctx.Done()
	if err = ctx.Err(); err != nil {
		log.Printf("Receive stop signal %v\n", ctx.Err())
	}
	log.Println("Graceful exiting...")
	hiveServer.Stop()
	log.Println("Graceful exit complete!")
}
