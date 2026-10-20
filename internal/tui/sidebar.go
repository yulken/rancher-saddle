package tui

import (
	"fmt"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 25

// SidebarModel represents the left sidebar navigation
type SidebarModel struct {
	activeView   int // 1=Clusters, 2=Create, 3=Delete
	clusterCount int
	connected    bool
}

// NewSidebarModel creates a new sidebar
func NewSidebarModel() SidebarModel {
	return SidebarModel{
		activeView:   1,
		clusterCount: 0,
		connected:    true,
	}
}

// SetActiveView updates which view is currently active
func (s *SidebarModel) SetActiveView(view int) {
	s.activeView = view
}

// UpdateClusterCount refreshes the cluster count from config
func (s *SidebarModel) UpdateClusterCount() error {
	cfg, err := config.LoadClustersConfig("config.yaml")
	if err != nil {
		s.clusterCount = 0
		return err
	}
	s.clusterCount = len(cfg.Clusters)
	return nil
}

// View renders the sidebar
func (s SidebarModel) View() string {
	var b strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(sidebarWidth - 2)

	statusGoodStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("82"))

	statusLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(sidebarWidth - 2)

	menuItemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Width(sidebarWidth - 2).
		PaddingLeft(1)

	activeMenuStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Background(lipgloss.Color("235")).
		Width(sidebarWidth - 2).
		PaddingLeft(1)

	dimMenuStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(sidebarWidth - 2).
		PaddingLeft(1)

	// Title
	b.WriteString(titleStyle.Render("🚀 K8s Helper") + "\n")
	b.WriteString(strings.Repeat(" ", sidebarWidth-2) + "\n")

	// Status section
	statusIcon := "●"
	if s.connected {
		b.WriteString(statusGoodStyle.Render("Status: "+statusIcon+" Connected") + "\n")
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Status: ○ Disconnected") + "\n")
	}

	b.WriteString(statusLabelStyle.Render(fmt.Sprintf("Clusters: %d", s.clusterCount)) + "\n")
	b.WriteString(strings.Repeat(" ", sidebarWidth-2) + "\n")

	// Navigation menu
	menuItems := []struct {
		key   string
		label string
		view  int
	}{
		{"1", "Clusters", 1},
		{"2", "Create", 2},
		{"3", "Delete", 3},
	}

	for _, item := range menuItems {
		line := fmt.Sprintf("[%s] %s", item.key, item.label)
		if s.activeView == item.view {
			b.WriteString(activeMenuStyle.Render(line) + "\n")
		} else {
			b.WriteString(menuItemStyle.Render(line) + "\n")
		}
	}

	b.WriteString(strings.Repeat(" ", sidebarWidth-2) + "\n")
	b.WriteString(dimMenuStyle.Render("[?] Help") + "\n")
	b.WriteString(dimMenuStyle.Render("[q] Quit") + "\n")

	return b.String()
}

// GetWidth returns the width of the sidebar
func (s SidebarModel) GetWidth() int {
	return sidebarWidth
}
