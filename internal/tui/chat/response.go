package chat

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

type chatResponseModel struct {
	id       string
	status   string
	thinking string
	output   string
	error    string
	state    state

	width int
}
type state int

const (
	stateThinking state = iota
	stateSucceed
	stateError
)

func newChatResponseModel(id string, width int) *chatResponseModel {
	return &chatResponseModel{
		id:    id,
		width: width,
		state: stateThinking,
	}
}

// Init implements [tui.Model].
func (c *chatResponseModel) Init() tea.Cmd {
	return tui.NoopCmd
}

// Update implements [tui.Model].
func (c *chatResponseModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
	case StreamStartMsg:
		c.state = stateThinking
	case StreamChunkMsg:
		c.status = msg.Status
		c.thinking += msg.Content
	case StreamCompleteMsg:
		if msg.Success {
			c.state = stateSucceed
			c.output = msg.Content
		} else {
			c.state = stateError
			c.error = msg.Error.Error()
		}
	}
	return tui.NoopCmd
}

// View implements [tui.Model].
func (c *chatResponseModel) View() string {
	// Configure card style based on role
	var (
		headerTile string
		content    string
	)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Background(tui.InputBg)

	contentStyle := lipgloss.NewStyle().
		Width(c.width-2).
		Background(tui.InputBg).
		Foreground(tui.Foreground).
		Padding(0, 1)

	cardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(c.width)

	switch c.state {
	case stateThinking:
		headerTile = "thinking..."
		headerStyle = headerStyle.Foreground(tui.Accent)
		cardBorder = cardBorder.BorderForeground(tui.Accent)
		content = c.thinking
	case stateError:
		headerTile = "error"
		headerStyle = headerStyle.Foreground(tui.Red)
		cardBorder = cardBorder.BorderForeground(tui.Red)
		content = c.error
	case stateSucceed:
		headerTile = "output"
		headerStyle = headerStyle.Foreground(tui.Green)
		cardBorder = cardBorder.BorderForeground(tui.Green)
		content = c.output
	default:
		return ""
	}
	// Build the card
	header := headerStyle.Render(headerTile)
	body := contentStyle.Render(content)

	// Combine header and body, then wrap in border
	card := lipgloss.JoinVertical(lipgloss.Left, header, body)
	return cardBorder.Render(card)
}
