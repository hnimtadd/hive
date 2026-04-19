package header

import (
	tea "charm.land/bubbletea/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

type ModelOptions struct {
	Status tui.Status
}

type Model struct {
	status tui.Status

	width, height int
}

func NewModel(opts ModelOptions) (*Model, error) {
	model := &Model{
		status: opts.Status,
	}

	return model, nil
}

func (f *Model) Init() tea.Cmd {
	return nil
}

func (f *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	// Handle error signal related msg
	switch msg := msg.(type) {
	case tui.ChangeStatusMsg:
		// Blindly set here at top model should take
		// responsbility to check the global status
		f.status = tui.Status(msg)

	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height
	}
	return tea.Batch(cmds...)
}

func (f *Model) View() tea.View {
	status := tui.AccentContainer.Render(string(f.status))
	return tea.NewView(tui.Regular.
		Inline(true).
		MaxWidth(f.width).
		Height(f.height).
		Width(f.width).
		Render(status))
}
