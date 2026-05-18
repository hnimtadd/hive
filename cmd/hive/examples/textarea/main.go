package main

import (
	"log"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

type model struct {
	textarea textarea.Model
	err      error
}

func initialModel() model {
	ti := textarea.New()
	ti.Placeholder = "Write something..."
	ti.Focus()

	// Minimalist styling: no borders, just a prompt
	ti.Prompt = ""
	ti.ShowLineNumbers = false
	ti.SetWidth(30)
	ti.SetHeight(5)

	return model{
		textarea: ti,
		err:      nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	v := tea.NewView(lipgloss.NewStyle().Width(m.textarea.Width() + 2).Height(m.textarea.Height()).Background(tui.Background).AlignHorizontal(lipgloss.Center).Render(m.textarea.View()))
	v.AltScreen = true

	return v
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

