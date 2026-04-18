package top

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/chat"
	"github.com/hnimtadd/hive/internal/tui/footer"
	"github.com/hnimtadd/hive/internal/tui/keys"
	"github.com/hnimtadd/hive/pkg/config"
)

type model struct {
	cfg *config.Config

	mode          tui.Mode
	chat          *chat.Model
	footer        *footer.Model
	height, width int
}

func newModel(cfg *config.Config) (*model, error) {
	footer, err := footer.NewModel(footer.ModelOptions{
		Mode: tui.DefaultMode,
	})
	if err != nil {
		return nil, err
	}
	chat, err := chat.NewModel(chat.ModelOptions{})
	if err != nil {
		return nil, err
	}
	return &model{
		cfg:    cfg,
		mode:   tui.DefaultMode,
		footer: footer,
		chat:   chat,
	}, nil
}

// Init implements [tea.Model].
func (m *model) Init() tea.Cmd {
	return tui.NoopCmd
}

// Update implements [tea.Model].
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := []tea.Cmd{}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height, m.width = msg.Height, msg.Width
		m.chat.Update(tea.WindowSizeMsg{
			Height: m.contentHeight(),
			Width:  m.contentWidth(),
		})
		m.footer.Update(tea.WindowSizeMsg{
			Height: tui.FooterHeight,
			Width:  m.width,
		})
	case tui.ChangeModeMsg:
		if m.mode != tui.Mode(msg) {
			m.mode = tui.Mode(msg)
			cmd = append(cmd, m.footer.Update(msg))
		}
	case tea.KeyMsg:
		// Pressing any key makes any info/error message in the footer disappear.
		m.footer.ResetStatus()
		switch m.mode {
		case tui.ModeInsert:
			switch {
			case key.Matches(msg, keys.Insert.Leave):
				return m, tui.MsgCmd(tui.ChangeModeMsg(tui.DefaultMode))
			default:
				cmd = append(cmd, m.chat.Update(msg))
			}
		case tui.ModeNormal:
			switch {
			case key.Matches(msg, keys.Normal.Insert):
				return m, tui.MsgCmd(tui.ChangeModeMsg(tui.ModeInsert))
			case key.Matches(msg, keys.Normal.Quit):
				return m, tea.Quit
			}
		}
	}
	return m, tea.Batch(cmd...)
}

// View implements [tea.Model].
func (m *model) View() tea.View {
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		m.chat.View(),
		m.footer.View(),
	))
	v.AltScreen = true
	return v
}

// contentHeight returns the height available to the panes.
func (m model) contentHeight() int {
	vh := m.height - tui.FooterHeight
	return max(tui.MinContentHeight, vh)
}

// contentWidth return the width available to the panes.
func (m model) contentWidth() int {
	return max(tui.MinContentWidth, m.width)
}

func Start(cfg *config.Config) error {
	model, err := newModel(cfg)
	if err != nil {
		return fmt.Errorf("failed to create new model: %w", err)
	}
	p := tea.NewProgram(model)
	defer p.Kill()

	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("failed to run the program: %w", err)
	}
	return err
}
