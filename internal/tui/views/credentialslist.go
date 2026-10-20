package views

import (
	"github.com/Felipalds/rancher-tui/internal/credentials"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CredentialsListModel displays all cloud credentials
type CredentialsListModel struct {
	table       table.Model
	width       int
	height      int
	credentials *credentials.CloudCredentials
	credNames   []string
}

// NewCredentialsListModel creates a new credentials list view
func NewCredentialsListModel() CredentialsListModel {
	columns := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Provider", Width: 12},
		{Title: "Region", Width: 15},
		{Title: "Access Key", Width: 20},
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

	return CredentialsListModel{
		table:  t,
		width:  80,
		height: 20,
	}
}

// Init initializes the credentials list
func (m CredentialsListModel) Init() tea.Cmd {
	return m.loadCredentials()
}

// SetSize updates the table dimensions
func (m *CredentialsListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)
	m.table.SetWidth(width - 4)
}

// Update handles messages
func (m CredentialsListModel) Update(msg tea.Msg) (CredentialsListModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n", "c":
			// Navigate to create credentials form
			return m, func() tea.Msg {
				return StateChangeMsg{
					NewState: StateCredentialsForm,
					Data:     nil, // nil means create new
				}
			}

		case "d":
			// Delete selected credential
			if len(m.credNames) > 0 {
				selectedRow := m.table.Cursor()
				if selectedRow < len(m.credNames) {
					credName := m.credNames[selectedRow]
					// TODO: Add confirmation modal
					return m, m.deleteCredential(credName)
				}
			}

		case "enter":
			// Edit selected credential
			if len(m.credNames) > 0 {
				selectedRow := m.table.Cursor()
				if selectedRow < len(m.credNames) {
					credName := m.credNames[selectedRow]
					return m, func() tea.Msg {
						return StateChangeMsg{
							NewState: StateCredentialsForm,
							Data:     credName, // Pass name for editing
						}
					}
				}
			}

		case "esc":
			// Go back to cluster list
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateClusterList}
			}
		}

	case credentialsLoadedMsg:
		m.credentials = msg.credentials
		m.credNames = msg.names
		m.updateTable()
		return m, nil

	case credentialDeletedMsg:
		// Reload credentials after deletion
		return m, m.loadCredentials()
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the credentials list
func (m CredentialsListModel) View() string {
	if m.credentials == nil || len(m.credNames) == 0 {
		return m.emptyState()
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	title := titleStyle.Render("Cloud Provider Credentials")

	return title + "\n" + baseStyle.Render(m.table.View())
}

// emptyState shows a message when no credentials exist
func (m CredentialsListModel) emptyState() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center)

	message := "No credentials configured.\n\nPress 'n' to add AWS credentials."
	return emptyStyle.Render(message)
}

// updateTable refreshes the table rows with current credentials
func (m *CredentialsListModel) updateTable() {
	rows := []table.Row{}

	for _, name := range m.credNames {
		cred, err := m.credentials.GetAWSCredential(name)
		if err != nil {
			continue
		}

		// Mask access key for security
		maskedKey := maskKey(cred.AccessKey)

		rows = append(rows, table.Row{
			cred.Name,
			"AWS",
			cred.DefaultRegion,
			maskedKey,
		})
	}

	m.table.SetRows(rows)
}

// loadCredentials loads credentials from file
func (m CredentialsListModel) loadCredentials() tea.Cmd {
	return func() tea.Msg {
		creds, err := credentials.LoadCredentials("cloud-credentials.yaml")
		if err != nil {
			return credentialsLoadedMsg{
				credentials: &credentials.CloudCredentials{AWS: []credentials.AWSCredential{}},
				names:       []string{},
			}
		}

		names := creds.ListAWSCredentials()
		return credentialsLoadedMsg{
			credentials: creds,
			names:       names,
		}
	}
}

// deleteCredential deletes a credential
func (m CredentialsListModel) deleteCredential(name string) tea.Cmd {
	return func() tea.Msg {
		creds, err := credentials.LoadCredentials("cloud-credentials.yaml")
		if err != nil {
			return credentialDeletedMsg{err: err}
		}

		if err := creds.DeleteAWSCredential(name); err != nil {
			return credentialDeletedMsg{err: err}
		}

		if err := creds.Save("cloud-credentials.yaml"); err != nil {
			return credentialDeletedMsg{err: err}
		}

		return credentialDeletedMsg{name: name}
	}
}

// maskKey masks an access key for display
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// Message types
type credentialsLoadedMsg struct {
	credentials *credentials.CloudCredentials
	names       []string
}

type credentialDeletedMsg struct {
	name string
	err  error
}
