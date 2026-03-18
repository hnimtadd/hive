package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/hnimtadd/hive/internal/redis"
	"github.com/hnimtadd/hive/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	task := types.NewHiveTask(command)

	// Initialize Redis client
	redisClient, err := redis.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	// Submit task to the task queue
	if err = redisClient.SubmitTask(ctx, task); err != nil {
		return fmt.Errorf("failed to submit task: %w", err)
	}

	log.Printf("Task submitted successfully with ID: %s\n", task.ID)

	// Start monitoring task progress
	return monitorTask(ctx, redisClient, task.ID)
}

// monitorTask watches task progress and provides real-time updates.
func monitorTask(ctx context.Context, redisClient *redis.Client, taskID string) error {
	log.Printf("Monitoring task progress for: %s\n", taskID)

	// Subscribe to task updates
	updates, err := redisClient.SubscribeToTaskUpdates(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to task updates: %w", err)
	}

	for update := range updates {
		switch update.Status {
		case types.TaskStatusInProgress:
			log.Printf("%s (%.1f%%)\n", update.Status)

		case types.TaskStatusPaused:

		case types.TaskStatusCompleted:
			log.Printf("Task completed successfully!\n")
			log.Printf("%s\n", update.Messages[len(update.Messages)-1])
			// Show Gitlab-specific information if available
			return nil

		case types.TaskStatusFailed:
			log.Printf("Task failed: %s\n", update.Errors[len(update.Errors)-1])
			return errors.New("task execution failed")
		}
	}

	return nil
}

// handleFeedbackRequest prompts the user for feedback and sends it back.
func handleFeedbackRequest(ctx context.Context, redisClient *redis.Client, taskID, message string) error {
	log.Printf("\nHuman input required:\n%s\n", message)
	fmt.Print("Your response: ") //nolint: forbidigo // this should be print inline

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	return redisClient.ProvideFeedback(ctx, taskID, response)
}

// listCmd represents the list command.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active tasks",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		redisClient, err := redis.NewClient()
		if err != nil {
			return fmt.Errorf("failed to connect to Redis: %w", err)
		}
		defer redisClient.Close()

		tasks, err := redisClient.ListActiveTasks(ctx)
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}

		if len(tasks) == 0 {
			log.Println("No active tasks found.")
			return nil
		}

		log.Printf("Active Tasks (%d):\n", len(tasks))
		for _, task := range tasks {
			log.Printf("  %s  [%s]\n",
				task.ID[:8], task.Status)
		}

		return nil
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.hive.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Command-specific flags
	rootCmd.Flags().StringVar(&jiraID, "jira", "", "Jira ticket ID to associate with the task")

	// Add subcommands
	rootCmd.AddCommand(listCmd)
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
