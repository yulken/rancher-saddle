package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TreeWidth is the fixed width (including borders) of the explorer pane.
const TreeWidth = 32

// treeTickMsg drives the tree's own periodic refresh while it is visible.
type treeTickMsg struct{}

// ClusterTreeModel is a Neovim-tree-style explorer listing clusters. Each
// cluster node can be expanded to reveal details (status, provider, nodes, URL).
type ClusterTreeModel struct {
	width    int
	height   int
	clusters map[string]*config.ClusterConfig
	names    []string
	cursor   int
	expanded map[string]bool
	focused  bool
}

// NewClusterTreeModel creates a new explorer pane.
func NewClusterTreeModel() ClusterTreeModel {
	return ClusterTreeModel{
		width:    TreeWidth,
		height:   20,
		expanded: make(map[string]bool),
	}
}

// Init loads clusters and starts the refresh tick.
func (m ClusterTreeModel) Init() tea.Cmd {
	return tea.Batch(loadClustersCmd(), m.tick())
}

func (m ClusterTreeModel) tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return treeTickMsg{} })
}

// SetSize updates the pane dimensions.
func (m *ClusterTreeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused marks the pane focused (affects border + key routing).
func (m *ClusterTreeModel) SetFocused(f bool) { m.focused = f }

// SelectedName returns the cluster under the cursor, or "".
func (m ClusterTreeModel) SelectedName() string {
	if m.cursor >= 0 && m.cursor < len(m.names) {
		return m.names[m.cursor]
	}
	return ""
}

// Update handles tree navigation and refresh messages.
func (m ClusterTreeModel) Update(msg tea.Msg) (ClusterTreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.names)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
		case "G":
			if len(m.names) > 0 {
				m.cursor = len(m.names) - 1
			}
		case "l", "right":
			if n := m.SelectedName(); n != "" {
				m.expanded[n] = true
			}
		case "h", "left":
			if n := m.SelectedName(); n != "" {
				m.expanded[n] = false
			}
		case "enter", "o":
			if n := m.SelectedName(); n != "" {
				name := n
				return m, func() tea.Msg { return OpenClusterLogsMsg{ClusterName: name} }
			}
		}
		return m, nil

	case treeTickMsg:
		return m, tea.Batch(loadClustersCmd(), m.tick())

	case clustersLoadedMsg:
		m.clusters = msg.clusters
		m.names = msg.names
		if m.cursor >= len(m.names) {
			m.cursor = len(m.names) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil
	}
	return m, nil
}

// View renders the explorer pane.
func (m ClusterTreeModel) View() string {
	borderColor := lipgloss.Color("240")
	if m.focused {
		borderColor = lipgloss.Color("86")
	}
	paneStyle := lipgloss.NewStyle().
		Width(m.width-2).
		Height(m.height-2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("EXPLORER") + "\n")
	b.WriteString(dimStyle.Render("clusters/") + "\n")

	if len(m.names) == 0 {
		b.WriteString(dimStyle.Render("  (no clusters)"))
		return paneStyle.Render(b.String())
	}

	// Width available for a node's text inside the pane.
	inner := m.width - 4
	if inner < 8 {
		inner = 8
	}

	for i, name := range m.names {
		cluster := m.clusters[name]
		arrow := "▸"
		if m.expanded[name] {
			arrow = "▾"
		}

		dot := "●"
		dotColored := dot
		label := name
		if cluster != nil {
			dotColored = lipgloss.NewStyle().Foreground(statusColor(cluster.Status)).Render(dot)
		}

		text := fmt.Sprintf("%s %s", arrow, label)
		text = truncate(text, inner-2)
		line := fmt.Sprintf("%s %s", text, dotColored)
		if i == m.cursor {
			line = cursorStyle.Render(fmt.Sprintf("%s %s", text, dot))
		}
		b.WriteString(line + "\n")

		if m.expanded[name] && cluster != nil {
			for _, d := range clusterDetailLines(cluster) {
				b.WriteString(detailStyle.Render("    "+truncate(d, inner-4)) + "\n")
			}
		}
	}

	return paneStyle.Render(b.String())
}

// clusterDetailLines returns the child detail rows shown under an expanded node.
func clusterDetailLines(c *config.ClusterConfig) []string {
	lines := []string{
		"status: " + c.Status,
		"provider: " + c.Provider.Type,
		"distro: " + c.Kubernetes.Distribution,
		fmt.Sprintf("nodes: %d", c.Cluster.InstanceCount),
	}
	if c.RancherURL != "" {
		lines = append(lines, "url: "+strings.TrimPrefix(c.RancherURL, "https://"))
	}
	if c.Status == "failed" && c.LastError != "" {
		lines = append(lines, "error: "+c.LastError)
	}
	return lines
}

// truncate shortens s to max runes, adding an ellipsis when cut.
func truncate(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// loadClustersCmd loads clusters from config.yaml and returns clustersLoadedMsg.
func loadClustersCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadClustersConfig("config.yaml")
		if err != nil {
			return clustersLoadedMsg{
				clusters: make(map[string]*config.ClusterConfig),
				names:    []string{},
			}
		}
		return clustersLoadedMsg{clusters: cfg.Clusters, names: cfg.ListClusters()}
	}
}
