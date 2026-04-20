package chat

type SendMessageMsg struct {
	Content string
}

type ResponseMsg struct {
	Content string
	Error   error
}

type StreamChunkMsg struct {
	Content string
	Status  string
	IsFirst bool
}

type StreamCompleteMsg struct {
	Success bool
	Content string
	Error   error
}

type FeedbackRequestMsg struct {
	Question string
}
