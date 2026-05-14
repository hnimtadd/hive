package server_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/internal/bee"
	beeRegistry "github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/internal/transport/server"
	"github.com/hnimtadd/hive/pkg/config"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeProvider struct{}

func (fakeProvider) GetModel(_ context.Context, _ llm.Tier) (model.ToolCallingChatModel, bool) {
	return nil, false
}
func (fakeProvider) ModelPool(_ llm.Tier) func() model.ToolCallingChatModel {
	return func() model.ToolCallingChatModel { return nil }
}

type fakeRegistry struct{}

func (fakeRegistry) ListAgents() []bee.CustomBee[beeRegistry.WorkerInput, beeRegistry.WorkerOutput] {
	return nil
}

func (fakeRegistry) GetByID(string) (bee.CustomBee[beeRegistry.WorkerInput, beeRegistry.WorkerOutput], bool) {
	return nil, false
}

func TestHiveSessionCreateConversationAndPing(t *testing.T) {
	stream, cleanup := newHiveSessionStream(t)
	defer cleanup()

	if err := stream.Send(&agentv1.HiveSessionRequest{
		RequestId: "create-req",
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_CreateConversation{
			CreateConversation: &agentv1.CreateConversationRequest{
				Mode: &agentv1.CreateConversationRequest_CreateNew{
					CreateNew: &emptypb.Empty{},
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to send create_conversation: %v", err)
	}

	createResp, err := stream.Recv()
	if err != nil {
		t.Fatalf("failed to receive create_conversation response: %v", err)
	}
	if got := createResp.GetInReplyTo(); got != "create-req" {
		t.Fatalf("unexpected in_reply_to: got %q", got)
	}
	if createResp.GetCreateConversation() == nil {
		t.Fatalf("expected create_conversation response payload")
	}
	if got := createResp.GetCreateConversation().GetConversationId(); got == "" {
		t.Fatalf("expected non-empty conversation_id")
	}

	if err = stream.Send(&agentv1.HiveSessionRequest{
		RequestId: "ping-req",
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_Ping{
			Ping: &agentv1.Ping{
				At: timestamppb.Now(),
			},
		},
	}); err != nil {
		t.Fatalf("failed to send ping: %v", err)
	}

	pongResp, err := stream.Recv()
	if err != nil {
		t.Fatalf("failed to receive pong response: %v", err)
	}
	if got := pongResp.GetInReplyTo(); got != "ping-req" {
		t.Fatalf("unexpected pong in_reply_to: got %q", got)
	}
	if pongResp.GetPong() == nil {
		t.Fatalf("expected pong payload")
	}
}

func TestHiveSessionCreateTurnReturnsAck(t *testing.T) {
	stream, cleanup := newHiveSessionStream(t)
	defer cleanup()

	if err := stream.Send(&agentv1.HiveSessionRequest{
		RequestId: "create-req",
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_CreateConversation{
			CreateConversation: &agentv1.CreateConversationRequest{
				Mode: &agentv1.CreateConversationRequest_CreateNew{
					CreateNew: &emptypb.Empty{},
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to send create_conversation: %v", err)
	}

	createResp, err := stream.Recv()
	if err != nil {
		t.Fatalf("failed to receive create_conversation response: %v", err)
	}
	conversationID := createResp.GetCreateConversation().GetConversationId()
	if conversationID == "" {
		t.Fatalf("expected non-empty conversation_id")
	}

	if err = stream.Send(&agentv1.HiveSessionRequest{
		RequestId: "turn-req",
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionRequest_TurnRequest{
			TurnRequest: &agentv1.TurnRequest{
				ConversationId: conversationID,
				TurnId:         "turn-1",
				At:             timestamppb.Now(),
				Payload: &agentv1.TurnRequest_CreateTurn{
					CreateTurn: &agentv1.CreateTurn{
						Message: "hello",
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("failed to send turn request: %v", err)
	}

	ackResp, err := stream.Recv()
	if err != nil {
		t.Fatalf("failed to receive turn ack: %v", err)
	}
	turnResp := ackResp.GetTurnResponse()
	if turnResp == nil {
		t.Fatalf("expected turn_response payload")
	}
	if got := ackResp.GetInReplyTo(); got != "turn-req" {
		t.Fatalf("unexpected ack in_reply_to: got %q", got)
	}
	if got := turnResp.GetConversationId(); got != conversationID {
		t.Fatalf("unexpected conversation_id: got %q, want %q", got, conversationID)
	}
	if got := turnResp.GetTurnId(); got != "turn-1" {
		t.Fatalf("unexpected turn_id: got %q", got)
	}
	if turnResp.GetAck() == nil {
		t.Fatalf("expected ack payload in turn_response")
	}
}

func newHiveSessionStream(
	t *testing.T,
) (grpc.BidiStreamingClient[agentv1.HiveSessionRequest, agentv1.HiveSessionResponse], func()) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:                    "127.0.0.1",
			Port:                    0,
			MaxTimeout:              30 * time.Second,
			GracefulShutdownTimeout: 5 * time.Second,
		},
		AI: config.AIConfig{
			MaxStep: 1,
			Context: config.ContextConfig{
				MaxTokens:               4096,
				SummaryTriggerThreshold: 3000,
				SummaryTargetTokens:     500,
			},
		},
		Session: config.SessionConfig{
			Dir:     tmpDir,
			Enabled: false,
		},
	}

	sessionStore, err := storage.NewSessionStorage(storage.Options{Storage: tmpDir})
	if err != nil {
		t.Fatalf("failed to create session storage: %v", err)
	}

	hiveServer, err := server.NewHiveServer(cfg, fakeProvider{}, fakeRegistry{}, sessionStore)
	if err != nil {
		t.Fatalf("failed to create hive server: %v", err)
	}

	addr := reserveAddr(t)
	errCh := make(chan error, 1)
	go func() {
		errCh <- hiveServer.Serve(addr)
	}()
	waitForServer(t, addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		hiveServer.Stop()
		t.Fatalf("failed to dial hive server: %v", err)
	}
	client := agentv1.NewAgentServiceClient(conn)
	stream, err := client.HiveSession(context.Background())
	if err != nil {
		_ = conn.Close()
		hiveServer.Stop()
		t.Fatalf("failed to open hive session stream: %v", err)
	}

	cleanup := func() {
		_ = stream.CloseSend()
		_ = conn.Close()
		hiveServer.Stop()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	}

	return stream, cleanup
}

func reserveAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve free address: %v", err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	return addr
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s", addr)
}
