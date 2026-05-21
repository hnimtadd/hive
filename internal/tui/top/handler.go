package top

import (
	tea "charm.land/bubbletea/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

func (m *model) handleOpenConversationMsg(msg tui.OpenConversationMsg) tea.Cmd {
	convID, respCh, err := m.client.StartConversation(m.ctx, msg.ConversationID)
	if err != nil {
		return tui.MsgCmd(tui.ErrorMsg(err))
	}
	// if current conversationID is not empty, which mean, we need to emit the
	// signal that we need to clear the old conversation.
	m.resetConversation()
	m.registerConversation(convID, respCh)
	m.content.Update(tui.ClearChatMsg{})
	// at this point, the activeconversation is already available inside model
	// we could subsitue and emit the message to the content model with the
	// new convID.
	msg.New = false
	msg.ConversationID = convID
	return m.content.Update(msg)
}

func (m *model) handleResizeMsg(msg tea.WindowSizeMsg) tea.Cmd { //nolint: unparam// This is conventional
	m.height, m.width = msg.Height, msg.Width
	mainHeight := max(0, msg.Height-tui.FooterHeight-tui.HeaderHeight)
	m.header.Update(tea.WindowSizeMsg{
		Height: tui.HeaderHeight,
		Width:  m.width,
	})
	m.footer.Update(tea.WindowSizeMsg{
		Height: tui.FooterHeight,
		Width:  m.width,
	})

	m.content.Update(tea.WindowSizeMsg{
		Height: mainHeight,
		Width:  m.width,
	})

	m.help.Update(tea.WindowSizeMsg{
		Height: mainHeight,
		Width:  m.width,
	})
	return nil
}

func (m *model) handleChangeModeMsg(msg tui.ChangeModeMsg) tea.Cmd {
	if m.mode != tui.Mode(msg) {
		m.mode = tui.Mode(msg)
		switch m.mode {
		case tui.ModeNormal:
			m.content.Update(tea.BlurMsg{})
		case tui.ModeInsert:
			m.content.Update(tea.FocusMsg{})
		}
		m.footer.Update(msg)
		// Forward the mode change to chat so inputbar can update
		return m.content.Update(msg)
	}
	return nil
}
