package tui

const (
	ModeInsert Mode = "insert"
	ModeNormal Mode = "normal"
)
const DefaultMode = ModeNormal

type (
	Mode string
)

func (m Mode) Short() string {
	switch m {
	case ModeInsert:
		return "I"
	case ModeNormal:
		return "N"
	default:
		return "?"
	}
}

// Long returns the full display name for the mode.
func (m Mode) Long() string {
	switch m {
	case ModeInsert:
		return "Insert Mode"
	case ModeNormal:
		return "Normal Mode"
	default:
		return "Unknown Mode"
	}
}
