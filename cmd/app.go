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

	// Create the program with alt screen. We deliberately do NOT enable mouse
	// capture (tea.WithMouseCellMotion): capturing the mouse hands click/drag
	// events to the app and prevents the terminal's own text selection, so
	// users couldn't select/copy the log panel with the mouse. With capture
	// off, native click-drag selection works like in Neovim/Claude TUIs (the
	// 'y' key still copies the full log via OSC52 for streaming logs).
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
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
