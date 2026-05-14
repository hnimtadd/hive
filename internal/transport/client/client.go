package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/hnimtadd/hive/pkg/config"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	config *config.Config

	streamsMu sync.RWMutex
	streams   map[string]grpc.BidiStreamingClient[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse] // Active streams keyed by conversation ID
}

func NewClient(cfg *config.Config) (*Client, error) {
	return &Client{
		config:  cfg,
		streams: make(map[string]grpc.BidiStreamingClient[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse]),
	}, nil
}

// SendFeedback sends input for an input-required event.
func (c *Client) SendFeedback(ctx context.Context, conversationID, turnID, feedback string) error {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	if conversationID == "" {
		return fmt.Errorf("conversationID cannot be empty")
	}
	if turnID == "" {
		return fmt.Errorf("turnID cannot be empty")
	}
	if feedback == "" {
		return fmt.Errorf("feedback cannot be empty")
	}

	stream, exists := c.getStream(conversationID)

	if !exists {
		return fmt.Errorf("no active stream found for conversationID: %s", conversationID)
	}

	req := &agentv1.HiveSessionRequest{
		RequestId: uuid.NewString(),
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_Input{
			Input: &agentv1.SubmitRequiredInput{
				ConversationId: conversationID,
				TurnId:         turnID,
				Answer:         feedback,
			},
		},
	}

	// Send feedback through the stream
	if err := stream.Send(req); err != nil {
		c.dropStream(conversationID)
		return fmt.Errorf("failed to send feedback: %w", err)
	}

	return nil
}

// ExecuteTaskWithChannel sends a command and returns a channel for receiving responses.
// This is a TUI-friendly version that doesn't use stdin for feedback.
func (c *Client) ExecuteTaskWithChannel(ctx context.Context, command string) (<-chan *agentv1.HiveSessionResponse, error) {
	cc, err := grpc.NewClient(c.config.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create connection to server: %w", err)
	}
	responseCh := make(chan *agentv1.HiveSessionResponse, 100)

	go func() {
		defer cc.Close()
		defer close(responseCh)

		client := agentv1.NewAgentServiceClient(cc)
		srv, srvErr := client.HiveSession(ctx)
		if srvErr != nil {
			responseCh <- &agentv1.HiveSessionResponse{
				At: timestamppb.Now(),
				Payload: &agentv1.HiveSessionResponse_Notification{
					Notification: &agentv1.Notification{
						Payload: &agentv1.Notification_Error{Error: fmt.Sprintf("failed to open stream: %v", srvErr)},
					},
				},
			}
			return
		}
		defer srv.CloseSend() //nolint: errcheck// this is acceptable

		conversationReqID := uuid.NewString()
		if srvErr = srv.Send(&agentv1.HiveSessionRequest{
			RequestId: conversationReqID,
			At:        timestamppb.Now(),
			Payload: &agentv1.HiveSessionRequest_CreateConversation{
				CreateConversation: &agentv1.CreateConversationRequest{
					Mode: &agentv1.CreateConversationRequest_CreateNew{
						CreateNew: &emptypb.Empty{},
					},
				},
			},
		}); srvErr != nil {
			c.sendNotificationError(responseCh, fmt.Sprintf("failed to create conversation: %v", srvErr))
			return
		}

		conversationID := ""
		turnReqID := uuid.NewString()
		for {
			var update *agentv1.HiveSessionResponse
			update, srvErr = srv.Recv()
			if srvErr != nil {
				// Clean up stream on completion or error
				if conversationID != "" {
					c.dropStream(conversationID)
				}
				if errors.Is(srvErr, io.EOF) {
					return
				}
				c.sendNotificationError(responseCh, srvErr.Error())
				return
			}

			if createConversation := update.GetCreateConversation(); createConversation != nil {
				conversationID = createConversation.GetConversationId()
				c.setStream(conversationID, srv)
				if srvErr = srv.Send(&agentv1.HiveSessionRequest{
					RequestId: turnReqID,
					At:        timestamppb.Now(),
					Payload: &agentv1.HiveSessionRequest_TurnRequest{
						TurnRequest: &agentv1.TurnRequest{
							ConversationId: conversationID,
							At:             timestamppb.Now(),
							Payload: &agentv1.TurnRequest_CreateTurn{
								CreateTurn: &agentv1.CreateTurn{
									Message:          command,
									InitialArtifacts: map[string]string{},
								},
							},
						},
					},
				}); srvErr != nil {
					c.sendNotificationError(responseCh, fmt.Sprintf("failed to create turn: %v", srvErr))
					return
				}
			}

			select {
			case responseCh <- update:
			case <-ctx.Done():
				// Clean up stream when context is cancelled
				if conversationID != "" {
					c.dropStream(conversationID)
				}
				c.sendNotificationError(responseCh, ctx.Err().Error())
				return
			}
		}
	}()
	return responseCh, nil
}

func (c *Client) getStream(conversationID string) (grpc.BidiStreamingClient[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse], bool) {
	c.streamsMu.RLock()
	defer c.streamsMu.RUnlock()

	if conversationID != "" {
		stream, ok := c.streams[conversationID]
		return stream, ok
	}

	if len(c.streams) == 1 {
		for _, stream := range c.streams {
			return stream, true
		}
	}
	return nil, false
}

func (c *Client) setStream(conversationID string, stream grpc.BidiStreamingClient[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse]) {
	if conversationID == "" || stream == nil {
		return
	}
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()
	c.streams[conversationID] = stream
}

func (c *Client) dropStream(conversationID string) {
	if conversationID == "" {
		return
	}
	c.streamsMu.Lock()
	defer c.streamsMu.Unlock()
	delete(c.streams, conversationID)
}

func (c *Client) sendNotificationError(ch chan<- *agentv1.HiveSessionResponse, errMsg string) {
	ch <- &agentv1.HiveSessionResponse{
		At: timestamppb.Now(),
		Payload: &agentv1.HiveSessionResponse_Notification{
			Notification: &agentv1.Notification{
				Payload: &agentv1.Notification_Error{Error: errMsg},
			},
		},
	}
}
