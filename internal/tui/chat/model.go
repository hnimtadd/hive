package chat

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/transport/client"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/inputbar"
)

type ModelOptions struct {
	Client *client.Client
}

type Model struct {
	msgs          []tui.Model
	streaming     map[string]tui.Model
	width, height int
	inputBar      *inputbar.Model
	viewport      viewport.Model

	currentFeedbackConversationID string
	currentFeedbackTurnID         string
	currentFeedbackQuestion       string // New field to store question text

	client *client.Client
}

func NewModel(opts ModelOptions) (*Model, error) {
	inputBar, err := inputbar.NewModel(inputbar.ModelOptions{
		Width:  80,
		Height: 9,
	})
	if err != nil {
		return nil, err
	}

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)
	vp.Style = tui.DefaultContainer

	model := &Model{
		msgs:      []tui.Model{},
		viewport:  vp,
		inputBar:  inputBar,
		streaming: make(map[string]tui.Model),
		client:    opts.Client,
	}

	return model, nil
}

func (m *Model) Init() tea.Cmd {
	return m.inputBar.Init()
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		m.inputBar.Update(msg)
		m.viewport.SetWidth(m.width)

		for _, model := range m.msgs {
			model.Update(msg)
		}

		m.viewport.SetHeight(m.height - m.inputBar.Height())
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+enter":
			if m.inputBar.Value() != "" {
				content := m.inputBar.Value()
				m.msgs = append(m.msgs, newChatRequestModel(content, m.width))
				// Send as feedback if feedback task is pending, otherwise as regular message
				if m.currentFeedbackConversationID != "" && m.currentFeedbackTurnID != "" {
					cmds = append(cmds, tui.MsgCmd(FeedbackResponseMsg{
						ConversationID: m.currentFeedbackConversationID,
						TurnID:         m.currentFeedbackTurnID,
						Response:       content,
					}))
					m.currentFeedbackConversationID = ""
					m.currentFeedbackTurnID = ""
					m.currentFeedbackQuestion = ""
					m.inputBar.ClearFeedback()
				} else {
					cmds = append(cmds, tui.MsgCmd(SendMessageMsg{Content: content}))
				}
				m.viewport.SetContent(m.renderMessages())
				m.inputBar.Reset()
				m.viewport.GotoBottom()
			}
		default:
			cmds = append(cmds, m.inputBar.Update(msg))
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
		m.currentFeedbackConversationID = msg.ConversationID
		m.currentFeedbackTurnID = msg.TurnID
		m.currentFeedbackQuestion = msg.Question
		// Set feedback mode in input bar
		m.inputBar.SetFeedback(msg.Question)
		// Switch to insert mode to allow user input (but context is now feedback)
		cmds = append(cmds, tui.MsgCmd(tui.ChangeModeMsg(tui.ModeInsert)))
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case tui.ClearChatMsg:
		// Clear all messages if any exist
		if len(m.msgs) > 0 {
			m.reset()
		}

	case tui.ChangeModeMsg:
		m.inputBar.SetMode(tui.Mode(msg))
		cmds = append(cmds, m.inputBar.Update(msg))

	case tea.BlurMsg:
		m.inputBar.Blur()

	case tea.FocusMsg:
		m.inputBar.Focus()

	case tui.OpenConversationMsg:
		if msg.New {
			cmds = append(cmds, tui.MsgCmd(tui.ClearChatMsg{}))
		} else {
			conversation, err := m.client.GetSession(msg.ConversationID)
			if err != nil {
				cmds = append(cmds, tui.MsgCmd(tui.ErrorMsg(err)))
			}

			m.reset()
			for _, msg := range conversation.Messages {
				m.msgs = append(m.msgs, newChatRequestModel(msg.Content, m.width))
			}
			m.viewport.SetContent(m.renderMessages())
			m.inputBar.Reset()
			m.viewport.GotoBottom()
		}

	default:
		cmds = append(cmds, m.inputBar.Update(msg))
	}

	return tea.Batch(cmds...)
}

func (m *Model) View() string {
	return lipgloss.JoinVertical(lipgloss.Top,
		m.viewport.View(),
		m.inputBar.View(),
	)
}

func (m *Model) reset() {
	m.msgs = []tui.Model{}
	m.streaming = make(map[string]tui.Model)
	m.currentFeedbackConversationID = ""
	m.currentFeedbackTurnID = ""
	m.currentFeedbackQuestion = ""
	m.inputBar.Reset()
	m.inputBar.ClearFeedback()
	m.viewport.SetContent(m.renderMessages())

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
