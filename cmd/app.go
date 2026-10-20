package cmd

import (
	"fmt"
	"os"

	"github.com/Felipalds/rancher-tui/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// RunApp launches the fullscreen TUI application
func RunApp() error {
	// Create the root model
	m := tui.NewRootModel()

	// Create the program with alt screen
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// LaunchTUI is a wrapper for RunApp for backwards compatibility
func LaunchTUI() {
	if err := RunApp(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
