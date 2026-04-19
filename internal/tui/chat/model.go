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
	input         textarea.Model
	viewport      viewport.Model
}

func NewModel(opts ModelOptions) (*Model, error) {
	ti := textarea.New()
	ti.Placeholder = ""
	ti.Prompt = ""
	ti.CharLimit = 5000
	ti.ShowLineNumbers = false
	ti.SetHeight(3)
	ti.MaxHeight = 6

	inputBg := lipgloss.NewStyle().Background(tui.InputBg).Foreground(tui.Foreground)
	inputText := tui.Regular.Foreground(tui.Foreground)

	styles := textarea.DefaultDarkStyles()
	styles.Focused.Base = inputBg
	styles.Focused.CursorLine = inputBg
	styles.Focused.Text = inputText
	styles.Blurred.Base = inputBg
	styles.Blurred.CursorLine = inputBg
	styles.Blurred.Text = inputText
	styles.Focused.Placeholder = lipgloss.NewStyle()
	styles.Blurred.Placeholder = lipgloss.NewStyle()
	ti.SetStyles(styles)

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	vp.Style = tui.DefaultContainer

	model := &Model{
		msgs:     []types.Message{},
		viewport: vp,
		input:    ti,
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

		m.input.SetWidth(m.width)

		m.viewport.SetWidth(m.width)
		m.viewport.SetHeight(m.height - m.input.Height())
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case tea.FocusMsg:
		m.input.Focus()

	case tea.BlurMsg:
		m.input.Blur()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+enter":
			if m.input.Value() != "" {
				m.msgs = append(m.msgs, types.NewMessage(types.RoleUser, m.input.Value()))
				m.viewport.SetContent(m.renderMessages())
				content := m.input.Value()
				m.input.Reset()
				m.viewport.GotoBottom()
				cmds = append(
					cmds,
					tui.MsgCmd(tui.ChangeStatusMsg(tui.StatusThinking)),
					tui.MsgCmd(SendMessageMsg{Content: content}),
				)
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}
	case cursor.BlinkMsg:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
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
	inputBg := lipgloss.NewStyle().Background(tui.InputBg).Width(m.width).Height(m.input.Height())
	return lipgloss.JoinVertical(lipgloss.Top,
		m.viewport.View(),
		inputBg.Render(m.input.View()),
	)
}

func (m *Model) renderMessages() string {
	if len(m.msgs) == 0 {
		welcome := lipgloss.NewStyle().
			Width(m.viewport.Width()).
			Background(tui.Background).
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
