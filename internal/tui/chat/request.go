package chat

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

type chatRequestModel struct {
	question string

	width int
}

func newChatRequestModel(question string, width int) *chatRequestModel {
	m := &chatRequestModel{
		question: question,
		width:    width,
	}
	return m
}

// Init implements [tui.Model].
func (m *chatRequestModel) Init() tea.Cmd {
	return tui.NoopCmd
}

// Update implements [tui.Model].
func (m *chatRequestModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	default:
	}
	return tui.NoopCmd
}

// View implements [tui.Model].
func (m *chatRequestModel) View() string {
	headerLabel := "Input"
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tui.Blue).
		Background(tui.InputBg).
		Padding(0, 1)
	contentStyle := lipgloss.NewStyle().
		Width(m.width-2).
		Background(tui.InputBg).
		Foreground(tui.Foreground).
		Padding(0, 1)
	cardBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.Blue).
		Width(m.width)

	// Format content
	content := m.question
	// Build the card
	header := headerStyle.Render(headerLabel)
	body := contentStyle.Render(content)

	// Combine header and body, then wrap in border
	card := lipgloss.JoinVertical(lipgloss.Left, header, body)
	return cardBorder.Render(card)
}
