package session

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/hnimtadd/hive/internal/eventbus"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/internal/pipeline"
	"github.com/hnimtadd/hive/internal/storage"
	"github.com/hnimtadd/hive/pkg/types"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	inputCh  <-chan *agentv1.HiveSessionRequest
	outputCh chan<- *agentv1.HiveSessionResponse
	pipeline *pipeline.Pipeline
	eventbus *eventbus.EventBus[*agentv1.SessionEvent]

	storage storage.SessionStorage

	sessions map[string]*types.Session
}

func NewHandler(
	inputCh <-chan *agentv1.HiveSessionRequest,
	outputCh chan<- *agentv1.HiveSessionResponse,
	pipeline *pipeline.Pipeline,
	eventbus *eventbus.EventBus[*agentv1.SessionEvent],
) *Handler {
	return &Handler{
		inputCh:  inputCh,
		outputCh: outputCh,
		pipeline: pipeline,
		eventbus: eventbus,
	}
}

func (h *Handler) Start(ctx context.Context) error {
	logger := observability.Logger(ctx)
	for req := range h.inputCh {
		switch payload := req.Payload.(type) {
		case *agentv1.HiveSessionRequest_CreateConversation:
			err := h.handleCreateConversation(ctx, req, payload)
			if err != nil {
				logger.ErrorContext(ctx, "failed while handling create conversation", slog.Any("error", err))
			}

		case *agentv1.HiveSessionRequest_Input:
			err := h.handleRequestInput(ctx, req, payload)
			if err != nil {
				logger.ErrorContext(ctx, "failed while handling create input", slog.Any("error", err))
			}

		case *agentv1.HiveSessionRequest_TurnRequest:
			err := h.handleTurnRequest(ctx, req, payload)
			if err != nil {
				logger.ErrorContext(ctx, "failed while handling turn request", slog.Any("error", err))
			}

		case *agentv1.HiveSessionRequest_Ping:
			err := h.handlePing(ctx, req, payload)
			if err != nil {
				logger.ErrorContext(ctx, "failed while handling ping", slog.Any("error", err))
			}

		}
	}

	return nil
}

func (h *Handler) handleCreateConversation(ctx context.Context, req *agentv1.HiveSessionRequest, payload *agentv1.HiveSessionRequest_CreateConversation) error {
	var resp *agentv1.HiveSessionResponse
	switch mode := payload.CreateConversation.GetMode().(type) {
	case *agentv1.CreateConversationRequest_CreateNew:
		conversation := types.NewSession()
		createConvResp := &agentv1.HiveSessionResponse_CreateConversation{
			CreateConversation: &agentv1.CreateConversationResponse{
				ConversationId: conversation.ID,
				// TODO: change this
				Location: conversation.ID,
			},
		}
		if err := h.storage.Create(conversation); err != nil {
			return err
		}
		resp = &agentv1.HiveSessionResponse{
			Payload:   createConvResp,
			At:        timestamppb.New(time.Now()),
			InReplyTo: req.RequestId,
		}

	case *agentv1.CreateConversationRequest_ResumeId:
		fmt.Println(mode.ResumeId)
		session, err := h.storage.Load(mode.ResumeId)
		if err != nil {
			return err
		}
		resp = &agentv1.HiveSessionResponse{
			Payload: &agentv1.HiveSessionResponse_CreateConversation{
				CreateConversation: &agentv1.CreateConversationResponse{
					ConversationId: session.ID,
					// TODO: change this
					Location: session.ID,
				},
			},
			At:        timestamppb.New(time.Now()),
			InReplyTo: req.RequestId,
		}

	default:
		return fmt.Errorf("receive unexpected conversation request: %s", payload.CreateConversation.String())
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case h.outputCh <- resp:
		return nil
	}
}

func (h *Handler) handleRequestInput(ctx context.Context, _ *agentv1.HiveSessionRequest, payload *agentv1.HiveSessionRequest_Input) error {
	if err := h.pipeline.Handle(ctx, pipeline.PipelineCommand{
		Key: pipeline.PipelineSubmitInputKey,
		Payload: pipeline.PipelineSubmitInputPayload{
			CorrelationID: payload.Input.GetTurnId(),
			Input:         payload.Input.GetAnswer(),
		}}); err != nil {
		return err
	}

	return nil
}

func (h *Handler) handleTurnRequest(ctx context.Context, req *agentv1.HiveSessionRequest, payload *agentv1.HiveSessionRequest_TurnRequest) error {
	switch turnRequest := payload.TurnRequest.Payload.(type) {
	case *agentv1.TurnRequest_CreateTurn:
		// For create turn, we create start new pipeline
		session, ok := h.sessions[payload.TurnRequest.GetConversationId()]
		if !ok {
			return fmt.Errorf("conversation %s not found", payload.TurnRequest.GetConversationId())
		}
		session.Messages = append(session.Messages, types.NewMessage(types.RoleUser, turnRequest.CreateTurn.GetMessage()))
		maps.Copy(session.Artifacts, turnRequest.CreateTurn.InitialArtifacts)
		state := pipeline.NewPipelineState(ctx, session)
		resp := &agentv1.HiveSessionResponse{
			Payload: &agentv1.HiveSessionResponse_TurnResponse{
				TurnResponse: &agentv1.TurnResponse{
					TurnId:         payload.TurnRequest.GetTurnId(),
					ConversationId: payload.TurnRequest.GetConversationId(),
					Payload: &agentv1.TurnResponse_Ack{
						Ack: &emptypb.Empty{},
					},
				},
			},
			At:        timestamppb.Now(),
			InReplyTo: req.GetRequestId(),
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h.outputCh <- resp:
		}

		result, err := h.pipeline.Execute(ctx, state)
		if err != nil {
			return err
		}

		resp = &agentv1.HiveSessionResponse{
			Payload: &agentv1.HiveSessionResponse_TurnResponse{
				TurnResponse: &agentv1.TurnResponse{
					TurnId:         payload.TurnRequest.GetTurnId(),
					ConversationId: payload.TurnRequest.GetConversationId(),
					Payload: &agentv1.TurnResponse_Completed{
						Completed: &agentv1.TurnCompleted{
							Payload: &agentv1.TurnCompleted_Success{
								Success: &agentv1.SuccessUpdate{
									Content: result.Output,
								},
							},
						},
					},
				},
			},
			At:        timestamppb.Now(),
			InReplyTo: req.GetRequestId(),
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h.outputCh <- resp:
		}
	case *agentv1.TurnRequest_CancelTurnId:
	case *agentv1.TurnRequest_RetryTurnId:
	}
	return nil
}

// handlePing catchs the ping message at timestamp A and create the pong message at timestamp B.
// later on, TUI could get the ping delay will be duration between A and B.
func (h *Handler) handlePing(ctx context.Context, req *agentv1.HiveSessionRequest, payload *agentv1.HiveSessionRequest_Ping) error {
	resp := &agentv1.HiveSessionResponse{
		InReplyTo: req.GetRequestId(),
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionResponse_Pong{
			Pong: &agentv1.Pong{
				At: payload.Ping.GetAt(),
			},
		},
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case h.outputCh <- resp:
		return nil
	}
}
