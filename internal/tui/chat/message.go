package chat

type SendMessageMsg struct {
	Content string
}

type ResponseMsg struct {
	Content string
	Error   error
}
