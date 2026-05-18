package agentv1

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
