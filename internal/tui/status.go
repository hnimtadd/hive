package tui

type Status string

const (
	StatusConnecting Status = "connecting..."
	StatusReady      Status = "ready"
	StatusThinking   Status = "thinking..."
)
