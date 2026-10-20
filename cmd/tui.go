package cmd

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/model"
	"github.com/Felipalds/rancher-tui/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// RunMenuTUI launches the main menu interface.
// It returns the selected action and any error.
func RunMenuTUI(cfg *model.Config) (tui.MenuAction, error) {
	menuModel := tui.NewMenuModel(cfg)
	p := tea.NewProgram(menuModel)

	finalModel, err := p.Run()
	if err != nil {
		return tui.MenuExit, err
	}

	m, ok := finalModel.(tui.MenuModel)
	if !ok {
		return tui.MenuExit, fmt.Errorf("unexpected model type")
	}

	return m.SelectedAction(), nil
}

// RunTUI launches the interactive terminal interface.
// It returns true if the user submitted the form, false if they aborted.
func RunTUI(cfg *model.Config) (bool, error) {
	initialModel := tui.NewModel(cfg)
	p := tea.NewProgram(initialModel)

	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	m, ok := finalModel.(tui.Model)
	if !ok {
		return false, fmt.Errorf("unexpected model type")
	}

	return m.Done(), nil
}

// RunDeleteMenuTUI shows the delete cluster selection menu.
// It returns the selected cluster name and whether the user canceled.
func RunDeleteMenuTUI() (string, bool, error) {
	deleteModel, err := tui.NewDeleteMenuModel()
	if err != nil {
		return "", false, err
	}

	p := tea.NewProgram(deleteModel)

	finalModel, err := p.Run()
	if err != nil {
		return "", false, err
	}

	m, ok := finalModel.(tui.DeleteMenuModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model type")
	}

	if m.Canceled() {
		return "", true, nil
	}

	return m.SelectedCluster(), false, nil
}
