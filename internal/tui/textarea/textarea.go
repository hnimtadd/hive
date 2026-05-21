package textarea

import (
	"strings"

	btextarea "charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type Styles = btextarea.Styles

type Model struct {
	inner btextarea.Model
}

func New() Model {
	return Model{inner: btextarea.New()}
}

func Blink() tea.Msg {
	return btextarea.Blink()
}

func (m *Model) SetPrompt(prompt string) {
	m.inner.Prompt = prompt
}

func (m *Model) SetCharLimit(limit int) {
	m.inner.CharLimit = limit
}

func (m *Model) SetShowLineNumbers(enabled bool) {
	m.inner.ShowLineNumbers = enabled
}

func (m *Model) SetMaxHeight(height int) {
	m.inner.MaxHeight = height
}

func (m *Model) SetStyles(styles Styles) {
	m.inner.SetStyles(styles)
}

func (m *Model) SetWidth(width int) {
	m.inner.SetWidth(width)
}

func (m *Model) Width() int {
	return m.inner.Width()
}

func (m *Model) SetHeight(height int) {
	m.inner.SetHeight(height)
}

func (m *Model) Height() int {
	return m.inner.Height()
}

func (m *Model) SetPlaceholder(placeholder string) {
	m.inner.Placeholder = placeholder
}

func (m *Model) Value() string {
	return m.inner.Value()
}

func (m *Model) Reset() {
	m.inner.Reset()
}

func (m *Model) Focus() tea.Cmd {
	return m.inner.Focus()
}

func (m *Model) Blur() {
	m.inner.Blur()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.inner.Value() == "" && m.inner.Placeholder != "" {
		return m.placeholderView()
	}
	return m.inner.View()
}

func (m Model) placeholderView() string {
	width := max(1, m.inner.Width())
	height := max(1, m.inner.Height())

	styles := m.inner.Styles()
	state := styles.Blurred
	if m.inner.Focused() {
		state = styles.Focused
	}

	base := state.Base
	placeholderStyle := state.Placeholder.Inherit(base).Inline(true)
	whitespaceStyle := base

	wrapped := ansi.Wordwrap(m.inner.Placeholder, width, "")
	wrapped = ansi.Hardwrap(wrapped, width, true)
	lines := strings.Split(strings.TrimSpace(wrapped), "\n")

	rendered := make([]string, 0, height)
	for i := 0; i < height; i++ {
		text := ""
		if i < len(lines) {
			text = placeholderStyle.Render(lines[i])
		}
		rendered = append(rendered, lipgloss.Place(
			width,
			1,
			lipgloss.Left,
			lipgloss.Top,
			text,
			lipgloss.WithWhitespaceStyle(whitespaceStyle),
		))
	}

	return base.Render(strings.Join(rendered, "\n"))
}
