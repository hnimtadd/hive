package tui

import tea "charm.land/bubbletea/v2"

type Model interface {
	Init() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	View() string
}
