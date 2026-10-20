package views

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewDashboardModel_Defaults(t *testing.T) {
	m := NewDashboardModel()
	assert.Equal(t, 0, m.menu.cursor, "cursor starts at top")
	assert.NotEmpty(t, m.menu.items, "menu must have items")
}

func TestDashboardMenu_Navigation(t *testing.T) {
	m := NewDashboardModel()
	n := len(m.menu.items)

	tests := []struct {
		name   string
		moves  func(menu *dashMenu)
		cursor int
	}{
		{"down once", func(menu *dashMenu) { menu.moveDown() }, 1},
		{"down twice", func(menu *dashMenu) { menu.moveDown(); menu.moveDown() }, 2},
		{"up wraps to bottom", func(menu *dashMenu) { menu.moveUp() }, n - 1},
		{"down wraps to top", func(menu *dashMenu) {
			for i := 0; i < n; i++ {
				menu.moveDown()
			}
		}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			menu := NewDashboardModel().menu
			tt.moves(&menu)
			assert.Equal(t, tt.cursor, menu.cursor)
		})
	}
}

func TestDashboardMenu_Select(t *testing.T) {
	m := NewDashboardModel()
	// Each menu entry (except quit) should target a reachable AppState.
	for i, it := range m.menu.items {
		t.Run(it.label, func(t *testing.T) {
			menu := m.menu
			menu.cursor = i
			sel := menu.selected()
			assert.Equal(t, it.label, sel.label)
			if !sel.isQuit {
				assert.NotEqual(t, StateDashboard, sel.target, "menu item must leave the dashboard")
			}
		})
	}
}

func TestDashboardModel_EnterEmitsStateChange(t *testing.T) {
	m := NewDashboardModel()
	// Point the cursor at the first non-quit item.
	m.menu.cursor = 0
	target := m.menu.items[0].target

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd, "enter should emit a command")

	msg := cmd()
	sc, ok := msg.(StateChangeMsg)
	assert.True(t, ok, "enter emits a StateChangeMsg")
	assert.Equal(t, target, sc.NewState)
}

func TestAppState_String(t *testing.T) {
	states := []AppState{
		StateClusterList, StateCreateForm, StateDeleteConfirm, StateClusterDetails,
		StateCredentialsList, StateCredentialsForm, StateProfilesList, StateProfilesForm,
		StateAMIsList, StateAMIsForm, StateUpgradeForm, StateHelp, StateQuitting,
		StateDashboard,
	}
	for _, s := range states {
		assert.NotEqual(t, "Unknown", s.String(), "state %d should have a label", int(s))
	}
}
