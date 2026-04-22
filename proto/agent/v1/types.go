package agentv1

import (
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewExecuteTaskResponseErr(err string) *ExecuteTaskResponse {
	return &ExecuteTaskResponse{
		Payload: &ExecuteTaskResponse_Error{
			Error: &ErrorUpdate{
				Message: err,
			},
		},
		At: timestamppb.Now(),
	}
}

func NewExecuteTaskResponseUpdate(status string, content string) *ExecuteTaskResponse {
	return &ExecuteTaskResponse{
		Payload: &ExecuteTaskResponse_Update{
			Update: &InProgressUpdate{
				Status:  status,
				Content: content,
			},
		},
		At: timestamppb.Now(),
	}
}

func NewExecuteTaskResponseFeedback(question string) *ExecuteTaskResponse {
	return &ExecuteTaskResponse{
		Payload: &ExecuteTaskResponse_Feedback{
			Feedback: &FeedbackRequire{
				Question: question,
			},
		},
		At: timestamppb.Now(),
	}
}

func NewExecuteTaskResponseSuccess(output string) *ExecuteTaskResponse {
	return &ExecuteTaskResponse{
		Payload: &ExecuteTaskResponse_Success{
			Success: &SuccessUpdate{
				Content: output,
			},
		},
		At: timestamppb.Now(),
	}
}

func NewExecuteTaskResponseACK(taskID string) *ExecuteTaskResponse {
	return &ExecuteTaskResponse{
		Payload: &ExecuteTaskResponse_Ack{
			Ack: &AckRequest{
				TaskId: taskID,
			},
		},
		At: timestamppb.Now(),
	}
}
