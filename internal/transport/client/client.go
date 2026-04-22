package client

import (
	"context"
	"errors"
	"fmt"
	"io"

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

	channels *channel.Manager
}

func NewClient(cfg *config.Config) (*Client, error) {
	// Create a new Hive task

	return &Client{config: cfg, channels: channel.NewManager()}, nil
}

// ExecuteTaskWithChannel sends a command and returns a channel for receiving responses.
// This is a TUI-friendly version that doesn't use stdin for feedback.
func (c *Client) ExecuteTaskWithChannel(ctx context.Context, command string) (<-chan *agentv1.ExecuteTaskResponse, error) {
	cc, err := grpc.NewClient(c.config.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create connection to server: %w", err)
	}
	client := agentv1.NewAgentServiceClient(cc)
	srv, err := client.ExecuteTask(ctx)
	if err != nil {
		cc.Close()      //nolint: gosec// this is acceptable
		srv.CloseSend() //nolint: errcheck,gosec// this is acceptable
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}

	responseCh := make(chan *agentv1.ExecuteTaskResponse, 100)

	go func() {
		defer srv.CloseSend() //nolint: errcheck// this is acceptable
		defer cc.Close()
		defer close(responseCh)

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
		if err = srv.Send(req); err != nil {
			responseCh <- agentv1.NewExecuteTaskResponseErr(err.Error())
			return
		}

		for {
			var update *agentv1.ExecuteTaskResponse
			update, err = srv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				responseCh <- agentv1.NewExecuteTaskResponseErr(err.Error())
				return
			}
			select {
			case responseCh <- update:
			case <-ctx.Done():
				responseCh <- agentv1.NewExecuteTaskResponseErr(ctx.Err().Error())
				return
			}
		}
	}()
	return responseCh, nil
}
