package views

import (
	"fmt"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// dashBanner is the Astronvim/Tornado-style ASCII banner shown on the home screen.
var dashBanner = []string{
	" ██▀███   ▄▄▄       ███▄    █  ▄████▄   ██░ ██ ▓█████  ██▀███  ",
	"▓██ ▒ ██▒▒████▄     ██ ▀█   █ ▒██▀ ▀█  ▓██░ ██▒▓█   ▀ ▓██ ▒ ██▒",
	"▓██ ░▄█ ▒▒██  ▀█▄  ▓██  ▀█ ██▒▒▓█    ▄ ▒██▀▀██░▒███   ▓██ ░▄█ ▒",
	"▒██▀▀█▄  ░██▄▄▄▄██ ▓██▒  ▐▌██▒▒▓▓▄ ▄██▒░▓█ ░██ ▒▓█  ▄ ▒██▀▀█▄  ",
	"░██▓ ▒██▒ ▓█   ▓██▒▒██░   ▓██░▒ ▓███▀ ░░▓█▒░██▓░▒████▒░██▓ ▒██▒",
	"             t u i  —  manage the Rancher product family       ",
}

// dashItem is a single selectable action on the dashboard.
type dashItem struct {
	label  string
	target AppState
	isQuit bool
}

// dashMenu is the pure-logic selectable menu (navigation is unit-tested).
type dashMenu struct {
	items  []dashItem
	cursor int
}

func defaultDashItems() []dashItem {
	return []dashItem{
		{label: "Clusters", target: StateClusterList},
		{label: "Create Cluster", target: StateCreateForm},
		{label: "Credentials", target: StateCredentialsList},
		{label: "Profiles", target: StateProfilesList},
		{label: "AMIs", target: StateAMIsList},
		{label: "Quit", isQuit: true},
	}
}

func (m *dashMenu) moveDown() {
	if len(m.items) == 0 {
		return
	}
	m.cursor = (m.cursor + 1) % len(m.items)
}

func (m *dashMenu) moveUp() {
	if len(m.items) == 0 {
		return
	}
	m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
}

func (m dashMenu) selected() dashItem {
	return m.items[m.cursor]
}

// DashboardModel is the Astronvim-style home screen.
type DashboardModel struct {
	width  int
	height int
	menu   dashMenu
}

// NewDashboardModel creates the dashboard with its default action menu.
func NewDashboardModel() DashboardModel {
	return DashboardModel{
		width:  80,
		height: 24,
		menu:   dashMenu{items: defaultDashItems()},
	}
}

// Init implements the view contract.
func (m DashboardModel) Init() tea.Cmd { return nil }

// SetSize updates the dashboard dimensions.
func (m *DashboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles navigation and selection.
func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "down", "j":
			m.menu.moveDown()
		case "up", "k":
			m.menu.moveUp()
		case "enter":
			return m, m.activate()
		}
	}
	return m, nil
}

// activate returns the command for the currently selected menu item.
func (m DashboardModel) activate() tea.Cmd {
	sel := m.menu.selected()
	if sel.isQuit {
		return tea.Quit
	}
	target := sel.target
	return func() tea.Msg {
		return StateChangeMsg{NewState: target}
	}
}

// View renders the dashboard.
func (m DashboardModel) View() string {
	bannerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	banner := bannerStyle.Render(strings.Join(dashBanner, "\n"))

	itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Padding(0, 2)
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236")).
		Background(lipgloss.Color("86")).Bold(true).Padding(0, 2)

	var menuLines []string
	for i, it := range m.menu.items {
		line := it.label
		if i == m.menu.cursor {
			menuLines = append(menuLines, selStyle.Render("▶ "+line))
		} else {
			menuLines = append(menuLines, itemStyle.Render("  "+line))
		}
	}
	menu := lipgloss.JoinVertical(lipgloss.Center, menuLines...)

	summary := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.clusterSummary())

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Faint(true).
		Render("j/k move · enter select · <space> leader · : command · / search · q quit")

	body := lipgloss.JoinVertical(lipgloss.Center,
		banner,
		"",
		menu,
		"",
		summary,
		"",
		hint,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, body)
}

// clusterSummary reads the cluster inventory and returns a short status line.
func (m DashboardModel) clusterSummary() string {
	cfg, err := config.LoadClustersConfig("config.yaml")
	if err != nil || len(cfg.Clusters) == 0 {
		return "No clusters yet — press enter on \"Create Cluster\" to get started"
	}

	var names []string
	for name, c := range cfg.Clusters {
		names = append(names, fmt.Sprintf("%s (%s)", name, c.Status))
		if len(names) >= 3 {
			break
		}
	}
	return fmt.Sprintf("%d cluster(s): %s", len(cfg.Clusters), strings.Join(names, ", "))
}
