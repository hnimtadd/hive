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
			Ack: &RequestAck{
				TaskId: taskID,
			},
		},
		At: timestamppb.Now(),
	}
}

func NewTurnCompletedSuccess(content string) *TurnCompleted {
	return &TurnCompleted{
		Payload: &TurnCompleted_Success{
			Success: &SuccessUpdate{
				Content: content,
			},
		},
	}
}

func NewTurnCompletedFailed(err string) *TurnCompleted {
	return &TurnCompleted{
		Payload: &TurnCompleted_Failed{
			Failed: &ErrorUpdate{
				Message: err,
			},
		},
	}
}

func NewTurnUpdate(content string) *TurnUpdate {
	return &TurnUpdate{
		Content: content,
	}
}

func NewInputRequired(conversationID, turnID, question string) *InputRequired {
	return &InputRequired{
		ConversationId: conversationID,
		TurnId:         turnID,
		Question:       question,
	}
}

func NewTurnResponseSuccess(conversationID, turnID, requestID, content string) *TurnResponse {
	return &TurnResponse{
		ConversationId: conversationID,
		TurnId:         turnID,
		RequestId:      requestID,
		Payload: &TurnResponse_Completed{
			Completed: NewTurnCompletedSuccess(content),
		},
	}
}

func NewTurnResponseFailed(conversationID, turnID, requestID, err string) *TurnResponse {
	return &TurnResponse{
		ConversationId: conversationID,
		TurnId:         turnID,
		RequestId:      requestID,
		Payload: &TurnResponse_Completed{
			Completed: NewTurnCompletedFailed(err),
		},
	}
}

func NewTurnResponseUpdate(conversationID, turnID, requestID, content string) *TurnResponse {
	return &TurnResponse{
		ConversationId: conversationID,
		TurnId:         turnID,
		RequestId:      requestID,
		Payload: &TurnResponse_Update{
			Update: NewTurnUpdate(content),
		},
	}
}
