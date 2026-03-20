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
	"google.golang.org/protobuf/types/known/timestamppb"
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

	srv, err := client.ExecuteTask(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Start monitoring task progress
	return handleTask(srv, command)
}

// monitorTask watches task progress and provides real-time updates.
func handleTask(srv grpc.BidiStreamingClient[agentv1.ClientMessage, agentv1.ServerMessage], command string) error {
	log.Printf("Monitoring task progress for\n")

	req := &agentv1.ClientMessage{
		Payload: &agentv1.ClientMessage_Request{
			Request: &agentv1.TaskRequest{
				GlobalGoal: command,
			},
		},
	}
	if err := srv.Send(req); err != nil {
		return fmt.Errorf("failed to send message to server: %w", err)
	}

	// Subscribe to task updates
	for {
		update, err := srv.Recv()
		if err != nil {
			return err
		}
		switch msg := update.GetPayload().(type) {
		case *agentv1.ServerMessage_Success:
			log.Println("Task success", msg.Success.GetContent())
			return nil

		case *agentv1.ServerMessage_Error:
			log.Printf("Task failed: %v\n", msg.Error.GetMessage())
			return errors.New("task execution failed")

		case *agentv1.ServerMessage_Update:
			log.Printf("Server update: %v\n", msg.Update.String())
			continue

		case *agentv1.ServerMessage_Feedback:
			log.Printf("Server feedback: %v\n", msg.Feedback.String())
			var response string

			log.Print("Enter your answer: ")
			// Use & to pass the variable by reference so Scanln can modify it
			_, err = fmt.Scanln(&response)
			if err != nil {
				return fmt.Errorf("error reading input: %w", err)
			}
			resp := &agentv1.ClientMessage{
				At: timestamppb.Now(),
				Payload: &agentv1.ClientMessage_Feedback{
					Feedback: &agentv1.UserFeedback{
						Feedback: response,
					},
				},
			}
			if err = srv.Send(resp); err != nil {
				log.Printf("Failed to resposne to server: %s", err)
				return fmt.Errorf("failed to resposne to server: %w", err)
			}

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
