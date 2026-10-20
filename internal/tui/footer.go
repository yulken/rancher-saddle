package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/tui/views"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// FooterModel represents the persistent footer with context-aware keybindings
type FooterModel struct {
	width  int
	height int
}

// keyMap defines keybindings for each state
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Back    key.Binding
	New     key.Binding
	Delete  key.Binding
	Refresh key.Binding
	Upgrade key.Binding
	Help    key.Binding
	Quit    key.Binding
}

var (
	clusterListKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "logs"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new cluster"),
		),
		Delete: key.NewBinding(
			key.WithKeys("x", "d"),
			key.WithHelp("x/d", "delete"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Upgrade: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "upgrade"),
		),
		Back: key.NewBinding(
			key.WithKeys("ctrl+x", "ctrl+p", "ctrl+a"),
			key.WithHelp("ctrl+x/ctrl+p/ctrl+a", "creds/profiles/amis"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}

	credentialsListKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit"),
		),
		New: key.NewBinding(
			key.WithKeys("n", "c"),
			key.WithHelp("n", "new credential"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	credentialsFormKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "shift+tab"),
			key.WithHelp("↑/shift+tab", "prev"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "tab"),
			key.WithHelp("↓/tab", "next"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "save"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	createFormKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "shift+tab"),
			key.WithHelp("↑/shift+tab", "prev field"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "tab", "left", "right"),
			key.WithHelp("↓/tab/◀/▶", "navigate/select"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter", "ctrl+p"),
			key.WithHelp("enter/ctrl+p", "submit/profile"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
	}

	profilesListKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit"),
		),
		New: key.NewBinding(
			key.WithKeys("n", "c"),
			key.WithHelp("n", "new profile"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	profilesFormKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "shift+tab"),
			key.WithHelp("↑/shift+tab", "prev"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "tab"),
			key.WithHelp("↓/tab", "next"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "save"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	amisListKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "edit"),
		),
		New: key.NewBinding(
			key.WithKeys("n", "c"),
			key.WithHelp("n", "new entry"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	amisFormKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "shift+tab"),
			key.WithHelp("↑/shift+tab", "prev"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "tab"),
			key.WithHelp("↓/tab", "next"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "save"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	upgradeFormKeys = keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "shift+tab"),
			key.WithHelp("↑/shift+tab", "prev"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "tab"),
			key.WithHelp("↓/tab", "next"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "upgrade"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	deleteModalKeys = keyMap{
		Enter: key.NewBinding(
			key.WithKeys("y", "enter"),
			key.WithHelp("y/enter", "confirm"),
		),
		Back: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "cancel"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
	}
)

// NewFooterModel creates a new footer
func NewFooterModel() FooterModel {
	return FooterModel{width: 80}
}

// SetWidth updates the footer width
func (f *FooterModel) SetWidth(width int) {
	f.width = width
}

// SetHeight updates the total terminal height (used to calculate log panel size)
func (f *FooterModel) SetHeight(height int) {
	f.height = height
}

// ViewForState renders the footer with all keybindings for the current state.
func (f FooterModel) ViewForState(state views.AppState) string {
	return f.ViewForStateWithError(state, "")
}

// ViewForStateWithError renders the footer with keybindings plus an optional
// red error line above them (used to surface e.g. failed-delete reasons).
func (f FooterModel) ViewForStateWithError(state views.AppState, errMsg string) string {
	footerStyle := lipgloss.NewStyle().
		Width(f.width).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("250")).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		BorderTop(true)

	var km keyMap
	switch state {
	case views.StateClusterList:
		km = clusterListKeys
	case views.StateCreateForm:
		km = createFormKeys
	case views.StateDeleteConfirm:
		km = deleteModalKeys
	case views.StateCredentialsList:
		km = credentialsListKeys
	case views.StateCredentialsForm:
		km = credentialsFormKeys
	case views.StateProfilesList:
		km = profilesListKeys
	case views.StateProfilesForm:
		km = profilesFormKeys
	case views.StateAMIsList:
		km = amisListKeys
	case views.StateAMIsForm:
		km = amisFormKeys
	case views.StateUpgradeForm:
		km = upgradeFormKeys
	default:
		km = clusterListKeys
	}

	body := renderBindings(km)
	if errMsg != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		body = errStyle.Render("✗ "+errMsg) + "\n" + body
	}
	return footerStyle.Render(body)
}

// renderBindings formats all non-empty bindings from km as a single line.
func renderBindings(km keyMap) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)
	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	sep := sepStyle.Render(" • ")

	all := []key.Binding{
		km.Up, km.Down, km.Enter, km.New, km.Delete,
		km.Refresh, km.Upgrade, km.Back, km.Help, km.Quit,
	}

	var parts []string
	for _, b := range all {
		h := b.Help()
		if h.Key == "" || h.Desc == "" {
			continue
		}
		parts = append(parts, keyStyle.Render(h.Key)+" "+h.Desc)
	}

	return strings.Join(parts, sep)
}

// ViewWithLogs renders the footer with deployment logs using 33% of the screen
func (f FooterModel) ViewWithLogs(clusterName string) string {
	// Calculate log panel height: 33% of terminal, minimum 6 lines
	logPanelHeight := f.height / 3
	if logPanelHeight < 6 {
		logPanelHeight = 6
	}

	// Account for border (1) and title line (1)
	logLines := logPanelHeight - 2
	if logLines < 3 {
		logLines = 3
	}

	logStyle := lipgloss.NewStyle().
		Width(f.width).
		Height(logPanelHeight).
		Background(lipgloss.Color("234")).
		Foreground(lipgloss.Color("250")).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		BorderTop(true)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	// Read logs from file
	logPath := fmt.Sprintf("logs/%s.log", clusterName)
	logs := readLastLines(logPath, logLines)

	title := titleStyle.Render(fmt.Sprintf("Logs: %s", clusterName))
	helpText := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true).Render(" • Enter: hide")

	content := title + helpText + "\n" + logs

	return logStyle.Render(content)
}

// readLastLines reads the last n lines from a file
func readLastLines(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "No logs available yet..."
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}
