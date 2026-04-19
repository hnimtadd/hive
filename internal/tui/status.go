package tui

type Status string

const (
	StatusConnecting Status = "connecting..."
	StatusReady      Status = "redy"
	StatusThinking   Status = "thinking..."
)
