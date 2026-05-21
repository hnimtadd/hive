package keys

import (
	"charm.land/bubbles/v2/key"
	"github.com/hnimtadd/hive/internal/tui"
)

type hive struct {
	Normal Norm
	Insert insert
}

// HiveKeys is a key map of keys available in hive modes.
var HiveKeys = hive{
	Normal: Norm{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q"),
			key.WithHelp("ctrl+c/q", "exit Hive"),
		),
		Insert: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "enter insert mode"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear current chat"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help panel"),
		),
		Sessions: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle sessions/chat view"),
		),
	},
	Insert: insert{
		Leave: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "leave insert mode"),
		),
	},
}

func (mapping hive) Mapping() map[tui.Mode][]key.Binding {
	return map[tui.Mode][]key.Binding{
		tui.ModeInsert: {
			mapping.Insert.Leave,
		},
		tui.ModeNormal: {
			mapping.Normal.Insert,
			mapping.Normal.Clear,
			mapping.Normal.Sessions,
			mapping.Normal.Quit,
			mapping.Normal.Help,
		},
	}
}
