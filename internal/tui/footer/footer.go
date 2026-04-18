package footer

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/version"
)

type ModelOptions struct {
	Mode tui.Mode
}

type Model struct {
	err           error
	info          string
	width, height int
	mode          tui.Mode
}

func NewModel(opts ModelOptions) (*Model, error) {
	model := &Model{
		mode: opts.Mode,
	}

	return model, nil
}

func (f *Model) Init() tea.Cmd {
	return nil
}

func (f *Model) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	// Handle error signal related msg
	switch msg := msg.(type) {
	case tui.ErrorMsg:
		f.err = msg
		return nil
	case tui.InfoMsg:
		f.info = string(msg)
		return nil
	}

	switch msg := msg.(type) {
	case tui.ChangeModeMsg:
		// Blindly set here at top model should take
		// responsbility to check the global mode
		f.mode = tui.Mode(msg)
	case tea.WindowSizeMsg:
		f.width = msg.Width
		f.height = msg.Height
	}
	return tea.Batch(cmds...)
}

func (f *Model) ResetStatus() {
	if f.err != nil {
		f.err = nil
	}
	f.info = ""
}

func (f *Model) View() tea.View {
	return tea.NewView(f.renderStatus())
}

var (
	modeWidgetStyle = tui.Padded.Background(tui.Accent).Foreground(tui.White)
	helpWidget      = tui.Padded.Background(tui.Accent).Foreground(tui.White).Render("? help")
	versionWidget   = tui.Padded.Background(tui.Accent).
			Foreground(tui.White).
			Render(version.Version)
)

func (f *Model) renderStatus() string {
	modeWidget := modeWidgetStyle.Render(f.mode.Short())

	// Compose footer
	footer := modeWidget
	switch {
	case f.err != nil:
		footer += tui.Regular.Padding(0, 1).
			Background(tui.Red).
			Foreground(tui.Foreground).
			Width(f.availableFooterMsgWidth(modeWidget, helpWidget, versionWidget)).
			Render(f.err.Error())
	case f.info != "":
		footer += tui.Padded.
			Background(tui.Background).
			Foreground(tui.Foreground).
			Width(f.availableFooterMsgWidth(modeWidget, helpWidget, versionWidget)).
			Render(f.info)
	default:
		footer += tui.Padded.
			Background(tui.Foreground).
			Foreground(tui.Foreground).
			Width(f.availableFooterMsgWidth(modeWidget, helpWidget, versionWidget)).
			Render("")
	}
	footer += helpWidget + versionWidget
	return tui.Regular.
		Inline(true).
		MaxWidth(f.width).
		Height(tui.FooterHeight).
		Width(f.width).
		Render(footer)
}

func (f *Model) availableFooterMsgWidth(components ...string) int {
	// -2 to accommodate padding
	curr := 0
	for _, component := range components {
		curr += lipgloss.Width(component)
	}
	return max(0, f.width-curr)
}
