package tui

import (
	"charm.land/lipgloss/v2"
)

var (
	Regular = lipgloss.NewStyle()
	Bold    = Regular.Bold(true)
	Padded  = Regular.Padding(0, 1)

	Border           = Regular.Border(lipgloss.NormalBorder())
	ThickBorder      = Regular.Border(lipgloss.ThickBorder()).BorderForeground(Accent)
	DefaultContainer = Regular.Background(Background).Foreground(Foreground)
	Test             = Regular.Background(Blue).Foreground(Red)

	Error           = Regular.Background(Red).Foreground(White)
	AccentContainer = Padded.Background(Accent).Foreground(White)
)

var (
	Background = lipgloss.Color("#1E1E1E")
	Foreground = lipgloss.Color("#D7D7D7")
	Red        = lipgloss.Color("#E3242B")
	Green      = lipgloss.Color("#4CAF50")
	Yellow     = lipgloss.Color("#FFC107")
	Blue       = lipgloss.Color("#2196F3")
	Accent     = lipgloss.Color("#BB86FC")
	Secondary  = lipgloss.Color("#03DAC6")
	Muted      = lipgloss.Color("#6C757D")
	Highlight  = lipgloss.Color("#FF4081")
	White      = lipgloss.Color("#FFFFFF")
	InputBg    = lipgloss.Color("#2C2C2C")
)
