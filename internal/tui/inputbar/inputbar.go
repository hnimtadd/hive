package inputbar

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
)

type ModelOptions struct {
	Width  int
	Height int
}

type Model struct {
	textarea         textarea.Model
	width, height    int
	mode             tui.Mode
	feedbackPending  bool
	feedbackQuestion string
}

func NewModel(opts ModelOptions) (*Model, error) {
	ti := textarea.New()
	ti.Prompt = ""
	ti.CharLimit = 5000
	ti.ShowLineNumbers = false

	// Set fixed height to prevent jumping
	ti.SetHeight(3)
	ti.MaxHeight = 6

	// Define a consistent background style for ALL textarea elements
	// This ensures placeholder, text, cursor line, and empty lines all match
	baseStyle := tui.Regular.
		Background(tui.InputBg).
		Foreground(tui.Foreground)

	placeholderStyle := tui.Regular.
		Foreground(tui.Muted).
		Background(tui.InputBg)

	styles := textarea.Styles{}

	// Use tui.InputBg consistently for EVERYTHING
	styles.Focused.Base = baseStyle
	styles.Blurred.Base = baseStyle
	styles.Focused.CursorLine = baseStyle
	styles.Blurred.CursorLine = baseStyle
	styles.Focused.EndOfBuffer = baseStyle
	styles.Blurred.EndOfBuffer = baseStyle
	styles.Focused.Text = baseStyle
	styles.Blurred.Text = baseStyle
	styles.Focused.Placeholder = placeholderStyle
	styles.Blurred.Placeholder = placeholderStyle

	ti.SetStyles(styles)

	model := &Model{
		textarea: ti,
		width:    opts.Width,
		height:   opts.Height,
		mode:     tui.ModeNormal,
	}

	// Set the initial placeholder text based on mode
	model.setPlaceholder()

	return model, nil
}

func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(m.width - 4)
	}

	m.textarea, cmd = m.textarea.Update(msg)
	return cmd
}

func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Determine border color
	borderColor := tui.Muted
	if m.feedbackPending {
		borderColor = tui.Highlight
	} else if m.mode == tui.ModeInsert {
		borderColor = tui.Accent
	}

	// Always use textarea view for proper cursor and editing
	textareaContent := m.textarea.View()

	// Container style - use Background for consistency with textarea
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		BorderBackground(tui.InputBg).
		Background(tui.InputBg).
		Padding(0, 1).
		Width(m.width)

	borderedInput := containerStyle.Render(textareaContent)

	if m.feedbackPending && m.feedbackQuestion != "" {
		feedbackPrompt := m.renderFeedbackPrompt()
		return lipgloss.JoinVertical(lipgloss.Left,
			feedbackPrompt,
			borderedInput,
		)
	}

	return borderedInput
}

func (m *Model) SetMode(mode tui.Mode) {
	m.mode = mode
	m.setPlaceholder()
}

func (m *Model) SetFeedback(question string) {
	m.feedbackPending = true
	m.feedbackQuestion = question
	m.setPlaceholder()
}

func (m *Model) ClearFeedback() {
	m.feedbackPending = false
	m.feedbackQuestion = ""
	m.setPlaceholder()
}

func (m *Model) Value() string { return m.textarea.Value() }
func (m *Model) Reset()        { m.textarea.Reset() }
func (m *Model) Focus()        { m.textarea.Focus() }
func (m *Model) Blur()         { m.textarea.Blur() }

func (m *Model) Height() int {
	h := m.textarea.Height() + 2
	if m.feedbackPending {
		h += 3
	}
	return h
}

func (m *Model) setPlaceholder() {
	p := ""
	switch {
	case m.feedbackPending:
		p = "Type your response and press ctrl+enter..."
	case m.mode == tui.ModeNormal:
		p = "Type your message... (Press 'i' to start)"
	case m.mode == tui.ModeInsert:
		p = "Type your message and press ctrl+enter to send..."
	}
	m.textarea.Placeholder = p
}

func (m *Model) renderFeedbackPrompt() string {
	// Use InputBg for consistency with textarea
	bg := lipgloss.NewStyle().Background(tui.InputBg).Width(m.width)

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tui.Accent).
		Background(tui.InputBg).
		Padding(0, 1)

	questionStyle := lipgloss.NewStyle().
		Foreground(tui.Foreground).
		Background(tui.InputBg).
		Padding(0, 1)

	separator := lipgloss.NewStyle().
		Foreground(tui.Muted).
		Background(tui.InputBg).
		Margin(0, 1).
		Render(strings.Repeat("─", m.width-2))

	return bg.Render(lipgloss.JoinVertical(lipgloss.Left,
		promptStyle.Render("Agent asks:"),
		questionStyle.Render(m.feedbackQuestion),
		separator,
	))
}
