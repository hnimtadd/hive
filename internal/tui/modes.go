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
