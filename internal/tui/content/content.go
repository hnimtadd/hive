package content

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/transport/client"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/chat"
	"github.com/hnimtadd/hive/pkg/types"
)

type ModelOptions struct {
	Chat   *chat.Model
	Client *client.Client
}

type Model struct {
	width, height int

	chat   *chat.Model
	client *client.Client

	view          sessionsView
	conversations []*types.Session
	cursor        int
	activeChatID  string
}

func NewModel(opts *ModelOptions) (*Model, error) {
	if opts == nil {
		return nil, errors.New("model options cannot be nil")
	}
	if opts.Chat == nil {
		return nil, errors.New("chat model is required")
	}

	model := &Model{
		chat:   opts.Chat,
		client: opts.Client,
		view:   viewConversationList,
		cursor: 0,
	}
	return model, nil
}

func (m *Model) Init() tea.Cmd {
	sessions, err := m.client.ListSessions()
	if err != nil {
		return tui.MsgCmd(tui.ErrorMsg(err))
	}
	m.conversations = sessions
	return m.chat.Init()
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch keyMsg := msg.(type) {
	case tea.KeyMsg:
		if m.view != viewConversationList {
			return m.chat.Update(msg)
		}
		switch keyMsg.String() {
		case "up", "k":
			m.moveCursor(-1)
			return nil
		case "down", "j":
			m.moveCursor(1)
			return nil
		case "enter":
			return m.openSelectedConversation()
		default:
			return nil
		}
	case tea.WindowSizeMsg:
		m.width = keyMsg.Width
		m.height = keyMsg.Height
		return m.chat.Update(msg)
	default:
		return m.chat.Update(msg)
	}
}

func (m *Model) View() string {
	if m.view == viewChat {
		return m.chat.View()
	}
	return m.renderConversationList()
}

type sessionsView string

const (
	viewConversationList sessionsView = "conversation_list"
	viewChat             sessionsView = "chat"
)

type OpenConversationMsg struct {
	ConversationID string
	New            bool
}

type conversationListItem struct {
	ID    string
	Title string
	New   bool
}

func (m *Model) IsChatView() bool {
	return m.view == viewChat
}

func (m *Model) ShowChat() {
	m.view = viewChat
}

func (m *Model) ShowConversationList() {
	m.view = viewConversationList
}

func (m *Model) ToggleView() {
	if m.view == viewChat {
		m.ShowConversationList()
		return
	}
	m.ShowChat()
}

func (m *Model) listItems() []conversationListItem {
	items := []conversationListItem{{
		Title: "Start a new conversation",
		New:   true,
	}}
	for _, conversation := range m.conversations {
		items = append(items, conversationListItem{
			ID: conversation.ID,
			// TODO: support title update
			Title: conversation.Location,
		})
	}
	return items
}

func (m *Model) moveCursor(delta int) {
	items := m.listItems()
	if len(items) == 0 {
		m.cursor = 0
		return
	}
	m.cursor = (m.cursor + delta + len(items)) % len(items)
}

func (m *Model) openSelectedConversation() tea.Cmd {
	items := m.listItems()
	if len(items) == 0 {
		return nil
	}
	item := items[m.cursor]
	m.ShowChat()
	if item.New {
		m.activeChatID = ""
		return tui.MsgCmd(OpenConversationMsg{New: true})
	}
	m.activeChatID = item.ID
	return tui.MsgCmd(OpenConversationMsg{
		ConversationID: item.ID,
	})
}

func (m *Model) renderConversationList() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(tui.Accent).Render("Sessions")
	subtitle := lipgloss.NewStyle().Foreground(tui.Muted).
		Render("Select a conversation to open chat view.")

	items := m.listItems()
	lines := make([]string, 0, len(items))
	for idx, item := range items {
		prefix := "  "
		lineStyle := lipgloss.NewStyle().Foreground(tui.Foreground)
		if idx == m.cursor {
			prefix = "> "
			lineStyle = lineStyle.Foreground(tui.Accent).Bold(true)
		}

		title := item.Title
		if item.ID != "" && item.ID == m.activeChatID {
			title += " (active)"
		}
		lines = append(lines, lineStyle.Render(prefix+title))
	}

	body := lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		header,
		subtitle,
		"",
		strings.Join(lines, "\n"),
	))

	// Force-fill the full sessions page area with themed background.
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left,
		lipgloss.Top,
		body,
		lipgloss.WithWhitespaceStyle(tui.DefaultContainer),
	)
}

func (m *Model) RegisterConversation(id string) error {
	if id == "" {
		return errors.New("id must be not nil")
	}
	for _, conversation := range m.conversations {
		if conversation.ID == id {
			m.activeChatID = id
			return nil
		}
	}
	conversation, err := m.client.GetSession(id)
	if err != nil {
		return err
	}
	m.conversations = append([]*types.Session{conversation}, m.conversations...)
	m.activeChatID = id
	return nil
}
