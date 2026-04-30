package chat

type SendMessageMsg struct {
	Content string
}

type StreamStartMsg struct {
	TaskID string
}

type StreamChunkMsg struct {
	TaskID  string
	Content string
	Status  string
}

type StreamCompleteMsg struct {
	TaskID  string
	Success bool
	Content string
	Error   error
}

type FeedbackRequestMsg struct {
	TaskID   string
	Question string
}

type FeedbackResponseMsg struct {
	TaskID   string
	Response string
}
