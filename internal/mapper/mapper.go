package mapper

import (
	"fmt"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/agent"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ToTaskUpdateSuccess(msg *agent.SupervisorOutput) *agentv1.ServerMessage {
	update := &agentv1.ServerMessage{}
	update.Payload = &agentv1.ServerMessage_Success{
		Success: &agentv1.SuccessUpdate{
			Content: msg.Content,
		},
	}
	update.At = timestamppb.Now()
	return update
}

func ToTaskUpdateFailed(msg *agent.SupervisorOutput) *agentv1.ServerMessage {
	update := &agentv1.ServerMessage{}
	update.Payload = &agentv1.ServerMessage_Error{
		Error: &agentv1.ErrorUpdate{
			Message: msg.Content,
		},
	}
	update.At = timestamppb.Now()
	return update
}

func ToTaskUpdateInProgress(msg *agent.SupervisorOutput) *agentv1.ServerMessage {
	update := &agentv1.ServerMessage{}
	update.Payload = &agentv1.ServerMessage_Update{
		Update: &agentv1.InProgressUpdate{
			Content: fmt.Sprintf("%s-next: %s", msg.Content, msg.NextAction),
			Status:  string(msg.Status),
		},
	}
	update.At = timestamppb.Now()
	return update
}

func ToTaskUpdateRequireFeedback(msg *agent.SupervisorOutput) *agentv1.ServerMessage {
	update := &agentv1.ServerMessage{}
	update.Payload = &agentv1.ServerMessage_Feedback{
		Feedback: &agentv1.FeedbackRequire{
			Question: msg.Content,
		},
	}
	update.At = timestamppb.Now()
	return update

}
