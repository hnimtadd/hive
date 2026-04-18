package tui

import tea "charm.land/bubbletea/v2"

var NoopCmd tea.Cmd = nil

func MsgCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}
