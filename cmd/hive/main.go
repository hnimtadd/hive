package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	cfgFile string
	jiraID  string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "hive [command]",
	Short:   "The Hive - Distributed AI Agent Platform for Developer Productivity",
	Long:    "The Hive is a distributed AI agent platform designed to reduce developer cognitive load and automate project management through natural language CLI commands.",
	Args:    cobra.ExactArgs(1),
	Example: `hive "Update the traffic shift script to deal with 0:100 page" --jira "PROJ-123"`,
	RunE: func(_ *cobra.Command, args []string) error {
		return executeCommand(args[0])
	},
}

// executeCommand processes the natural language command.
func executeCommand(command string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	ctx := context.Background()

	if verbose {
		log.Printf("Processing command: %s\n", command)
		if jiraID != "" {
			log.Printf("Jira ID: %s\n", jiraID)
		}
	}

	// Create a new Hive task
	cc, err := grpc.NewClient(cfg.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create connecction to server: %w", err)
	}
	client := agentv1.NewAgentServiceClient(cc)

	req := &agentv1.ExecuteTaskRequest{
		GlobalGoal: command,
	}
	srv, err := client.ExecuteTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Start monitoring task progress
	return monitorTask(srv)
}

// monitorTask watches task progress and provides real-time updates.
func monitorTask(srv grpc.ServerStreamingClient[agentv1.TaskUpdate]) error {
	log.Printf("Monitoring task progress for\n")

	// Subscribe to task updates
	for {
		update, err := srv.Recv()
		if err != nil {
			return err
		}
		switch msg := update.GetPayload().(type) {
		case *agentv1.TaskUpdate_Result:
			log.Println("Task success", msg.Result.GetContent())
			return nil

		case *agentv1.TaskUpdate_Error:
			log.Printf("Task failed: %v\n", msg.Error.GetMessage())
			return errors.New("task execution failed")
		default:
			log.Println("unexpected")
			return errors.New("unexpected response")
		}
	}
}

func main() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.hive.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Command-specific flags
	rootCmd.Flags().StringVar(&jiraID, "jira", "", "Jira ticket ID to associate with the task")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
