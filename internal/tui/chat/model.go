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

	streamingContent strings.Builder
	isStreaming      bool
	streamingIndex   int // Index of the message currently streaming (-1 if none)
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
		msgs:           []types.Message{},
		viewport:       vp,
		input:          ti,
		streamingIndex: -1, // No message streaming initially
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

	case SendMessageMsg:
		return tui.MsgCmd(msg)

	case StreamChunkMsg:
		if msg.IsFirst {
			m.streamingContent.Reset()
			m.isStreaming = true
		}
		m.streamingContent.WriteString(msg.Content)

		// Update or create assistant message
		if len(m.msgs) > 0 && m.msgs[len(m.msgs)-1].Role == types.RoleAssistant && m.isStreaming {
			m.msgs[len(m.msgs)-1].Content = m.streamingContent.String()
			m.streamingIndex = len(m.msgs) - 1
		} else if m.isStreaming {
			m.msgs = append(m.msgs, types.NewMessage(types.RoleAssistant, m.streamingContent.String()))
			m.streamingIndex = len(m.msgs) - 1
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case StreamCompleteMsg:
		// Update or create assistant message
		if m.isStreaming {
			if msg.Error != nil {
				m.msgs[len(m.msgs)-1].Content = fmt.Sprintf("Error: %s", msg.Error.Error())
			} else {
				m.msgs[len(m.msgs)-1].Content = msg.Content
			}
		} else {
			if msg.Error != nil {
				m.msgs = append(m.msgs, types.NewMessage(types.RoleAssistant, fmt.Sprintf("Error: %s", msg.Error.Error())))
			} else {
				m.msgs = append(m.msgs, types.NewMessage(types.RoleAssistant, msg.Content))
			}
		}
		m.streamingIndex = -1
		m.isStreaming = false

		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

		cmds = append(cmds, tui.MsgCmd(tui.ChangeStatusMsg(tui.StatusReady)))

	case FeedbackRequestMsg:
		// For now, just display the question as an assistant message
		m.msgs = append(m.msgs, types.NewMessage(types.RoleAssistant, "Feedback requested: "+msg.Question))
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

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

	var cards []string
	for i, msg := range m.msgs {
		isStreaming := (i == m.streamingIndex)
		cards = append(cards, m.formatMessage(msg, isStreaming))
	}
	return lipgloss.NewStyle().
		Width(m.viewport.Width()).
		Background(tui.Background).
		Render(strings.Join(cards, "\n\n"))
}

func (m *Model) formatMessage(msg types.Message, isStreaming bool) string {
	// Leave margin for borders
	cardWidth := max(20, m.viewport.Width()-4)

	var (
		headerLabel     string
		streamIndicator string
	)

	// Configure card style based on role
	var headerStyle, contentStyle, cardBorder lipgloss.Style

	switch msg.Role {
	case types.RoleUser:
		headerLabel = "Input"
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(tui.Blue).
			Background(tui.InputBg).
			Padding(0, 1)
		contentStyle = lipgloss.NewStyle().
			Width(cardWidth).
			Background(tui.InputBg).
			Foreground(tui.Foreground).
			Padding(0, 1)
		cardBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tui.Blue).
			Width(cardWidth)

	case types.RoleAssistant:
		headerLabel = "Output"
		if isStreaming {
			streamIndicator = " thinking..."
		}
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(tui.Green).
			Background(tui.InputBg).
			Padding(0, 1)
		contentStyle = lipgloss.NewStyle().
			Width(cardWidth).
			Background(tui.InputBg).
			Foreground(tui.Foreground).
			Padding(0, 1)
		cardBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tui.Green).
			Width(cardWidth)

	default:
		headerLabel = "Message"
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(tui.Muted).
			Background(tui.InputBg).
			Padding(0, 1)
		contentStyle = lipgloss.NewStyle().
			Width(cardWidth).
			Background(tui.InputBg).
			Foreground(tui.Foreground).
			Padding(0, 1)
		cardBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(tui.Muted).
			Width(cardWidth)
	}

	// Format content
	content := msg.Content
	if content == "" && isStreaming {
		content = "..."
	}

	// Build the card
	header := headerStyle.Render(headerLabel + streamIndicator)
	body := contentStyle.Render(content)

	// Combine header and body, then wrap in border
	card := lipgloss.JoinVertical(lipgloss.Left, header, body)
	return cardBorder.Render(card)
}
