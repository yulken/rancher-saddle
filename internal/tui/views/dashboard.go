package views

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// embeddedBanner is the project banner art, compiled into the binary from
// rancher-logo.txt at build time. Edit that file to change the banner.
//
//go:embed rancher-logo.txt
var embeddedBanner string

// figletFallback is the built-in banner used only when neither the embedded
// art nor a custom banner file is available.
var figletFallback = []string{
	" ██▀███   ▄▄▄       ███▄    █  ▄████▄   ██░ ██ ▓█████  ██▀███  ",
	"▓██ ▒ ██▒▒████▄     ██ ▀█   █ ▒██▀ ▀█  ▓██░ ██▒▓█   ▀ ▓██ ▒ ██▒",
	"▓██ ░▄█ ▒▒██  ▀█▄  ▓██  ▀█ ██▒▒▓█    ▄ ▒██▀▀██░▒███   ▓██ ░▄█ ▒",
	"▒██▀▀█▄  ░██▄▄▄▄██ ▓██▒  ▐▌██▒▒▓▓▄ ▄██▒░▓█ ░██ ▒▓█  ▄ ▒██▀▀█▄  ",
	"░██▓ ▒██▒ ▓█   ▓██▒▒██░   ▓██░▒ ▓███▀ ░░▓█▒░██▓░▒████▒░██▓ ▒██▒",
	"             t u i  —  manage the Rancher product family       ",
}

// bannerSource identifies where the resolved banner came from.
type bannerSource int

const (
	bannerFiglet bannerSource = iota
	bannerEmbedded
	bannerCustom
)

// bannerFileName is the file the dashboard looks for to override the banner.
const bannerFileName = "banner.txt"

// bannerSearchPaths returns, in priority order, the locations checked for a
// custom banner file: the RANCHER_TUI_BANNER env var, a banner.txt in the
// working directory, then ~/.config/rancher-tui/banner.txt.
func bannerSearchPaths() []string {
	var paths []string
	if p := os.Getenv("RANCHER_TUI_BANNER"); p != "" {
		paths = append(paths, p)
	}
	paths = append(paths, bannerFileName)
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "rancher-tui", bannerFileName))
	}
	return paths
}

// parseBanner splits raw file contents into banner lines. It reports ok=false
// when the content is empty/whitespace-only so callers fall back to the default.
func parseBanner(raw string) (lines []string, ok bool) {
	text := strings.TrimRight(raw, "\r\n")
	if strings.TrimSpace(text) == "" {
		return nil, false
	}
	return strings.Split(text, "\n"), true
}

// loadBannerFrom returns the first readable, non-empty banner from paths, and
// whether one was found.
func loadBannerFrom(paths []string) (lines []string, found bool) {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if parsed, ok := parseBanner(string(data)); ok {
			return parsed, true
		}
	}
	return nil, false
}

// loadBanner resolves the banner to display: a custom override file wins, else
// the embedded rancher-logo.txt art, else the built-in figlet fallback.
func loadBanner() (lines []string, source bannerSource) {
	if parsed, ok := loadBannerFrom(bannerSearchPaths()); ok {
		return parsed, bannerCustom
	}
	if parsed, ok := parseBanner(embeddedBanner); ok {
		return parsed, bannerEmbedded
	}
	return figletFallback, bannerFiglet
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
	width        int
	height       int
	menu         dashMenu
	banner       []string
	bannerSource bannerSource
}

// NewDashboardModel creates the dashboard with its default action menu.
func NewDashboardModel() DashboardModel {
	banner, source := loadBanner()
	return DashboardModel{
		width:        80,
		height:       24,
		menu:         dashMenu{items: defaultDashItems()},
		banner:       banner,
		bannerSource: source,
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
	// If the art carries its own ANSI color codes, render it as-is; otherwise
	// paint it Rancher-blue so plain (uncolored) art is always visible.
	banner := strings.Join(m.banner, "\n")
	if !strings.Contains(banner, "\x1b") {
		banner = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true).Render(banner)
	}

	// Uniform width so every item (and the selection highlight) is the same size
	// and centered on one axis.
	menuWidth := 0
	for _, it := range m.menu.items {
		if w := lipgloss.Width(it.label); w > menuWidth {
			menuWidth = w
		}
	}
	menuWidth += 6 // room for the ▶ marker and side padding

	itemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250")).
		Width(menuWidth).Align(lipgloss.Center)
	selStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236")).
		Background(lipgloss.Color("86")).Bold(true).
		Width(menuWidth).Align(lipgloss.Center)

	var menuLines []string
	for i, it := range m.menu.items {
		line := it.label
		if i == m.menu.cursor {
			menuLines = append(menuLines, selStyle.Render("▶ "+line))
		} else {
			menuLines = append(menuLines, itemStyle.Render(line))
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
