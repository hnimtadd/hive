package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	grpcClient *grpc.ClientConn
	config     *config.Config

	channels *channel.Manager
}

func NewClient(cfg *config.Config) (*Client, error) {
	// Create a new Hive task

	return &Client{config: cfg, channels: channel.NewManager()}, nil
}

// ExecuteTask implements [agentv1.AgentServiceClient].
func (c *Client) ExecuteTask(ctx context.Context, command string) (*channel.TaskChannels, error) {
	cc, err := grpc.NewClient(c.config.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create connection to server: %w", err)
	}
	defer cc.Close()
	client := agentv1.NewAgentServiceClient(cc)
	srv, err := client.ExecuteTask(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	task := types.NewHiveTask(command, nil)
	tc := c.channels.ForTask(task.ID)

	// Start monitoring task progress
	if err := c.handleTask(srv, task, tc); err != nil {
		log.Printf("task handling failed: %s\n", err)
		_ = srv.CloseSend()
		return nil, err
	}

	return tc, nil
}

// ExecuteTaskWithChannel sends a command and returns a channel for receiving responses.
// This is a TUI-friendly version that doesn't use stdin for feedback.
func (c *Client) ExecuteTaskWithChannel(ctx context.Context, command string, responseCh chan<- *agentv1.ExecuteTaskResponse) error {
	cc, err := grpc.NewClient(c.config.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create connection to server: %w", err)
	}
	defer cc.Close()

	client := agentv1.NewAgentServiceClient(cc)
	srv, err := client.ExecuteTask(ctx)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}
	defer srv.CloseSend()

	task := types.NewHiveTask(command, nil)

	req := &agentv1.ExecuteTaskRequest{
		Payload: &agentv1.ExecuteTaskRequest_Request{
			Request: &agentv1.TaskRequest{
				GlobalGoal:       task.Goal,
				InitialArtifacts: task.Artifacts,
			},
		},
		At: timestamppb.Now(),
	}
	if err := srv.Send(req); err != nil {
		return fmt.Errorf("failed to send message to server: %w", err)
	}

	for {
		update, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil // Normal completion
			}
			return err
		}
		select {
		case responseCh <- update:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) handleTask(
	srv grpc.BidiStreamingClient[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse],
	task *types.HiveTask,
	channel *channel.TaskChannels,
) error {
	// Note: This method is primarily used in CLI mode and may block on stdin reads
	// For TUI usage, see ExecuteTaskWithChannel instead
	log.Printf("Monitoring task progress for\n")

	reader := bufio.NewReader(os.Stdin)

	req := &agentv1.ExecuteTaskRequest{
		Payload: &agentv1.ExecuteTaskRequest_Request{
			Request: &agentv1.TaskRequest{
				GlobalGoal:       task.Goal,
				InitialArtifacts: task.Artifacts,
			},
		},
		At: timestamppb.Now(),
	}
	if err := srv.Send(req); err != nil {
		return fmt.Errorf("failed to send message to server: %w", err)
	}

	// Subscribe to task updates
	for {
		update, err := srv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil // Normal completion
			}
			return err
		}
		switch msg := update.GetPayload().(type) {
		case *agentv1.ExecuteTaskResponse_Success:
			log.Println("Task success", msg.Success.GetContent())
			channel.OutputCh <- update
			return nil

		case *agentv1.ExecuteTaskResponse_Error:
			log.Printf("Task failed: %v\n", msg.Error.GetMessage())
			channel.OutputCh <- update
			return errors.New("task execution failed")

		case *agentv1.ExecuteTaskResponse_Update:
			log.Printf("Server update: %v\n", msg.Update.String())
			channel.OutputCh <- update
			continue

		case *agentv1.ExecuteTaskResponse_Feedback:
			log.Printf("Server feedback: %v\n", msg.Feedback.String())
			log.Print("Enter your answer: ")
			channel.OutputCh <- update

			var response string
			response, err = reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %w", err)
			}
			resp := &agentv1.ExecuteTaskRequest{
				At: timestamppb.Now(),
				Payload: &agentv1.ExecuteTaskRequest_Feedback{
					Feedback: &agentv1.UserFeedback{
						Feedback: response,
					},
				},
			}
			if err = srv.Send(resp); err != nil {
				log.Printf("Failed to response to server: %s", err)
				return fmt.Errorf("failed to response to server: %w", err)
			}

		default:
			log.Println("unexpected")
			return errors.New("unexpected response")
		}
	}
}
