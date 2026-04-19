package keys

import (
	"charm.land/bubbles/v2/key"
)

type normal struct {
	Quit   key.Binding
	Insert key.Binding
	Clear  key.Binding
	Help   key.Binding
}

// Normal is a key map of keys available in Normal mode.
var Normal = normal{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("ctrl+c/q", "exit"),
	),
	Insert: key.NewBinding(
		key.WithKeys("i", "enter"),
		key.WithHelp("i/enter", "insert"),
	),
	Clear: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "clear chat"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}
