package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hnimtadd/hive/internal/bee"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/server"
	"github.com/hnimtadd/hive/internal/tools"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/config"
)

func main() {
	log.Println("Starting Hive Server...")
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize tracing
	if cfg.Tracing.Enabled {
		logOutput := os.Stdout
		if cfg.Tracing.LogFile != "" {
			f, err := os.OpenFile(cfg.Tracing.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalf("failed to open log file: %v", err)
			}
			defer f.Close()
			logOutput = f
		}

		trace.InitLogger(&trace.LogConfig{
			Level:     trace.ParseLogLevel(cfg.Tracing.LogLevel),
			Format:    cfg.Tracing.LogFormat,
			Output:    logOutput,
			AddSource: cfg.Tracing.AddSource,
		})

		log.Printf("Tracing initialized: level=%s format=%s", cfg.Tracing.LogLevel, cfg.Tracing.LogFormat)
	}

	llm, err := llm.NewLLMToolCallingClient()
	if err != nil {
		log.Fatalf("faield to create llm: %v", err)
	}
	toolRegistry, err := tools.NewRegistry(cfg)
	if err != nil {
		log.Fatalf("failed to init tool registry: %s", err)
	}

	// Create agent agentRegistry
	agentRegistry, err := bee.NewBeeResitry(cfg, toolRegistry)
	if err != nil {
		log.Fatalf("failed to init registry: %s", err)
	}

	// Start the Hive server
	hiveServer, err := server.NewHiveServer(cfg, llm, agentRegistry)
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
