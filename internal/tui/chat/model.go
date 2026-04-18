package chat

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/pkg/types"
)

type ModelOptions struct {
}

type Model struct {
	width, height int

	msgs []types.Message

	textarea textarea.Model
	viewport viewport.Model
}

func NewModel(_ ModelOptions) (*Model, error) {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.SetVirtualCursor(false)
	ta.Prompt = "┃ "
	ta.CharLimit = 280

	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	vp := viewport.New(viewport.WithWidth(30), viewport.WithHeight(5))
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	ta.KeyMap.InsertNewline.SetEnabled(false)

	model := &Model{
		textarea: ta,
		msgs:     []types.Message{},
		viewport: vp,
	}

	return model, nil
}

func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	// Handle error signal related msg
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.SetWidth(m.width)
		m.textarea.SetWidth(m.width)
		m.viewport.SetHeight(m.height - m.textarea.Height())

		if len(m.msgs) > 0 {
			m.viewport.SetContent(m.ContentString())
		}
		m.viewport.GotoBottom()

	case tea.FocusMsg:
		m.textarea.Focus()

	case tea.BlurMsg:
		m.textarea.Blur()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.msgs = append(m.msgs, types.NewMessage(types.RoleUser, m.textarea.Value()))
			m.viewport.SetContent(m.ContentString())
			m.textarea.Reset()
			m.viewport.GotoBottom()
			// submit user feedback/message
		default:
			// Send all other keypresses to the textarea.
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}
	case cursor.BlinkMsg:
		// Textarea should also process cursor blinks.
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *Model) View() tea.View {
	viewportView := m.viewport.View()
	v := tea.NewView(viewportView + "\n" + m.textarea.View())
	c := m.textarea.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(viewportView)
	}
	v.Cursor = c
	v.AltScreen = true
	return v
}

func (m *Model) ContentString() string {
	value := make([]string, len(m.msgs))
	for i, msg := range m.msgs {
		value[i] = fmt.Sprintf("%s:%s", msg.Role, msg.Content)
	}
	return lipgloss.NewStyle().Width(m.viewport.Width()).Render(strings.Join(value, "\n"))
}
