package mapper

import (
	"fmt"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/bee/system"
	"github.com/hnimtadd/hive/pkg/utils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ToTaskUpdateSuccess(msg *system.QueenOutput) *agentv1.ExecuteTaskResponse {
	update := &agentv1.ExecuteTaskResponse{}
	update.Payload = &agentv1.ExecuteTaskResponse_Success{
		Success: &agentv1.SuccessUpdate{
			Content: utils.SanitizeUTF8(msg.Content),
		},
	}
	update.At = timestamppb.Now()
	return update
}

func ToTaskUpdateFailed(msg *system.QueenOutput) *agentv1.ExecuteTaskResponse {
	update := &agentv1.ExecuteTaskResponse{}
	update.Payload = &agentv1.ExecuteTaskResponse_Error{
		Error: &agentv1.ErrorUpdate{
			Message: utils.SanitizeUTF8(msg.Content),
		},
	}
	update.At = timestamppb.Now()
	return update
}

func ToTaskUpdateInProgress(msg *system.QueenOutput) *agentv1.ExecuteTaskResponse {
	update := &agentv1.ExecuteTaskResponse{}
	content := utils.SanitizeUTF8(fmt.Sprintf("%s-next: %s", msg.Content, msg.NextAction))
	status := utils.SanitizeUTF8(string(msg.Status))
	update.Payload = &agentv1.ExecuteTaskResponse_Update{
		Update: &agentv1.InProgressUpdate{
			Content: content,
			Status:  status,
		},
	}
	update.At = timestamppb.Now()
	return update
}

func ToTaskUpdateRequireFeedback(msg *system.QueenOutput) *agentv1.ExecuteTaskResponse {
	update := &agentv1.ExecuteTaskResponse{}
	update.Payload = &agentv1.ExecuteTaskResponse_Feedback{
		Feedback: &agentv1.FeedbackRequire{
			Question: utils.SanitizeUTF8(msg.Content),
		},
	}
	update.At = timestamppb.Now()
	return update
}
