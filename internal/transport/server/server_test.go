package server

import (
	"os"
	"testing"
	"time"

	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/storage"
	toolRegistry "github.com/hnimtadd/hive/internal/tools/registry"
	"github.com/hnimtadd/hive/pkg/config"
)

func setupTestServer(t *testing.T) (*HiveServer, func()) {
	t.Helper()

	// Create temp directory for storage
	tmpDir, err := os.MkdirTemp("", "hive-server-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create storage
	store, err := storage.NewLocalStorage(storage.Options{
		Storage: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                    0, // Use random port
			Host:                    "127.0.0.1",
			MaxTimeout:              30 * time.Second,
			GracefulShutdownTimeout: 5 * time.Second,
		},
		AI: config.AIConfig{
			Tiers: map[string]config.ModelTier{
				"smart": {
					Provider: "ollama",
					Model:    "llama3",
				},
			},
			Ollama: &config.OllamaConfig{
				BaseURL: "http://localhost:11434",
			},
		},
		Bees: config.BeeConfig{
			Dir:            tmpDir + "/bees",
			DefaultTimeout: 2 * time.Minute,
			PoolSize:       2,
		},
		Tools: config.ToolConfig{
			Dir:            tmpDir + "/tools",
			DefaultTimeout: 1 * time.Minute,
		},
		Tasks: config.TaskConfig{
			Storage: tmpDir + "/tasks",
			Timeout: 10 * time.Minute,
		},
	}

	// Create dependencies
	llmProvider, err := llm.NewLLMProvider(&cfg.AI)
	if err != nil {
		t.Fatalf("Failed to create LLM provider: %v", err)
	}

	toolReg, err := toolRegistry.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("Failed to create tool registry: %v", err)
	}

	beeReg, err := registry.NewBeeRegistry(cfg, llmProvider, toolReg)
	if err != nil {
		t.Fatalf("Failed to create bee registry: %v", err)
	}

	// Create server
	srv, err := NewHiveServer(cfg, llmProvider, beeReg, nil, store)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	cleanup := func() {
		srv.Stop()
		os.RemoveAll(tmpDir)
	}

	return srv, cleanup
}

func TestNewHiveServer(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	if srv == nil {
		t.Fatal("NewHiveServer returned nil")
	}

	if srv.taskManager == nil {
		t.Fatal("taskManager is nil")
	}

	if srv.channelManager == nil {
		t.Fatal("channelManager is nil")
	}

	if srv.workerPool == nil {
		t.Fatal("workerPool is nil")
	}

	if srv.config == nil {
		t.Fatal("config is nil")
	}
}

func TestServerStartStop(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Start server on a random port
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(":0")
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		srv.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return in time")
	}

	// Serve should return (possibly nil or error) after Stop()
	// gRPC GracefulStop returns nil when the listener closes cleanly
	<-errCh // Wait for Serve to return, value doesn't matter
}

func TestServerGracefulShutdown(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Start server
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(":0")
	}()

	time.Sleep(200 * time.Millisecond)

	// Stop should shut down both gRPC and worker pool
	stopDone := make(chan struct{})
	go func() {
		srv.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		// OK
	case <-time.After(10 * time.Second):
		t.Fatal("Graceful shutdown took too long")
	}

	// Worker pool should be stopped
	select {
	case <-srv.workerPool.Done():
		// OK - worker pool is stopped
	case <-time.After(2 * time.Second):
		t.Fatal("Worker pool did not stop")
	}
}
