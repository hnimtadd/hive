package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
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

	session *types.Session

	unsubscribe func()
	runMu       sync.Mutex
	activeRun   *turnRun
}

func NewHandler(
	inputCh <-chan *agentv1.HiveSessionRequest,
	outputCh chan<- *agentv1.HiveSessionResponse,
	pipeline *pipeline.Pipeline,
	eventbus *eventbus.EventBus[*agentv1.SessionEvent],
	storage storage.SessionStorage,
) *Handler {
	return &Handler{
		inputCh:  inputCh,
		outputCh: outputCh,
		pipeline: pipeline,
		eventbus: eventbus,
		storage:  storage,
	}
}

func (h *Handler) Start(ctx context.Context) error {
	logger := observability.Logger(ctx)
	defer h.cleanup()

	for req := range h.inputCh {
		switch payload := req.Payload.(type) {
		case *agentv1.HiveSessionRequest_CreateConversation:
			if err := h.handleCreateConversation(ctx, req, payload); err != nil {
				logger.ErrorContext(ctx, "failed while handling create conversation", slog.Any("error", err))
				_ = h.sendError(ctx, req.GetRequestId(), err.Error())
			}

		case *agentv1.HiveSessionRequest_Input:
			if err := h.handleRequestInput(ctx, req, payload); err != nil {
				logger.ErrorContext(ctx, "failed while handling create input", slog.Any("error", err))
				_ = h.sendError(ctx, req.GetRequestId(), err.Error())
			}

		case *agentv1.HiveSessionRequest_TurnRequest:
			if err := h.handleTurnRequest(ctx, req, payload); err != nil {
				logger.ErrorContext(ctx, "failed while handling turn request", slog.Any("error", err))
				_ = h.sendError(ctx, req.GetRequestId(), err.Error())
			}

		case *agentv1.HiveSessionRequest_Ping:
			if err := h.handlePing(ctx, req, payload); err != nil {
				logger.ErrorContext(ctx, "failed while handling ping", slog.Any("error", err))
				_ = h.sendError(ctx, req.GetRequestId(), err.Error())
			}

		default:
			_ = h.sendError(ctx, req.GetRequestId(), fmt.Sprintf("unsupported request payload type %T", payload))
		}
	}

	return nil
}

func (h *Handler) handleCreateConversation(ctx context.Context, req *agentv1.HiveSessionRequest, payload *agentv1.HiveSessionRequest_CreateConversation) error {
	if h.storage == nil {
		return fmt.Errorf("session storage is required")
	}
	if h.session != nil {
		return h.sendError(ctx, req.GetRequestId(), "conversation already initialized for this stream")
	}

	var resp *agentv1.HiveSessionResponse
	switch mode := payload.CreateConversation.GetMode().(type) {
	case *agentv1.CreateConversationRequest_CreateNew:
		conversation := types.NewSession()
		conversation.ConversationID = conversation.ID
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
		h.session = conversation
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
		if session.ConversationID == "" {
			session.ConversationID = session.ID
		}
		h.session = session
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

	eventCh, cancel := h.eventbus.SubscribeWithCancel(h.session.ID)
	h.unsubscribe = cancel
	go h.forwardEvents(ctx, eventCh)

	return h.sendResponse(ctx, resp)
}

func (h *Handler) forwardEvents(ctx context.Context, eventCh <-chan *agentv1.SessionEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if event == nil || event.Payload == nil {
				// Ignore internal/non-user-facing events that don't carry a streamable payload.
				continue
			}
			resp, err := event.ToHiveSessionResponse()
			if err != nil {
				// Keep stream alive on malformed/internal events.
				continue
			}
			if err = h.sendResponse(ctx, resp); err != nil {
				return
			}
		}
	}
}

func (h *Handler) handleRequestInput(ctx context.Context, _ *agentv1.HiveSessionRequest, payload *agentv1.HiveSessionRequest_Input) error {
	if h.session == nil {
		return fmt.Errorf("conversation not initialized")
	}
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
	if h.session == nil {
		return fmt.Errorf("conversation not initialized")
	}

	switch turnRequest := payload.TurnRequest.Payload.(type) {
	case *agentv1.TurnRequest_CreateTurn:
		if convID := payload.TurnRequest.GetConversationId(); convID != "" && convID != h.session.ID {
			return fmt.Errorf("conversation mismatch: expected %s got %s", h.session.ID, convID)
		}
		h.stopActiveRun()
		h.session.Messages = append(h.session.Messages, types.NewMessage(types.RoleUser, turnRequest.CreateTurn.GetMessage()))
		for key, value := range turnRequest.CreateTurn.InitialArtifacts {
			h.session.Artifacts[key] = value
		}
		if err := h.storage.Save(h.session); err != nil {
			return err
		}

		turnID := payload.TurnRequest.GetTurnId()
		if turnID == "" {
			turnID = uuid.NewString()
		}

		resp := &agentv1.HiveSessionResponse{
			Payload: &agentv1.HiveSessionResponse_TurnResponse{
				TurnResponse: &agentv1.TurnResponse{
					TurnId:         turnID,
					ConversationId: h.session.ID,
					RequestId:      req.GetRequestId(),
					Payload: &agentv1.TurnResponse_Ack{
						Ack: &emptypb.Empty{},
					},
				},
			},
			At:        timestamppb.Now(),
			InReplyTo: req.GetRequestId(),
		}
		if err := h.sendResponse(ctx, resp); err != nil {
			return err
		}

		runCtx, cancel := context.WithCancel(ctx)
		h.runMu.Lock()
		h.activeRun = &turnRun{
			id:     turnID,
			cancel: cancel,
		}
		h.runMu.Unlock()

		go func(turnRunID string) {
			state := pipeline.NewPipelineState(runCtx, h.session)
			state.RunID = turnRunID
			if _, err := h.pipeline.Execute(runCtx, state); err != nil {
				_ = h.sendError(runCtx, req.GetRequestId(), fmt.Sprintf("pipeline execution failed: %v", err))
			}
			_ = h.storage.Save(h.session)

			h.runMu.Lock()
			if h.activeRun != nil && h.activeRun.id == turnRunID {
				h.activeRun = nil
			}
			h.runMu.Unlock()
		}(turnID)

	case *agentv1.TurnRequest_CancelTurnId:
		h.runMu.Lock()
		if h.activeRun != nil && (turnRequest.CancelTurnId == "" || turnRequest.CancelTurnId == h.activeRun.id) {
			h.activeRun.cancel()
			h.activeRun = nil
		}
		h.runMu.Unlock()

	case *agentv1.TurnRequest_RetryTurnId:
		if err := h.sendInfo(ctx, req.GetRequestId(), "retry_turn_id is not implemented yet"); err != nil {
			return err
		}
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
	return h.sendResponse(ctx, resp)
}

func (h *Handler) sendResponse(ctx context.Context, resp *agentv1.HiveSessionResponse) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case h.outputCh <- resp:
		return nil
	}
}

func (h *Handler) sendError(ctx context.Context, inReplyTo, msg string) error {
	return h.sendResponse(ctx, &agentv1.HiveSessionResponse{
		InReplyTo: inReplyTo,
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionResponse_Notification{
			Notification: &agentv1.Notification{
				Payload: &agentv1.Notification_Error{Error: msg},
			},
		},
	})
}

func (h *Handler) sendInfo(ctx context.Context, inReplyTo, msg string) error {
	return h.sendResponse(ctx, &agentv1.HiveSessionResponse{
		InReplyTo: inReplyTo,
		At:        timestamppb.Now(),
		Payload: &agentv1.HiveSessionResponse_Notification{
			Notification: &agentv1.Notification{
				Payload: &agentv1.Notification_Info{Info: msg},
			},
		},
	})
}

func (h *Handler) stopActiveRun() {
	h.runMu.Lock()
	defer h.runMu.Unlock()
	if h.activeRun != nil {
		h.activeRun.cancel()
		h.activeRun = nil
	}
}

func (h *Handler) cleanup() {
	h.stopActiveRun()
	if h.unsubscribe != nil {
		h.unsubscribe()
		h.unsubscribe = nil
	}
}

type turnRun struct {
	id     string
	cancel context.CancelFunc
}
