package keymap

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderWhichKey renders a which-key style popup listing the candidate bindings
// that can continue the current pending sequence. Pure presentation; the set of
// candidates comes from Registry.WhichKey / ResolveResult.Pending.
func RenderWhichKey(cands []Binding, width int) string {
	if len(cands) == 0 {
		return ""
	}

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("which-key"))
	b.WriteString("\n")
	for _, c := range cands {
		b.WriteString(keyStyle.Render(strings.Join(c.Keys, "")))
		b.WriteString("  ")
		b.WriteString(descStyle.Render(c.Desc))
		b.WriteString("\n")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)
	if width > 4 {
		box = box.Width(width - 2)
	}
	return box.Render(strings.TrimRight(b.String(), "\n"))
}
