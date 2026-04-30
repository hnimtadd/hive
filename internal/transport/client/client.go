package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/hnimtadd/hive/internal/channel"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	config *config.Config

	channels  *channel.Manager
	streamsMu sync.RWMutex
	streams   map[string]grpc.ClientStream // Active streams keyed by taskID
}

func NewClient(cfg *config.Config) (*Client, error) {
	// Create a new Hive task

	return &Client{
		config:   cfg,
		channels: channel.NewManager(),
		streams:  make(map[string]grpc.ClientStream),
	}, nil
}

// SendFeedback sends feedback response to a running task through its active bidirectional stream.
func (c *Client) SendFeedback(ctx context.Context, taskID string, feedback string) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	if taskID == "" {
		return fmt.Errorf("taskID cannot be empty")
	}

	// Acquire read lock to safely access the stream
	c.streamsMu.RLock()
	stream, exists := c.streams[taskID]
	c.streamsMu.RUnlock()

	if !exists {
		return fmt.Errorf("no active stream found for taskID: %s", taskID)
	}

	req := &agentv1.ExecuteTaskRequest{
		At: timestamppb.Now(),
		Payload: &agentv1.ExecuteTaskRequest_Feedback{
			Feedback: &agentv1.UserFeedback{
				Feedback: feedback,
			},
		},
	}

	// Send feedback through the stream
	if err := stream.SendMsg(req); err != nil {
		// Acquire write lock to safely delete the stream on error
		c.streamsMu.Lock()
		delete(c.streams, taskID)
		c.streamsMu.Unlock()
		return fmt.Errorf("failed to send feedback: %w", err)
	}

	return nil
}

// ExecuteTaskWithChannel sends a command and returns a channel for receiving responses.
// This is a TUI-friendly version that doesn't use stdin for feedback.
func (c *Client) ExecuteTaskWithChannel(ctx context.Context, command string) (<-chan *agentv1.ExecuteTaskResponse, error) {
	cc, err := grpc.NewClient(c.config.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create connection to server: %w", err)
	}
	responseCh := make(chan *agentv1.ExecuteTaskResponse, 100)

	go func() {
		defer cc.Close()
		defer close(responseCh)

		client := agentv1.NewAgentServiceClient(cc)
		srv, srvErr := client.ExecuteTask(ctx)
		if srvErr != nil {
			return
		}
		defer srv.CloseSend() //nolint: errcheck// this is acceptable

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
		if srvErr = srv.Send(req); srvErr != nil {
			responseCh <- agentv1.NewExecuteTaskResponseErr(srvErr.Error())
			return
		}

		// Store the stream for potential feedback sending
		taskID := ""
		for {
			var update *agentv1.ExecuteTaskResponse
			update, srvErr = srv.Recv()
			if srvErr != nil {
				// Clean up stream on completion or error
				if taskID != "" {
					c.streamsMu.Lock()
					delete(c.streams, taskID)
					c.streamsMu.Unlock()
				}
				if errors.Is(srvErr, io.EOF) {
					return
				}
				responseCh <- agentv1.NewExecuteTaskResponseErr(srvErr.Error())
				return
			}

			// Capture task ID from acknowledgement for stream storage
			if ack := update.GetAck(); ack != nil {
				taskID = ack.GetTaskId()
				c.streamsMu.Lock()
				c.streams[taskID] = srv
				c.streamsMu.Unlock()
			}

			select {
			case responseCh <- update:
			case <-ctx.Done():
				// Clean up stream when context is cancelled
				if taskID != "" {
					c.streamsMu.Lock()
					delete(c.streams, taskID)
					c.streamsMu.Unlock()
				}
				responseCh <- agentv1.NewExecuteTaskResponseErr(ctx.Err().Error())
				return
			}
		}
	}()
	return responseCh, nil
}
