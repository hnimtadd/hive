package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/server"
	"github.com/hnimtadd/hive/internal/storage"
	toolRegistry "github.com/hnimtadd/hive/internal/tools/registry"
	"github.com/hnimtadd/hive/pkg/config"
)

func main() {
	log.Println("Starting Hive Server...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	observability.Initialize(&cfg.Tracing)

	llm, err := llm.NewLLMProvider(&cfg.AI)
	if err != nil {
		log.Fatalf("failed to create llm: %v", err)
	}
	toolRegistry, err := toolRegistry.NewRegistry(cfg)
	if err != nil {
		log.Fatalf("failed to init tool registry: %s", err)
	}

	// Create agent registry
	agentRegistry, err := registry.NewBeeRegistry(cfg, llm, toolRegistry)
	if err != nil {
		log.Fatalf("failed to init registry: %s", err)
	}

	taskStorage, err := storage.NewLocalStorage(storage.Options{
		Storage: cfg.Tasks.Storage,
	})
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}

	// Start the Hive server
	hiveServer, err := server.NewHiveServer(cfg, llm, agentRegistry, taskStorage)
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
