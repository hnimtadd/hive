package keys

import (
	"charm.land/bubbles/v2/key"
)

type normal struct {
	Quit   key.Binding
	Insert key.Binding
}

// Normal is a key map of keys available in Normal mode.
var Normal = normal{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "exit"),
	),
	Insert: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "insert"),
	),
}
