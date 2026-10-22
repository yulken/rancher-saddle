package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpModel represents the help overlay
type HelpModel struct {
	visible bool
	width   int
	height  int
}

// NewHelpModel creates a new help overlay
func NewHelpModel() HelpModel {
	return HelpModel{
		visible: false,
		width:   60,
		height:  20,
	}
}

// Toggle shows/hides the help overlay
func (m *HelpModel) Toggle() {
	m.visible = !m.visible
}

// IsVisible returns whether the help is currently displayed
func (m HelpModel) IsVisible() bool {
	return m.visible
}

// View renders the help overlay
func (m HelpModel) View() string {
	if !m.visible {
		return ""
	}

	var s strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Align(lipgloss.Center).
		Width(m.width - 4)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	helpBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width).
		Background(lipgloss.Color("235"))

	// Build content
	s.WriteString(titleStyle.Render("⌨  Keyboard Shortcuts") + "\n")

	// Navigation section (nvim-style)
	s.WriteString(sectionStyle.Render("Navigation (nvim-style)") + "\n")
	s.WriteString(keyStyle.Render("  j/k, ↓/↑  ") + descStyle.Render("Move down/up") + "\n")
	s.WriteString(keyStyle.Render("  gg / G    ") + descStyle.Render("Jump to top / bottom") + "\n")
	s.WriteString(keyStyle.Render("  enter     ") + descStyle.Render("Select/Open") + "\n")
	s.WriteString(keyStyle.Render("  esc       ") + descStyle.Render("Back to dashboard") + "\n")

	// Command section
	s.WriteString(sectionStyle.Render("Command") + "\n")
	s.WriteString(keyStyle.Render("  <space>   ") + descStyle.Render("Leader (which-key popup)") + "\n")
	s.WriteString(keyStyle.Render("  <space> e ") + descStyle.Render("Toggle cluster explorer tree") + "\n")
	s.WriteString(keyStyle.Render("  :         ") + descStyle.Render("Command mode") + "\n")
	s.WriteString(keyStyle.Render("  /         ") + descStyle.Render("Search") + "\n")

	// Panes section
	s.WriteString(sectionStyle.Render("Panes / Buffers") + "\n")
	s.WriteString(keyStyle.Render("  ctrl+b    ") + descStyle.Render("Switch pane focus, then h/j/k/l or ←↓↑→") + "\n")
	s.WriteString(keyStyle.Render("  h/l       ") + descStyle.Render("(in tree) collapse / expand node") + "\n")
	s.WriteString(keyStyle.Render("  enter     ") + descStyle.Render("(in tree) open cluster logs") + "\n")

	// Actions section
	s.WriteString(sectionStyle.Render("Actions") + "\n")
	s.WriteString(keyStyle.Render("  d         ") + descStyle.Render("Delete cluster") + "\n")
	s.WriteString(keyStyle.Render("  enter     ") + descStyle.Render("Toggle logs panel") + "\n")
	s.WriteString(keyStyle.Render("  y         ") + descStyle.Render("Copy logs to clipboard") + "\n")
	s.WriteString(keyStyle.Render("  r         ") + descStyle.Render("Refresh now") + "\n")
	s.WriteString(keyStyle.Render("  q         ") + descStyle.Render("Quit/Exit") + "\n")

	// Credentials section
	s.WriteString(sectionStyle.Render("Credentials") + "\n")
	s.WriteString(keyStyle.Render("  ctrl+x    ") + descStyle.Render("Open credentials list") + "\n")
	s.WriteString(keyStyle.Render("  n         ") + descStyle.Render("New credential (static keys or AWS SSO)") + "\n")
	s.WriteString(keyStyle.Render("  r         ") + descStyle.Render("Refresh statuses (after logging in elsewhere)") + "\n")
	s.WriteString(keyStyle.Render("  enter     ") + descStyle.Render("Edit selected credential") + "\n")
	s.WriteString(keyStyle.Render("  d         ") + descStyle.Render("Delete selected credential") + "\n")

	// Help section
	s.WriteString(sectionStyle.Render("Help") + "\n")
	s.WriteString(keyStyle.Render("  ?         ") + descStyle.Render("Toggle this help") + "\n")
	s.WriteString(keyStyle.Render("  ctrl+c    ") + descStyle.Render("Force quit") + "\n")

	s.WriteString("\n")
	closeStyle := lipgloss.NewStyle().
		Faint(true).
		Foreground(lipgloss.Color("240")).
		Align(lipgloss.Center).
		Width(m.width - 4)
	s.WriteString(closeStyle.Render("[press any key to close]") + "\n")

	return helpBoxStyle.Render(s.String())
}

// Update handles help overlay events
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		if m.visible {
			// Close help on any key press
			m.visible = false
			return m, nil
		}
	}
	return m, nil
}
