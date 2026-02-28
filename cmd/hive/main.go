package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hnimtadd/hive/internal/parser"
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

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "hive [command]",
	Short: "The Hive - Distributed AI Agent Platform for Developer Productivity",
	Long: `The Hive is a distributed AI agent platform designed to reduce developer
cognitive load and automate project management through natural language CLI commands.

Example usage:
  hive "Update the traffic shift script to deal with 0:100 page" --jira "PROJ-123"
  hive "Fix the authentication bug in the login handler" --jira "AUTH-456"
  hive "Add unit tests for the payment service" --jira "TEST-789"`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeCommand(args[0])
	},
}

// executeCommand processes the natural language command
func executeCommand(command string) error {
	ctx := context.Background()

	if verbose {
		fmt.Printf("Processing command: %s\n", command)
		if jiraID != "" {
			fmt.Printf("Jira ID: %s\n", jiraID)
		}
	}

	// Parse the natural language intent
	intent, err := parser.ParseIntent(command, jiraID)
	if err != nil {
		return fmt.Errorf("failed to parse command intent: %w", err)
	}

	if verbose {
		fmt.Printf("Parsed intent: %+v\n", intent)
	}

	// Create a new Hive task
	task := types.NewHiveTask(intent.Goal, intent.JiraID)
	task.Title = intent.Title
	task.Description = intent.Description
	task.Command = command
	task.WorkingDir, _ = os.Getwd()

	// Initialize Redis client
	redisClient, err := redis.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer redisClient.Close()

	// Submit task to the task queue
	if err := redisClient.SubmitTask(ctx, task); err != nil {
		return fmt.Errorf("failed to submit task: %w", err)
	}

	fmt.Printf("Task submitted successfully with ID: %s\n", task.ID)

	// Start monitoring task progress
	return monitorTask(ctx, redisClient, task.ID)
}

// monitorTask watches task progress and provides real-time updates
func monitorTask(ctx context.Context, redisClient *redis.Client, taskID string) error {
	fmt.Printf("Monitoring task progress for: %s\n", taskID)

	// Subscribe to task updates
	updates, err := redisClient.SubscribeToTaskUpdates(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to task updates: %w", err)
	}

	for update := range updates {
		switch update.Status {
		case types.TaskStatusInProgress:
			fmt.Printf("Task in progress: %s (%.1f%%)\n", update.Description, update.Progress)
		case types.TaskStatusPaused:
			if update.RequiresFeedback {
				fmt.Printf("  Task paused - feedback required: %s\n", update.FeedbackMessage)
				// Handle feedback request
				if err := handleFeedbackRequest(ctx, redisClient, taskID, update.FeedbackMessage); err != nil {
					return err
				}
			}
		case types.TaskStatusCompleted:
			fmt.Printf("Task completed successfully!\n")
			fmt.Printf("Summary: %s\n", update.ExecutionSummary)
			if update.LinesChanged > 0 {
				fmt.Printf("Lines changed: %d\n", update.LinesChanged)
			}
			if len(update.FilesModified) > 0 {
				fmt.Printf("Files modified: %v\n", update.FilesModified)
			}
			return nil
		case types.TaskStatusFailed:
			fmt.Printf("Task failed: %s\n", update.ErrorMessage)
			return fmt.Errorf("task execution failed")
		}
	}

	return nil
}

// handleFeedbackRequest prompts the user for feedback and sends it back
func handleFeedbackRequest(ctx context.Context, redisClient *redis.Client, taskID, message string) error {
	fmt.Printf("\nHuman input required:\n%s\n", message)
	fmt.Print("Your response: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	return redisClient.ProvideFeedback(ctx, taskID, response)
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status [task-id]",
	Short: "Check the status of a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		taskID := args[0]

		redisClient, err := redis.NewClient()
		if err != nil {
			return fmt.Errorf("failed to connect to Redis: %w", err)
		}
		defer redisClient.Close()

		task, err := redisClient.GetTask(ctx, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		fmt.Printf("Task ID: %s\n", task.ID)
		fmt.Printf("Status: %s\n", task.Status)
		fmt.Printf("Goal: %s\n", task.Goal)
		fmt.Printf("Progress: %.1f%%\n", task.Progress)
		if task.AssignedAgent != "" {
			fmt.Printf("Assigned Agent: %s\n", task.AssignedAgent)
		}
		if task.ErrorMessage != "" {
			fmt.Printf("Error: %s\n", task.ErrorMessage)
		}

		return nil
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Println("No active tasks found.")
			return nil
		}

		fmt.Printf("Active Tasks (%d):\n", len(tasks))
		for _, task := range tasks {
			fmt.Printf("  %s - %s [%s] %.1f%%\n",
				task.ID[:8], task.Goal, task.Status, task.Progress)
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
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
}

// initConfig reads in config file and ENV variables if set
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
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
