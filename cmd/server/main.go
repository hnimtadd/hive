package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/agent"
	"github.com/hnimtadd/hive/internal/llm"
	"github.com/hnimtadd/hive/internal/server"
	"github.com/hnimtadd/hive/pkg/config"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting Hive Server Worker...")
	appConfig, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Handle shutdown signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping agent...")
	}()

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
	hiveServer, err := server.NewHiveServer(llm, registry)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	port := ":15052"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(grpcServer, hiveServer)

	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	log.Println("Agent worker stopped gracefully")
}
