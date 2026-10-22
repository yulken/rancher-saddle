package views

import (
	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProfilesListModel displays all saved profiles
type ProfilesListModel struct {
	table        table.Model
	width        int
	height       int
	profiles     *config.ProfilesConfig
	profileNames []string
}

// NewProfilesListModel creates a new profiles list view
func NewProfilesListModel() ProfilesListModel {
	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Region", Width: 15},
		{Title: "Instance Type", Width: 15},
		{Title: "AMI", Width: 25},
	}

	t := table.New(
		table.WithColumns(columns),
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

	return ProfilesListModel{
		table:  t,
		width:  80,
		height: 20,
	}
}

// Init initializes the profiles list
func (m ProfilesListModel) Init() tea.Cmd {
	return m.loadProfiles()
}

// SetSize updates the table dimensions
func (m *ProfilesListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)
	m.table.SetWidth(width - 4)
}

// Update handles messages
func (m ProfilesListModel) Update(msg tea.Msg) (ProfilesListModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n", "c":
			// Create new profile
			return m, func() tea.Msg {
				return StateChangeMsg{
					NewState: StateProfilesForm,
					Data:     nil,
				}
			}

		case "d":
			// Delete selected profile
			if row := m.table.Cursor(); row >= 0 && row < len(m.profileNames) {
				profileName := m.profileNames[row]
				return m, m.deleteProfile(profileName)
			}

		case "enter":
			// Edit selected profile
			if row := m.table.Cursor(); row >= 0 && row < len(m.profileNames) {
				profileName := m.profileNames[row]
				return m, func() tea.Msg {
					return StateChangeMsg{
						NewState: StateProfilesForm,
						Data:     profileName,
					}
				}
			}

		case "esc":
			// Go back to cluster list
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateClusterList}
			}
		}

	case profilesLoadedMsg:
		m.profiles = msg.profiles
		m.profileNames = msg.names
		m.updateTable()
		return m, nil

	case profileDeletedMsg:
		// Reload profiles after deletion
		return m, m.loadProfiles()
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the profiles list
func (m ProfilesListModel) View() string {
	if m.profiles == nil || len(m.profileNames) == 0 {
		return m.emptyState()
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	title := titleStyle.Render("Configuration Profiles")

	return title + "\n" + baseStyle.Render(m.table.View())
}

// emptyState shows a message when no profiles exist
func (m ProfilesListModel) emptyState() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	message := "No profiles configured.\n\nPress 'n' to create a default configuration profile."
	return emptyStyle.Render(message)
}

// updateTable refreshes the table rows
func (m *ProfilesListModel) updateTable() {
	rows := []table.Row{}

	for _, name := range m.profileNames {
		profile, err := m.profiles.GetProfile(name)
		if err != nil {
			continue
		}

		rows = append(rows, table.Row{
			name,
			profile.Region,
			profile.InstanceType,
			profile.AMI,
		})
	}

	setTableRows(&m.table, rows)
}

// loadProfiles loads profiles from file
func (m ProfilesListModel) loadProfiles() tea.Cmd {
	return func() tea.Msg {
		profiles, err := config.LoadProfiles("profiles.yaml")
		if err != nil {
			return profilesLoadedMsg{
				profiles: &config.ProfilesConfig{Profiles: make(map[string]*config.Profile)},
				names:    []string{},
			}
		}

		names := profiles.ListProfiles()
		return profilesLoadedMsg{
			profiles: profiles,
			names:    names,
		}
	}
}

// deleteProfile deletes a profile
func (m ProfilesListModel) deleteProfile(name string) tea.Cmd {
	return func() tea.Msg {
		profiles, err := config.LoadProfiles("profiles.yaml")
		if err != nil {
			return profileDeletedMsg{err: err}
		}

		if err := profiles.DeleteProfile(name); err != nil {
			return profileDeletedMsg{err: err}
		}

		if err := profiles.Save("profiles.yaml"); err != nil {
			return profileDeletedMsg{err: err}
		}

		return profileDeletedMsg{name: name}
	}
}

// Message types
type profilesLoadedMsg struct {
	profiles *config.ProfilesConfig
	names    []string
}

type profileDeletedMsg struct {
	name string
	err  error
}
