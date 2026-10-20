package tui

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/lipgloss"
)

const appVersion = "v0.5.0"

// HeaderModel represents the persistent header
type HeaderModel struct {
	width        int
	clusterCount int
	connected    bool
}

// NewHeaderModel creates a new header
func NewHeaderModel() HeaderModel {
	h := HeaderModel{
		width:     80,
		connected: true,
	}
	h.UpdateClusterCount()
	return h
}

// SetWidth updates the header width
func (h *HeaderModel) SetWidth(width int) {
	h.width = width
}

// UpdateClusterCount refreshes the cluster count
func (h *HeaderModel) UpdateClusterCount() {
	cfg, err := config.LoadClustersConfig("config.yaml")
	if err != nil {
		h.clusterCount = 0
		h.connected = false
		return
	}
	h.clusterCount = len(cfg.Clusters)
	h.connected = true
}

// View renders the header
func (h HeaderModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Background(lipgloss.Color("235"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Background(lipgloss.Color("235")).
		Padding(0, 1)

	if !h.connected {
		statusStyle = statusStyle.Foreground(lipgloss.Color("196"))
	}

	headerStyle := lipgloss.NewStyle().
		Width(h.width).
		Background(lipgloss.Color("235")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("62")).
		BorderBottom(true)

	// Build header content
	title := titleStyle.Render("🚀 rancher-tui")
	version := versionStyle.Render(" " + appVersion)

	var statusIcon string
	var statusText string
	if h.connected {
		statusIcon = "●"
		statusText = fmt.Sprintf(" %s Connected • %d clusters", statusIcon, h.clusterCount)
	} else {
		statusIcon = "○"
		statusText = fmt.Sprintf(" %s Disconnected", statusIcon)
	}
	status := statusStyle.Render(statusText)

	// Calculate spacing
	titleLen := lipgloss.Width(title) + lipgloss.Width(version)
	statusLen := lipgloss.Width(status)
	spacingLen := h.width - titleLen - statusLen - 2

	spacing := ""
	if spacingLen > 0 {
		spacing = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Render(lipgloss.PlaceHorizontal(spacingLen, lipgloss.Left, ""))
	}

	content := title + version + spacing + status

	return headerStyle.Render(content)
}
