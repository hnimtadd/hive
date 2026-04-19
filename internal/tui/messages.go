package tui

type (
	InfoMsg         string
	ErrorMsg        error
	ChangeModeMsg   Mode
	ChangeStatusMsg Status
)

type SendMessageMsg struct {
	Content string
}

type MessageReceivedMsg struct {
	Content string
	IsError bool
}

type ClearChatMsg struct{}

type ToggleHelpMsg struct{}
