package views

import (
	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AMIsListModel displays all AMI entries from amis.yaml.
type AMIsListModel struct {
	table   table.Model
	width   int
	height  int
	amis    *config.AMIsConfig
	entries []config.AMIEntry // flat ordered list matching table rows
}

// NewAMIsListModel creates a new AMI list view.
func NewAMIsListModel() AMIsListModel {
	columns := []table.Column{
		{Title: "Distro", Width: 22},
		{Title: "Region", Width: 18},
		{Title: "AMI ID", Width: 25},
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

	return AMIsListModel{
		table:  t,
		width:  80,
		height: 20,
	}
}

// Init loads amis.yaml.
func (m AMIsListModel) Init() tea.Cmd {
	return m.loadAMIs()
}

// SetSize updates the table dimensions.
func (m *AMIsListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)
	m.table.SetWidth(width - 4)
}

// Update handles messages.
func (m AMIsListModel) Update(msg tea.Msg) (AMIsListModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n", "c":
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateAMIsForm, Data: nil}
			}

		case "enter":
			if len(m.entries) > 0 {
				row := m.table.Cursor()
				if row < len(m.entries) {
					e := m.entries[row]
					return m, func() tea.Msg {
						return StateChangeMsg{
							NewState: StateAMIsForm,
							Data:     e.Distro + "|" + e.Region,
						}
					}
				}
			}

		case "d":
			if len(m.entries) > 0 {
				row := m.table.Cursor()
				if row < len(m.entries) {
					e := m.entries[row]
					return m, m.deleteEntry(e.Distro, e.Region)
				}
			}

		case "esc":
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateClusterList}
			}
		}

	case amisLoadedMsg:
		m.amis = msg.amis
		m.entries = msg.entries
		m.updateTable()
		return m, nil

	case amiDeletedMsg:
		return m, m.loadAMIs()
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the AMI list.
func (m AMIsListModel) View() string {
	if m.amis == nil || len(m.entries) == 0 {
		return m.emptyState()
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	title := titleStyle.Render("AMI Catalog  (amis.yaml)")

	return title + "\n" + baseStyle.Render(m.table.View())
}

func (m AMIsListModel) emptyState() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	return emptyStyle.Render("No AMI entries found.\n\nPress 'n' to add a new entry.")
}

func (m *AMIsListModel) updateTable() {
	rows := []table.Row{}
	for _, e := range m.entries {
		rows = append(rows, table.Row{e.Distro, e.Region, e.AMIID})
	}
	m.table.SetRows(rows)
}

func (m AMIsListModel) loadAMIs() tea.Cmd {
	return func() tea.Msg {
		amis, err := config.LoadAMIs("amis.yaml")
		if err != nil {
			return amisLoadedMsg{
				amis:    &config.AMIsConfig{},
				entries: nil,
			}
		}
		return amisLoadedMsg{amis: amis, entries: amis.AMIs}
	}
}

func (m AMIsListModel) deleteEntry(distro, region string) tea.Cmd {
	return func() tea.Msg {
		amis, err := config.LoadAMIs("amis.yaml")
		if err != nil {
			return amiDeletedMsg{err: err}
		}
		if err := amis.DeleteEntry(distro, region); err != nil {
			return amiDeletedMsg{err: err}
		}
		if err := amis.Save("amis.yaml"); err != nil {
			return amiDeletedMsg{err: err}
		}
		return amiDeletedMsg{}
	}
}

// Message types
type amisLoadedMsg struct {
	amis    *config.AMIsConfig
	entries []config.AMIEntry
}

type amiDeletedMsg struct {
	err error
}
