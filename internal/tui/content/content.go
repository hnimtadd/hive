package content

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/chat"
)

type ModelOptions struct {
	Chat *chat.Model
}

type Model struct {
	width, height int

	chat *chat.Model

	view          sessionsView
	conversations []conversationItem
	cursor        int
	activeChatID  string
	opts          *ModelOptions
}

type sessionsView string

const (
	viewConversationList sessionsView = "conversation_list"
	viewChat             sessionsView = "chat"
)

type conversationItem struct {
	ID    string
	Title string
}

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

func (m *Model) RegisterConversation(id string) {
	if id == "" {
		return
	}
	for _, conversation := range m.conversations {
		if conversation.ID == id {
			m.activeChatID = id
			return
		}
	}
	m.conversations = append([]conversationItem{{
		ID:    id,
		Title: fmt.Sprintf("Conversation %s", truncateID(id)),
	}}, m.conversations...)
	m.activeChatID = id
}

func truncateID(id string) string {
	const size = 8
	if len(id) <= size {
		return id
	}
	return id[:size]
}

func (m *Model) listItems() []conversationListItem {
	items := []conversationListItem{{
		Title: "Start a new conversation",
		New:   true,
	}}
	for _, conversation := range m.conversations {
		items = append(items, conversationListItem{
			ID:    conversation.ID,
			Title: conversation.Title,
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

func NewModel(opts *ModelOptions) (*Model, error) {
	if opts == nil {
		return nil, fmt.Errorf("model options cannot be nil")
	}
	if opts.Chat == nil {
		return nil, fmt.Errorf("chat model is required")
	}

	model := &Model{
		opts:   opts,
		chat:   opts.Chat,
		view:   viewConversationList,
		cursor: 0,
	}
	return model, nil
}

func (f *Model) Init() tea.Cmd {
	return f.chat.Init()
}

func (f *Model) Update(msg tea.Msg) tea.Cmd {
	switch keyMsg := msg.(type) {
	case tea.KeyMsg:
		if f.view != viewConversationList {
			return f.chat.Update(msg)
		}
		switch keyMsg.String() {
		case "up", "k":
			f.moveCursor(-1)
			return nil
		case "down", "j":
			f.moveCursor(1)
			return nil
		case "enter":
			return f.openSelectedConversation()
		default:
			return nil
		}
	case tea.WindowSizeMsg:
		f.width = keyMsg.Width
		f.height = keyMsg.Height
		return f.chat.Update(msg)
	default:
		return f.chat.Update(msg)
	}
}

func (f *Model) View() string {
	if f.view == viewChat {
		return f.chat.View()
	}
	return f.renderConversationList()
}
