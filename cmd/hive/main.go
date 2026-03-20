package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	Use:   "hive [command]",
	Short: "The Hive - Distributed AI Agent Platform for Developer Productivity",
	Long: `The Hive is a distributed AI agent platform designed to reduce developer
cognitive load and automate project management through natural language CLI commands.

Example usage:
  # Simple task (legacy mode)
  hive "Update the traffic shift script to deal with 0:100 page" --jira "PROJ-123"

  # AI-powered feature development
  hive "Add user authentication with JWT tokens" --jira "AUTH-456" --gitlab-project "mygroup/myrepo" --target-branch "main"
  hive "Implement rate limiting for API endpoints" --gitlab-project "mygroup/myrepo" --feature "Rate limiting with Redis backend"`,

	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return executeCommand(args[0])
	},
}

// executeCommand processes the natural language command.
func executeCommand(command string) error {
	ctx := context.Background()

	if verbose {
		log.Printf("Processing command: %s\n", command)
		if jiraID != "" {
			log.Printf("Jira ID: %s\n", jiraID)
		}
	}

	// Create a new Hive task
	cc, err := grpc.NewClient("localhost:15052", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	return nil
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.hive.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Command-specific flags
	rootCmd.Flags().StringVar(&jiraID, "jira", "", "Jira ticket ID to associate with the task")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".hive")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil && verbose {
		log.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
