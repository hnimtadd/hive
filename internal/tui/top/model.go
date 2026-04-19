package top

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/chat"
	"github.com/hnimtadd/hive/internal/tui/footer"
	"github.com/hnimtadd/hive/internal/tui/header"
	"github.com/hnimtadd/hive/internal/tui/keys"
	"github.com/hnimtadd/hive/pkg/config"
)

type model struct {
	cfg *config.Config

	mode   tui.Mode
	status tui.Status

	header        *header.Model
	chat          *chat.Model
	footer        *footer.Model
	height, width int
}

func newModel(cfg *config.Config) (*model, error) {
	header, err := header.NewModel(header.ModelOptions{
		Status: tui.StatusConnecting,
	})
	if err != nil {
		return nil, err
	}
	footer, err := footer.NewModel(footer.ModelOptions{
		Mode: tui.DefaultMode,
	})
	if err != nil {
		return nil, err
	}
	chat, err := chat.NewModel(chat.ModelOptions{
		OnSendMessage: func(_ string) {
		},
	})
	if err != nil {
		return nil, err
	}
	return &model{
		cfg:    cfg,
		mode:   tui.DefaultMode,
		header: header,
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
		m.header.Update(tea.WindowSizeMsg{
			Height: tui.HeaderHeight,
			Width:  m.width,
		})
		m.footer.Update(tea.WindowSizeMsg{
			Height: tui.FooterHeight,
			Width:  m.width,
		})

		m.chat.Update(tea.WindowSizeMsg{
			Height: msg.Height - tui.FooterHeight - tui.HeaderHeight - 2,
			Width:  m.width - 2,
		})

	case tui.ChangeModeMsg:
		if m.mode != tui.Mode(msg) {
			m.mode = tui.Mode(msg)
			switch m.mode {
			case tui.ModeNormal:
				m.chat.Update(tea.BlurMsg{})
			case tui.ModeInsert:
				m.chat.Update(tea.FocusMsg{})
			}
			m.footer.Update(msg)
		}
	case tui.ChangeStatusMsg:
		if m.status != tui.Status(msg) {
			m.status = tui.Status(msg)
			m.header.Update(msg)
		}

	case tui.ClearChatMsg:
		m.chat.ClearMessages()

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
			case key.Matches(msg, keys.Normal.Clear):
				return m, tui.MsgCmd(tui.ClearChatMsg{})
			}
		}
	default:
		cmd = append(cmd, m.chat.Update(msg))
	}
	return m, tea.Batch(cmd...)
}

// View implements [tea.Model].
func (m *model) View() tea.View {
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Top,
		m.header.View().Content,
		tui.DefaultContainer.Width(m.width).PaddingBottom(2).AlignHorizontal(lipgloss.Center).Render(m.chat.View()),
		m.footer.View().Content,
	))

	v.AltScreen = true
	return v
}
