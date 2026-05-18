package agentv1

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// SessionEventType represents an event kind emitted by the session pipeline.
type SessionEventType string

const (
	SessionEventTypeCreateConversation SessionEventType = "create_conversation"
	SessionEventTypeTurnResponse       SessionEventType = "turn_response"
	SessionEventTypeInputRequired      SessionEventType = "input_required"
	SessionEventTypeNotification       SessionEventType = "notification"
	SessionEventTypePong               SessionEventType = "pong"
)

// SessionEvent is a transport-agnostic envelope that can be carried via event bus.
// It can later be converted into the protobuf stream response type.
type SessionEvent struct {
	Type      SessionEventType
	InReplyTo string
	Payload   SessionEventPayload
}

type SessionEventPayload interface {
	isSessionEventPayload()
}

type SessionEventCreateConversationPayload struct {
	Response *CreateConversationResponse
}

func (*SessionEventCreateConversationPayload) isSessionEventPayload() {}

type SessionEventTurnResponsePayload struct {
	Response *TurnResponse
}

func (*SessionEventTurnResponsePayload) isSessionEventPayload() {}

type SessionEventInputRequiredPayload struct {
	Input *InputRequired
}

func (*SessionEventInputRequiredPayload) isSessionEventPayload() {}

type SessionEventNotificationPayload struct {
	Notification *Notification
}

func (*SessionEventNotificationPayload) isSessionEventPayload() {}

type SessionEventPongPayload struct {
	Pong *Pong
}

func (*SessionEventPongPayload) isSessionEventPayload() {}

func NewSessionEventCreateConversation(inReplyTo string, resp *CreateConversationResponse) *SessionEvent {
	return &SessionEvent{
		Type:      SessionEventTypeCreateConversation,
		InReplyTo: inReplyTo,
		Payload: &SessionEventCreateConversationPayload{
			Response: resp,
		},
	}
}

func NewSessionEventTurnResponse(inReplyTo string, resp *TurnResponse) *SessionEvent {
	return &SessionEvent{
		Type:      SessionEventTypeTurnResponse,
		InReplyTo: inReplyTo,
		Payload: &SessionEventTurnResponsePayload{
			Response: resp,
		},
	}
}

func NewSessionEventInputRequired(inReplyTo string, input *InputRequired) *SessionEvent {
	return &SessionEvent{
		Type:      SessionEventTypeInputRequired,
		InReplyTo: inReplyTo,
		Payload: &SessionEventInputRequiredPayload{
			Input: input,
		},
	}
}

func NewSessionEventNotification(inReplyTo string, notification *Notification) *SessionEvent {
	return &SessionEvent{
		Type:      SessionEventTypeNotification,
		InReplyTo: inReplyTo,
		Payload: &SessionEventNotificationPayload{
			Notification: notification,
		},
	}
}

func NewSessionEventPong(inReplyTo string, pong *Pong) *SessionEvent {
	return &SessionEvent{
		Type:      SessionEventTypePong,
		InReplyTo: inReplyTo,
		Payload: &SessionEventPongPayload{
			Pong: pong,
		},
	}
}

func (e *SessionEvent) ToHiveSessionResponse() (*HiveSessionResponse, error) {
	if e == nil {
		return nil, fmt.Errorf("session event is nil")
	}
	if e.Payload == nil {
		return nil, fmt.Errorf("session event payload is nil")
	}

	resp := &HiveSessionResponse{
		InReplyTo: e.InReplyTo,
		At:        timestamppb.Now(),
	}

	switch payload := e.Payload.(type) {
	case *SessionEventCreateConversationPayload:
		resp.Payload = &HiveSessionResponse_CreateConversation{
			CreateConversation: payload.Response,
		}
	case *SessionEventTurnResponsePayload:
		resp.Payload = &HiveSessionResponse_TurnResponse{
			TurnResponse: payload.Response,
		}
	case *SessionEventInputRequiredPayload:
		resp.Payload = &HiveSessionResponse_InputRequired{
			InputRequired: payload.Input,
		}
	case *SessionEventNotificationPayload:
		resp.Payload = &HiveSessionResponse_Notification{
			Notification: payload.Notification,
		}
	case *SessionEventPongPayload:
		resp.Payload = &HiveSessionResponse_Pong{
			Pong: payload.Pong,
		}
	default:
		return nil, fmt.Errorf("unsupported session event payload type %T", e.Payload)
	}

	return resp, nil
}
