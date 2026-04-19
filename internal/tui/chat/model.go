package chat

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/pkg/types"
)

type ModelOptions struct {
	OnSendMessage func(string)
}

type Model struct {
	msgs []types.Message

	width, height int
	textarea      textarea.Model
	viewport      viewport.Model
}

func NewModel(opts ModelOptions) (*Model, error) {
	ti := textarea.New()
	ti.Placeholder = "Type your message... (Enter to send)"
	ti.Prompt = ""
	ti.ShowLineNumbers = false
	ti.MaxHeight = 6
	ti.SetHeight(3)
	defaultStylesStates := textarea.StyleState{
		Base:             tui.DefaultContainer,
		Text:             tui.DefaultContainer,
		CursorLine:       tui.DefaultContainer,
		EndOfBuffer:      tui.DefaultContainer,
		LineNumber:       tui.DefaultContainer,
		CursorLineNumber: tui.DefaultContainer,
		Placeholder:      tui.DefaultContainer,
		Prompt:           tui.DefaultContainer,
	}
	ti.SetStyles(textarea.Styles{
		Blurred: defaultStylesStates,
		Focused: defaultStylesStates,
	})

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	vp.Style = tui.DefaultContainer

	model := &Model{
		msgs:     []types.Message{},
		viewport: vp,
		textarea: ti,
	}

	return model, nil
}

func (m *Model) AddMessage(msg types.Message) {
	m.msgs = append(m.msgs, msg)
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()
}

func (m *Model) ClearMessages() {
	m.msgs = []types.Message{}
	m.viewport.SetContent(m.renderMessages())
}

func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		m.viewport.SetWidth(m.width)
		m.viewport.SetHeight(m.height - m.textarea.Height())
		m.textarea.SetWidth(m.width)
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case tea.FocusMsg:
		m.textarea.Focus()

	case tea.BlurMsg:
		m.textarea.Blur()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if m.textarea.Value() != "" {
				m.msgs = append(m.msgs, types.NewMessage(types.RoleUser, m.textarea.Value()))
				m.viewport.SetContent(m.renderMessages())
				content := m.textarea.Value()
				m.textarea.Reset()
				m.viewport.GotoBottom()
				cmds = append(
					cmds,
					tui.MsgCmd(tui.ChangeStatusMsg(tui.StatusThinking)),
					tui.MsgCmd(SendMessageMsg{Content: content}),
				)
			}
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}
	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	case ResponseMsg:
		cmds = append(cmds, tui.MsgCmd(tui.ChangeStatusMsg(tui.StatusReady)))
		if msg.Error != nil {
			m.msgs = append(m.msgs, types.NewMessage(types.RoleAssistant, fmt.Sprintf("Error: %v", msg.Error)))
		} else {
			m.msgs = append(m.msgs, types.NewMessage(types.RoleAssistant, msg.Content))
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
	}

	return tea.Batch(cmds...)
}

func (m *Model) View() string {
	return lipgloss.JoinVertical(lipgloss.Top,
		m.viewport.View(),
		tui.Regular.Width(m.width).Height(m.textarea.Height()).AlignHorizontal(lipgloss.Center).Render(m.textarea.View()),
	)
}

func (m *Model) renderMessages() string {
	if len(m.msgs) == 0 {
		welcome := lipgloss.NewStyle().
			Width(m.viewport.Width()).
			Foreground(tui.Muted).
			Align(lipgloss.Center).
			Render("Welcome to Hive Agentic Chat!\n\nPress 'i' to enter insert mode and start chatting.\nPress '?' for help.\n\n")
		return welcome
	}

	var lines []string
	for _, msg := range m.msgs {
		lines = append(lines, m.formatMessage(msg))
	}
	return lipgloss.NewStyle().
		Width(m.viewport.Width()).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) formatMessage(msg types.Message) string {
	var roleStyle, contentStyle lipgloss.Style

	switch msg.Role {
	case types.RoleUser:
		roleStyle = tui.Bold.Foreground(tui.Blue)
		contentStyle = tui.Regular.Foreground(tui.Foreground)
	case types.RoleAssistant:
		roleStyle = tui.Bold.Foreground(tui.Green)
		contentStyle = tui.Regular.Foreground(tui.Foreground)
	default:
		roleStyle = tui.Bold.Foreground(tui.Muted)
		contentStyle = tui.Regular.Foreground(tui.Muted)
	}

	roleLabel := string(msg.Role)
	if roleLabel == "user" {
		roleLabel = "You"
	}

	rolePart := roleStyle.Render(roleLabel + ":")
	lines := strings.Split(msg.Content, "\n")
	var contentLines []string
	for i, line := range lines {
		if i == 0 {
			contentLines = append(contentLines, contentStyle.Render(line))
		} else {
			contentLines = append(contentLines, contentStyle.PaddingLeft(lipgloss.Width(roleLabel)+2).Render(line))
		}
	}

	return rolePart + " " + strings.Join(contentLines, "\n")
}
