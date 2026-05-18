package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/google/uuid"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
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
	storage   storage.SessionStorage
}

type InlineRoundResult struct {
	ConversationID string
	TurnID         string
	Status         string
	Content        string
	Question       string
	Updates        []string
}

func NewClient(cfg *config.Config) (*Client, error) {
	storage, err := storage.NewSessionStorage(storage.Options{
		Storage: cfg.Storage.Dir,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		config:  cfg,
		storage: storage,
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

// StartConversation opens a persistent HiveSession stream and initializes a conversation.
// The returned response channel carries all subsequent server updates for the stream.
func (c *Client) StartConversation(ctx context.Context, resumeID string) (string, <-chan *agentv1.HiveSessionResponse, error) {
	cc, err := grpc.NewClient(c.config.Server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create connection to server: %w", err)
	}

	client := agentv1.NewAgentServiceClient(cc)
	srv, err := client.HiveSession(ctx)
	if err != nil {
		_ = cc.Close()
		return "", nil, fmt.Errorf("failed to open stream: %w", err)
	}

	createReq := &agentv1.HiveSessionRequest{
		RequestId: uuid.NewString(),
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_CreateConversation{
			CreateConversation: &agentv1.CreateConversationRequest{
				Mode: &agentv1.CreateConversationRequest_CreateNew{
					CreateNew: &emptypb.Empty{},
				},
			},
		},
	}
	if resumeID != "" {
		createReq.Payload = &agentv1.HiveSessionRequest_CreateConversation{
			CreateConversation: &agentv1.CreateConversationRequest{
				Mode: &agentv1.CreateConversationRequest_ResumeId{
					ResumeId: resumeID,
				},
			},
		}
	}

	if err = srv.Send(createReq); err != nil {
		_ = srv.CloseSend()
		_ = cc.Close()
		return "", nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	firstResp, err := srv.Recv()
	if err != nil {
		_ = srv.CloseSend()
		_ = cc.Close()
		return "", nil, fmt.Errorf("failed to receive create conversation response: %w", err)
	}
	if n := firstResp.GetNotification(); n != nil && n.GetError() != "" {
		_ = srv.CloseSend()
		_ = cc.Close()
		return "", nil, errors.New(n.GetError())
	}

	conv := firstResp.GetCreateConversation()
	if conv == nil || conv.GetConversationId() == "" {
		_ = srv.CloseSend()
		_ = cc.Close()
		return "", nil, fmt.Errorf("expected create conversation response, got %T", firstResp.GetPayload())
	}
	conversationID := conv.GetConversationId()
	c.setStream(conversationID, srv)

	responseCh := make(chan *agentv1.HiveSessionResponse, 100)
	responseCh <- firstResp

	go func() {
		defer close(responseCh)
		defer c.dropStream(conversationID)
		defer srv.CloseSend() //nolint: errcheck // best-effort stream close
		defer cc.Close()      //nolint: errcheck // best-effort connection close

		for {
			update, recvErr := srv.Recv()
			if recvErr != nil {
				if !errors.Is(recvErr, io.EOF) && ctx.Err() == nil {
					c.sendNotificationError(responseCh, recvErr.Error())
				}
				return
			}

			select {
			case responseCh <- update:
			case <-ctx.Done():
				return
			}
		}
	}()

	return conversationID, responseCh, nil
}

// SendTurn sends a create_turn request over an existing conversation stream.
func (c *Client) SendTurn(ctx context.Context, conversationID, command string) (turnID string, requestID string, err error) {
	if conversationID == "" {
		return "", "", fmt.Errorf("conversationID cannot be empty")
	}
	if command == "" {
		return "", "", fmt.Errorf("command cannot be empty")
	}

	select {
	case <-ctx.Done():
		return "", "", fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	stream, exists := c.getStream(conversationID)
	if !exists {
		return "", "", fmt.Errorf("no active stream found for conversationID: %s", conversationID)
	}

	turnID = uuid.NewString()
	requestID = uuid.NewString()
	if err = stream.Send(&agentv1.HiveSessionRequest{
		RequestId: requestID,
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_TurnRequest{
			TurnRequest: &agentv1.TurnRequest{
				ConversationId: conversationID,
				TurnId:         turnID,
				At:             timestamppb.Now(),
				Payload: &agentv1.TurnRequest_CreateTurn{
					CreateTurn: &agentv1.CreateTurn{
						Message:          command,
						InitialArtifacts: map[string]string{},
					},
				},
			},
		},
	}); err != nil {
		c.dropStream(conversationID)
		return "", "", fmt.Errorf("failed to create turn: %w", err)
	}

	return turnID, requestID, nil
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

// ExecuteTaskInline runs one request/response round and returns the terminal outcome.
// It consumes a single turn flow and returns when it reaches one of:
// - turn completed (success/failed)
// - input required
// - notification error
func (c *Client) ExecuteTaskInline(ctx context.Context, command string) (*InlineRoundResult, error) {
	responseCh, err := c.ExecuteTaskWithChannel(ctx, command)
	if err != nil {
		return nil, err
	}

	result := &InlineRoundResult{
		Status:  "in_progress",
		Updates: []string{},
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp, ok := <-responseCh:
			if !ok {
				if result.Content != "" || result.Question != "" || len(result.Updates) > 0 {
					return result, nil
				}
				return nil, fmt.Errorf("inline round terminated without response")
			}

			if conv := resp.GetCreateConversation(); conv != nil {
				result.ConversationID = conv.GetConversationId()
				continue
			}

			if n := resp.GetNotification(); n != nil {
				if errMsg := n.GetError(); errMsg != "" {
					return nil, errors.New(errMsg)
				}
				if info := n.GetInfo(); info != "" {
					result.Updates = append(result.Updates, info)
				}
				continue
			}

			if in := resp.GetInputRequired(); in != nil {
				result.Status = "input_required"
				result.ConversationID = in.GetConversationId()
				result.TurnID = in.GetTurnId()
				result.Question = in.GetQuestion()
				return result, nil
			}

			if turn := resp.GetTurnResponse(); turn != nil {
				if turn.GetConversationId() != "" {
					result.ConversationID = turn.GetConversationId()
				}
				if turn.GetTurnId() != "" {
					result.TurnID = turn.GetTurnId()
				}
				if update := turn.GetUpdate(); update != nil {
					result.Updates = append(result.Updates, update.GetContent())
					continue
				}
				if completed := turn.GetCompleted(); completed != nil {
					if success := completed.GetSuccess(); success != nil {
						result.Status = "completed"
						result.Content = success.GetContent()
						return result, nil
					}
					if failed := completed.GetFailed(); failed != nil {
						result.Status = "failed"
						result.Content = failed.GetMessage()
						return result, nil
					}
				}
			}
		}
	}
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

func (c *Client) ListSessions() ([]*types.Session, error) {
	return c.storage.List()
}

func (c *Client) GetSession(sessionID string) (*types.Session, error) {
	return c.storage.Load(sessionID)
}
