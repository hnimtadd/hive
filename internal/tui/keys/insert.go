package keys

import "charm.land/bubbles/v2/key"

type insert struct {
	Leave key.Binding
}

// Insert is a key map of keys available in insert mode.
var Insert = insert{
	Leave: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "leave command"),
	),
}
