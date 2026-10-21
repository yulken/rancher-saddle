package views

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

// ClusterListModel displays all clusters in a table
type ClusterListModel struct {
	table           table.Model
	width           int
	height          int
	clusters        map[string]*config.ClusterConfig
	clusterNames    []string
	selectedCluster string // Currently selected cluster for log viewing
	showLogs        bool   // Whether to show logs

	// autoShownErrors tracks clusters whose logs were auto-opened because they
	// entered the "failed" state, so we don't force the panel back open every
	// refresh after the user hides it. The flag is cleared when the cluster
	// leaves the failed state, so a later failure re-opens the logs.
	autoShownErrors map[string]bool

	// copyNotice is a transient message shown in the log panel after a copy.
	copyNotice string
}

// logsCopiedMsg is returned after the deployment logs are copied to the clipboard.
type logsCopiedMsg struct {
	notice string
}

// refreshTickMsg is sent periodically to auto-refresh the cluster list
type refreshTickMsg struct{}

// NewClusterListModel creates a new cluster list view
func NewClusterListModel() ClusterListModel {
	m := ClusterListModel{
		width:  80,
		height: 20,
	}

	t := table.New(
		table.WithColumns(m.calculateColumns(80)),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m.table = t
	return m
}

// Init initializes the cluster list
func (m ClusterListModel) Init() tea.Cmd {
	return tea.Batch(m.loadClusters(), m.scheduleRefresh())
}

// scheduleRefresh returns a command that sends a refreshTickMsg periodically.
// Uses 1s interval so logs and status updates appear in near real-time.
func (m ClusterListModel) scheduleRefresh() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

// SetSize updates the table dimensions
func (m *ClusterListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4) // Account for borders and padding
	m.table.SetWidth(width - 4)
	m.table.SetColumns(m.calculateColumns(width - 4))
}

// calculateColumns returns table columns sized to fill the given width
func (m *ClusterListModel) calculateColumns(totalWidth int) []table.Column {
	usable := totalWidth - 2
	if usable < 80 {
		usable = 80
	}

	// Fixed minimum widths for short columns, proportional for the rest
	nameW := usable * 16 / 100
	versionW := usable * 10 / 100
	statusW := usable * 12 / 100
	nodesW := usable * 8 / 100
	providerW := usable * 10 / 100
	regionW := usable * 12 / 100
	ageW := usable * 8 / 100
	urlW := usable - nameW - versionW - statusW - nodesW - providerW - regionW - ageW

	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Version", Width: versionW},
		{Title: "Status", Width: statusW},
		{Title: "Nodes", Width: nodesW},
		{Title: "Provider", Width: providerW},
		{Title: "Region", Width: regionW},
		{Title: "Rancher URL", Width: urlW},
		{Title: "Age", Width: ageW},
	}
}

// Update handles messages
func (m ClusterListModel) Update(msg tea.Msg) (ClusterListModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+x":
			// Navigate to credentials management
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateCredentialsList}
			}

		case "ctrl+p":
			// Navigate to profiles management
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateProfilesList}
			}

		case "ctrl+a":
			// Navigate to AMI catalog management
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateAMIsList}
			}

		case "n", "c":
			// Navigate to create form
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateCreateForm}
			}

		case "x", "d":
			// Delete selected cluster
			if len(m.clusterNames) > 0 {
				selectedRow := m.table.Cursor()
				if selectedRow < len(m.clusterNames) {
					clusterName := m.clusterNames[selectedRow]
					return m, func() tea.Msg {
						return StateChangeMsg{
							NewState: StateDeleteConfirm,
							Data:     clusterName,
						}
					}
				}
			}

		case "u":
			// Upgrade Rancher on selected cluster
			if len(m.clusterNames) > 0 {
				selectedRow := m.table.Cursor()
				if selectedRow < len(m.clusterNames) {
					clusterName := m.clusterNames[selectedRow]
					cluster := m.clusters[clusterName]
					if cluster != nil && cluster.Rancher.Deploy && cluster.Status == "running" {
						return m, func() tea.Msg {
							return StateChangeMsg{
								NewState: StateUpgradeForm,
								Data:     clusterName,
							}
						}
					}
				}
			}

		case "r":
			// Manual refresh
			return m, m.loadClusters()

		case "enter":
			// Toggle log viewing for selected cluster
			if len(m.clusterNames) > 0 {
				selectedRow := m.table.Cursor()
				if selectedRow < len(m.clusterNames) {
					clusterName := m.clusterNames[selectedRow]
					if m.showLogs && m.selectedCluster == clusterName {
						// Hide logs if already showing
						m.showLogs = false
						m.selectedCluster = ""
						m.copyNotice = ""
					} else {
						// Show logs for this cluster
						m.showLogs = true
						m.selectedCluster = clusterName
						m.copyNotice = ""
					}
					return m, nil
				}
			}

		case "y":
			// Copy the visible cluster's full deployment log to the clipboard.
			if m.showLogs && m.selectedCluster != "" {
				return m, copyLogsCmd(m.selectedCluster)
			}
		}

	case logsCopiedMsg:
		m.copyNotice = msg.notice
		return m, nil

	case clustersLoadedMsg:
		m.clusters = msg.clusters
		m.clusterNames = msg.names
		m.updateTable()
		m.autoShowFailedLogs()
		return m, nil

	case refreshTickMsg:
		// Auto-refresh: reload clusters and schedule next tick
		return m, tea.Batch(m.loadClusters(), m.scheduleRefresh())
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the cluster list
func (m ClusterListModel) View() string {
	if len(m.clusters) == 0 {
		return m.emptyState()
	}

	return baseStyle.Render(m.table.View())
}

// emptyState shows a message when no clusters exist
func (m ClusterListModel) emptyState() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	message := "No clusters found.\n\nPress 'n' to create a new cluster."
	return emptyStyle.Render(message)
}

// updateTable refreshes the table rows with current cluster data
func (m *ClusterListModel) updateTable() {
	rows := []table.Row{}

	for _, name := range m.clusterNames {
		cluster := m.clusters[name]

		style := lipgloss.NewStyle().Foreground(statusColor(cluster.Status))
		status := formatStatus(cluster.Status)

		// Get region
		region := "-"
		if r, ok := cluster.Provider.Config["region"].(string); ok {
			region = r
		}

		// Rancher URL: show public DNS of first node when cluster is running
		rancherURL := ""
		if cluster.Status == "running" {
			if cluster.RancherURL != "" {
				rancherURL = strings.TrimPrefix(cluster.RancherURL, "https://")
			} else if len(cluster.InstanceDNS) > 0 {
				rancherURL = cluster.InstanceDNS[0]
			}
		}

		age := formatAge(cluster.CreatedAt)

		// Rancher version
		version := "-"
		if cluster.Rancher.Deploy && cluster.Rancher.Version != "" {
			version = cluster.Rancher.Version
		}

		rows = append(rows, table.Row{
			style.Render(name),
			style.Render(version),
			status,
			style.Render(fmt.Sprintf("%d", cluster.Cluster.InstanceCount)),
			style.Render(cluster.Provider.Type),
			style.Render(region),
			style.Render(rancherURL),
			style.Render(age),
		})
	}

	m.table.SetRows(rows)
}

// loadClusters loads clusters from config
func (m ClusterListModel) loadClusters() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadClustersConfig("config.yaml")
		if err != nil {
			return clustersLoadedMsg{
				clusters: make(map[string]*config.ClusterConfig),
				names:    []string{},
			}
		}

		names := cfg.ListClusters()
		return clustersLoadedMsg{
			clusters: cfg.Clusters,
			names:    names,
		}
	}
}

// Message types
type clustersLoadedMsg struct {
	clusters map[string]*config.ClusterConfig
	names    []string
}

// GetSelectedCluster returns the currently selected cluster name and whether logs are shown
func (m ClusterListModel) GetSelectedCluster() (string, bool) {
	return m.selectedCluster, m.showLogs
}

// GetCopyNotice returns the transient post-copy message for the log panel.
func (m ClusterListModel) GetCopyNotice() string {
	return m.copyNotice
}

// copyLogsCmd reads a cluster's full deployment log and copies it to the system
// clipboard via OSC52, returning a status notice for the log panel.
func copyLogsCmd(clusterName string) tea.Cmd {
	return func() tea.Msg {
		path := fmt.Sprintf("logs/%s.log", clusterName)
		data, err := os.ReadFile(path)
		if err != nil {
			return logsCopiedMsg{notice: "no logs to copy"}
		}
		copyToClipboard(string(data))
		lines := strings.Count(strings.TrimRight(string(data), "\n"), "\n") + 1
		return logsCopiedMsg{notice: fmt.Sprintf("copied %d lines to clipboard", lines)}
	}
}

// ShowLogsFor opens the log panel for the named cluster and moves the table
// cursor to it. Used when a cluster is selected from the tree explorer.
func (m *ClusterListModel) ShowLogsFor(name string) {
	m.selectedCluster = name
	m.showLogs = true
	for i, n := range m.clusterNames {
		if n == name {
			m.table.SetCursor(i)
			break
		}
	}
}

// autoShowFailedLogs opens the log panel automatically for a cluster that has
// just entered the "failed" state, so deployment errors are surfaced without the
// user needing to press Enter.
func (m *ClusterListModel) autoShowFailedLogs() {
	if m.autoShownErrors == nil {
		m.autoShownErrors = make(map[string]bool)
	}
	for _, name := range m.clusterNames {
		cluster := m.clusters[name]
		if cluster == nil {
			continue
		}
		if cluster.Status == "failed" {
			if !m.autoShownErrors[name] {
				m.autoShownErrors[name] = true
				m.ShowLogsFor(name)
			}
		} else {
			// Reset so a future failure re-opens the logs.
			delete(m.autoShownErrors, name)
		}
	}
}

// GetCursorClusterError returns the LastError of the cluster currently under the
// table cursor, or "" if no cluster is selected or it has no error stored.
func (m ClusterListModel) GetCursorClusterError() string {
	if len(m.clusterNames) == 0 {
		return ""
	}
	row := m.table.Cursor()
	if row < 0 || row >= len(m.clusterNames) {
		return ""
	}
	cluster := m.clusters[m.clusterNames[row]]
	if cluster == nil {
		return ""
	}
	return cluster.LastError
}
