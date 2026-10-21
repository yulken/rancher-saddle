package views

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/providers/libvirt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DeleteModalModel represents the delete confirmation modal
type DeleteModalModel struct {
	width       int
	height      int
	clusterName string
}

// NewDeleteModalModel creates a new delete modal
func NewDeleteModalModel() DeleteModalModel {
	return DeleteModalModel{
		width:  80,
		height: 20,
	}
}

// SetSize updates the modal dimensions
func (m *DeleteModalModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetCluster sets the cluster to be deleted
func (m *DeleteModalModel) SetCluster(name string) {
	m.clusterName = name
}

// Update handles messages
func (m DeleteModalModel) Update(msg tea.Msg) (DeleteModalModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			clusterName := m.clusterName
			return m, func() tea.Msg {
				// Mark as deleting immediately
				cfg, err := config.LoadClustersConfig("config.yaml")
				if err == nil {
					if cluster, exists := cfg.GetCluster(clusterName); exists {
						cluster.Status = "deleting"
						cluster.LastError = ""
						cfg.AddCluster(clusterName, cluster)
						cfg.Save("config.yaml")
					}
				}

				// Run destruction in background so TUI stays responsive
				go destroyCluster(clusterName)

				return ClusterDeletedMsg{ClusterName: clusterName}
			}

		case "n", "esc":
			// Cancel deletion
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateClusterList}
			}
		}
	}

	return m, nil
}

// ViewOver renders the modal over existing content
func (m DeleteModalModel) ViewOver(content string) string {
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(50).
		Background(lipgloss.Color("235"))

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Render("⚠ Delete Cluster")

	message := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Render("Are you sure you want to delete:\n\n  " + m.clusterName + "\n\nThis action cannot be undone.")

	actions := lipgloss.NewStyle().
		Faint(true).
		Render("\n[y] Confirm  [n] Cancel")

	modal := modalStyle.Render(title + "\n\n" + message + actions)

	// Overlay on top of content
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("0")),
	)
}

// destroyCluster runs infrastructure destruction and config cleanup in the background
func destroyCluster(name string) {
	// Set up logging
	logPath := fmt.Sprintf("logs/%s.log", name)
	os.MkdirAll("logs", 0755)
	logFile, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	writeLog := func(message string) {
		if logFile != nil {
			fmt.Fprintf(logFile, "[%s] %s\n", getTimestamp(), message)
			logFile.Sync()
		}
	}

	writeLog(fmt.Sprintf("=== Starting deletion for cluster: %s ===", name))

	// Load cluster config to get build directory
	cfg, err := config.LoadClustersConfig("config.yaml")
	if err != nil {
		writeLog(fmt.Sprintf("ERROR: Failed to load config: %v", err))
		return
	}

	cluster, exists := cfg.GetCluster(name)
	if !exists {
		writeLog("Cluster not found in config, nothing to destroy")
		return
	}

	buildDir := cluster.BuildDir
	if buildDir == "" {
		buildDir = filepath.Join("clusters", name)
	}

	// Local libvirt clusters are torn down with virsh, not OpenTofu.
	// "vagrant" is the legacy id for clusters created before the libvirt rename.
	if cluster.Provider.Type == string(core.ProviderLibvirt) || cluster.Provider.Type == "vagrant" {
		writeLog("Tearing down libvirt VMs, network and disks...")
		var w io.Writer = io.Discard
		if logFile != nil {
			w = logFile
		}
		if err := libvirt.Teardown(context.Background(), name, buildDir, w); err != nil {
			writeLog(fmt.Sprintf("Warning: libvirt teardown reported: %v", err))
		}
		if err := os.RemoveAll(buildDir); err != nil {
			writeLog(fmt.Sprintf("Warning: failed to remove build directory: %v", err))
		}
		writeLog("Removing cluster from config.yaml...")
		cfg, _ = config.LoadClustersConfig("config.yaml")
		cfg.DeleteCluster(name)
		cfg.Save("config.yaml")
		writeLog(fmt.Sprintf("Cluster '%s' deleted successfully", name))
		if logFile != nil {
			logFile.Close()
		}
		return
	}

	// Destroy infrastructure with tofu
	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		writeLog("Destroying infrastructure with tofu destroy...")
		cmd := exec.Command("tofu", "destroy", "-auto-approve")
		cmd.Dir = buildDir

		// Capture stderr to a buffer so we can surface a short error in the TUI,
		// while also tee'ing into the log file for the full record.
		var stderrBuf bytes.Buffer
		if logFile != nil {
			cmd.Stdout = logFile
			cmd.Stderr = io.MultiWriter(logFile, &stderrBuf)
		} else {
			cmd.Stderr = &stderrBuf
		}

		if err := cmd.Run(); err != nil {
			msg := summarizeDestroyError(err, stderrBuf.String())
			writeLog(fmt.Sprintf("Warning: tofu destroy failed: %v", err))
			writeLog("You may need to manually clean up cloud resources.")
			updateClusterStatusWithError(name, "delete-failed", msg)
			if logFile != nil {
				logFile.Close()
			}
			return
		}

		// Remove build directory
		writeLog("Removing build directory...")
		if err := os.RemoveAll(buildDir); err != nil {
			writeLog(fmt.Sprintf("Warning: failed to remove build directory: %v", err))
		}
	} else {
		writeLog("No build directory found, skipping infrastructure destruction")
	}

	// Remove cluster from config
	writeLog("Removing cluster from config.yaml...")
	cfg, _ = config.LoadClustersConfig("config.yaml")
	cfg.DeleteCluster(name)
	cfg.Save("config.yaml")

	writeLog(fmt.Sprintf("Cluster '%s' deleted successfully", name))
	if logFile != nil {
		logFile.Close()
	}
}

// summarizeDestroyError produces a short, user-facing message from a tofu
// destroy failure. It special-cases common environment problems (missing or
// wrong tofu binary) so the TUI shows actionable text instead of a raw stack.
func summarizeDestroyError(err error, stderr string) string {
	errStr := err.Error()
	if strings.Contains(errStr, "executable file not found") {
		return "OpenTofu binary 'tofu' not found in PATH. Install opentofu and retry."
	}

	// Known imposter on some Arch systems: a TOTP Manager package ships /usr/bin/tofu.
	lower := strings.ToLower(stderr)
	if strings.Contains(lower, "totp manager") {
		return "'tofu' in PATH is a TOTP Manager, not OpenTofu. Remove the 'tofu' package and install opentofu."
	}

	trimmed := strings.TrimSpace(stderr)
	if trimmed != "" {
		return firstNonEmptyLine(trimmed, 200)
	}
	return errStr
}

// firstNonEmptyLine returns the first non-empty line of s, truncated to max chars.
func firstNonEmptyLine(s string, max int) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > max {
			return line[:max] + "…"
		}
		return line
	}
	return ""
}

// updateClusterStatusWithError sets both status and LastError atomically.
func updateClusterStatusWithError(name, status, errMsg string) {
	cfg, err := config.LoadClustersConfig("config.yaml")
	if err != nil {
		return
	}
	cluster, exists := cfg.GetCluster(name)
	if !exists {
		return
	}
	cluster.Status = status
	cluster.LastError = errMsg
	cfg.AddCluster(name, cluster)
	cfg.Save("config.yaml")
}
