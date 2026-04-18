package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/hnimtadd/hive/internal/tui/state"
)

type Renderer struct {
	state state.State
}

func New(state state.State) *Renderer {
	return &Renderer{
		state: state,
	}
}

// Init implements [tea.Model].
func (r *Renderer) Init() tea.Cmd {
	return NoopCmd
}

// Update implements [tea.Model].
func (r *Renderer) Update(tea.Msg) (tea.Model, tea.Cmd) {

	panic("unimplemented")
}

// View implements [tea.Model].
func (r *Renderer) View() tea.View {
	panic("unimplemented")
}
