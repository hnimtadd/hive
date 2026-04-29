package help

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hnimtadd/hive/internal/tui"
	"github.com/hnimtadd/hive/internal/tui/keys"
)

// Model represents the help overlay.
type Model struct {
	width  int
	height int
	active bool
}

// NewModel creates a new help overlay model.
func NewModel() *Model {
	return &Model{}
}

// SetSize sets the dimensions of the help overlay.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Toggle toggles the help overlay visibility.
func (m *Model) Toggle() {
	m.active = !m.active
}

// IsActive returns whether the help overlay is visible.
func (m *Model) IsActive() bool {
	return m.active
}

// Update handles messages for the help overlay model.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if resizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(resizeMsg.Width, resizeMsg.Height)
	}
	return nil
}

// View renders the help overlay.
func (m *Model) View() string {
	if !m.active {
		return ""
	}

	// Handle edge case: if dimensions are 0, return empty
	if m.width == 0 || m.height == 0 {
		return ""
	}

	content := m.buildModalContent()
	modal := m.buildModal(content)

	// Center the modal on screen
	return tui.DefaultContainer.Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center).Render(modal)
}

// buildModalContent builds the keybindings list content.
func (m *Model) buildModalContent() string {
	keyMappings := keys.HiveKeys.Mapping()

	// Sort modes to ensure consistent display order (Normal first, then Insert)
	modes := make([]tui.Mode, 0, len(keyMappings))
	for mode := range keyMappings {
		modes = append(modes, mode)
	}
	slices.Sort(modes)

	builder := &strings.Builder{}

	for i, mode := range modes {
		// Add mode header
		builder.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(tui.Accent).
			Render(mode.Long()))
		builder.WriteString("\n")

		bindings := keyMappings[mode]
		for _, binding := range bindings {
			help := binding.Help()
			// Format as two-column: key on left, description on right
			line := fmt.Sprintf("  %-15s %s\n", help.Key, help.Desc)
			builder.WriteString(line)
		}

		// Add spacing between mode sections (but not after the last one)
		if i < len(modes)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// buildModal wraps content in header/footer and centers it.
func (m *Model) buildModal(content string) string {
	modalWidth := m.width * 60 / 100
	modalWidth = max(modalWidth, 20)
	modalWidth = min(modalWidth, m.width-4)

	// Calculate internal width (subtract 2 for the left/right border characters)
	innerWidth := modalWidth - 2

	// 1. Shared Base Style for Background
	// We use .Width(innerWidth) to force the background to paint the full width
	baseStyle := tui.DefaultContainer.
		Background(tui.Background).
		Width(innerWidth)

	// 2. Specialized Styles
	headerStyle := baseStyle.
		Bold(true).
		Foreground(tui.Foreground).
		Padding(0, 1)

	// Body style needs to handle multi-line filling
	contentStyle := baseStyle.
		Foreground(tui.Foreground).
		Padding(0, 1)

	footerStyle := baseStyle.
		Foreground(tui.Muted).
		Padding(0, 1).
		Align(lipgloss.Center)

	// 3. Render the parts
	header := headerStyle.Render("Keybindings")
	body := contentStyle.Render(content)
	footer := footerStyle.Render("Press ? to close")

	// 4. Join them vertically
	card := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	// 5. Wrap in Border
	// We don't set a background on the borderStyle itself,
	// so the rounded corners look clean.
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.Accent)

	return borderStyle.Render(card)
}
