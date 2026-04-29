package chat

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

type ModelOptions struct{}

type Model struct {
	msgs      []tui.Model
	streaming map[string]tui.Model

	width, height int
	input         textarea.Model
	viewport      viewport.Model

	currentFeedbackTaskID   string
	currentFeedbackQuestion string // New field to store question text
}

func NewModel(_ ModelOptions) (*Model, error) {
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
		msgs:      []tui.Model{},
		viewport:  vp,
		input:     ti,
		streaming: make(map[string]tui.Model),
	}

	return model, nil
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

		for _, model := range m.msgs {
			model.Update(msg)
		}

		m.viewport.SetHeight(m.height - m.input.Height())
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+enter":
			if m.input.Value() != "" {
				content := m.input.Value()
				m.msgs = append(m.msgs, newChatRequestModel(content, m.width))
				// Send as feedback if feedback task is pending, otherwise as regular message
				if m.currentFeedbackTaskID != "" {
					cmds = append(cmds, tui.MsgCmd(FeedbackResponseMsg{
						TaskID:   m.currentFeedbackTaskID,
						Response: content,
					}))
					m.currentFeedbackTaskID = ""
					m.currentFeedbackQuestion = ""
				} else {
					cmds = append(cmds, tui.MsgCmd(SendMessageMsg{Content: content}))
				}
				m.viewport.SetContent(m.renderMessages())
				m.input.Reset()
				m.viewport.GotoBottom()
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
		}

	case StreamStartMsg:
		model := newChatResponseModel(msg.TaskID, m.width)
		m.msgs = append(m.msgs, model)
		m.streaming[msg.TaskID] = model
		cmds = append(cmds, tui.MsgCmd(tui.ChangeStatusMsg(tui.StatusThinking)))
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case StreamChunkMsg:
		model, isStreaming := m.streaming[msg.TaskID]
		if isStreaming {
			cmds = append(cmds, model.Update(msg))
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}

	case StreamCompleteMsg:
		model, isStreaming := m.streaming[msg.TaskID]
		if isStreaming {
			cmds = append(cmds, model.Update(msg))
			delete(m.streaming, msg.TaskID)

			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
		}

		cmds = append(cmds, tui.MsgCmd(tui.ChangeStatusMsg(tui.StatusReady)))

	case FeedbackRequestMsg:
		// Store the task ID and question for feedback response
		m.currentFeedbackTaskID = msg.TaskID
		m.currentFeedbackQuestion = msg.Question
		// Switch to insert mode to allow user input (but context is now feedback)
		cmds = append(cmds, tui.MsgCmd(tui.ChangeModeMsg(tui.ModeInsert)))
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case tui.ClearChatMsg:
		// Clear all messages if any exist
		if len(m.msgs) > 0 {
			m.msgs = []tui.Model{}
			m.streaming = make(map[string]tui.Model)
			m.currentFeedbackTaskID = ""
			m.currentFeedbackQuestion = ""
			m.input.Reset()
			m.viewport.SetContent(m.renderMessages())
		}

	case tea.BlurMsg:
		m.input.Blur()

	case tea.FocusMsg:
		m.input.Focus()

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *Model) View() string {
	inputArea := m.input.View()

	// If feedback is pending, render the question prominently above input
	if m.currentFeedbackQuestion != "" {
		feedbackPrompt := m.renderFeedbackPrompt()
		inputArea = lipgloss.JoinVertical(lipgloss.Left, feedbackPrompt, inputArea)
	}

	inputBg := lipgloss.NewStyle().Background(tui.InputBg).Width(m.width)
	return lipgloss.JoinVertical(lipgloss.Top,
		m.viewport.View(),
		inputBg.Render(inputArea),
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
	for _, msg := range m.msgs {
		cards = append(cards, msg.View())
	}
	return lipgloss.NewStyle().
		Width(m.viewport.Width()).
		Background(tui.Background).
		Render(strings.Join(cards, "\n"))
}

// renderFeedbackPrompt renders the feedback question above the input.
func (m *Model) renderFeedbackPrompt() string {
	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tui.Accent).
		Background(tui.InputBg).
		Padding(0, 1).
		Width(m.width - 2)

	questionStyle := lipgloss.NewStyle().
		Foreground(tui.Foreground).
		Background(tui.InputBg).
		Padding(0, 1).
		Width(m.width - 2)

	separator := lipgloss.NewStyle().
		Foreground(tui.Muted).
		Background(tui.InputBg).
		Render(strings.Repeat("─", m.width-2))

	prompt := promptStyle.Render("Agent asks:")
	question := questionStyle.Render(m.currentFeedbackQuestion)

	return lipgloss.JoinVertical(lipgloss.Left,
		prompt,
		question,
		separator,
	)
}
