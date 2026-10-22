package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/credentials"
	provaws "github.com/Felipalds/rancher-tui/internal/providers/aws"
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
	statuses    map[string]string
	// expiries holds the cached SSO access-token expiry per credential name.
	// They come from ~/.aws/sso/cache, so rendering them costs no API call.
	expiries map[string]time.Time
	notice   string
}

// NewCredentialsListModel creates a new credentials list view
func NewCredentialsListModel() CredentialsListModel {
	columns := []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Provider", Width: 10},
		{Title: "Region", Width: 14},
		{Title: "Status", Width: 28},
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
		table:    t,
		width:    80,
		height:   20,
		statuses: map[string]string{},
		expiries: map[string]time.Time{},
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

		case "r":
			// Refresh from disk and re-probe the sessions. Both are needed when
			// the credentials file or the SSO token cache changed outside the
			// TUI (e.g. `aws sso login` in another terminal): nothing tells this
			// screen about it, so it would otherwise keep the statuses it
			// gathered when it was opened.
			m.statuses = map[string]string{}
			m.expiries = map[string]time.Time{}
			m.notice = refreshingNotice
			m.updateTable()
			return m, m.loadCredentials()

		case "d":
			// Delete selected credential
			if row := m.table.Cursor(); row >= 0 && row < len(m.credNames) {
				credName := m.credNames[row]
				// TODO: Add confirmation modal
				return m, m.deleteCredential(credName)
			}

		case "enter":
			// Edit selected credential
			if row := m.table.Cursor(); row >= 0 && row < len(m.credNames) {
				credName := m.credNames[row]
				return m, func() tea.Msg {
					return StateChangeMsg{
						NewState: StateCredentialsForm,
						Data:     credName, // Pass name for editing
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
		if msg.imported > 0 {
			m.notice = fmt.Sprintf("Imported %d SSO profile(s) from %s", msg.imported, provaws.DefaultAWSConfigPath())
		}
		m.updateTable()
		statusCmd := m.refreshStatuses()
		// Without SSO credentials there is no probe to wait for, so the
		// refresh notice has to be cleared here.
		if statusCmd == nil && m.notice == refreshingNotice {
			m.notice = ""
		}
		return m, statusCmd

	case ssoStatusMsg:
		for name, status := range msg.statuses {
			m.statuses[name] = status
		}
		for name, expiry := range msg.expiries {
			m.expiries[name] = expiry
		}
		if m.notice == refreshingNotice {
			m.notice = ""
		}
		m.updateTable()
		return m, nil

	case credentialDeletedMsg:
		// Reload credentials after deletion
		delete(m.statuses, msg.name)
		return m, m.loadCredentials()
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// selected returns the credential under the cursor.
func (m CredentialsListModel) selected() *credentials.AWSCredential {
	if m.credentials == nil {
		return nil
	}
	row := m.table.Cursor()
	if row < 0 || row >= len(m.credNames) {
		return nil
	}
	cred, err := m.credentials.GetAWSCredential(m.credNames[row])
	if err != nil {
		return nil
	}
	return cred
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

	view := title + "\n" + baseStyle.Render(m.table.View())

	if detail := m.detailLine(); detail != "" {
		view += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  "+detail)
	}

	if m.notice != "" {
		view += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  "+m.notice)
	}

	return view
}

// detailLine describes the selected credential. For SSO it includes how long
// the cached access token is still valid, which is the number users ask for
// after "am I logged in?".
func (m CredentialsListModel) detailLine() string {
	cred := m.selected()
	if cred == nil || cred.AuthType() != core.AuthSSO {
		return ""
	}

	region := cred.SSORegion
	if region == "" {
		region = "-"
	}

	parts := []string{
		fmt.Sprintf("profile %s", cred.Profile),
		fmt.Sprintf("identity center %s", region),
	}

	if expiry, ok := m.expiries[cred.Name]; ok {
		parts = append(parts, "token "+formatTokenRemaining(expiry))
	} else {
		parts = append(parts, fmt.Sprintf("no cached token (run: aws sso login --profile %s)", cred.Profile))
	}

	return strings.Join(parts, " · ")
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

		provider := "AWS"
		status := "keys"
		if cred.AuthType() == core.AuthSSO {
			provider = "AWS SSO"
			status = m.statuses[cred.Name]
			if status == "" {
				status = "checking…"
			}
		}

		rows = append(rows, table.Row{
			cred.Name,
			provider,
			cred.DefaultRegion,
			status,
		})
	}

	setTableRows(&m.table, rows)
}

// refreshStatuses probes every SSO profile in the background.
func (m CredentialsListModel) refreshStatuses() tea.Cmd {
	if m.credentials == nil {
		return nil
	}
	ssos := make([]credentials.AWSCredential, 0, len(m.credentials.AWS))
	for _, cred := range m.credentials.AWS {
		if cred.AuthType() == core.AuthSSO {
			ssos = append(ssos, cred)
		}
	}
	if len(ssos) == 0 {
		return nil
	}

	return func() tea.Msg {
		// Profiles are read once; the token cache lookup below is a local file
		// read, so it adds no API call on top of the STS probe.
		profiles, _ := provaws.ListSSOProfiles()
		byName := make(map[string]provaws.SSOProfile, len(profiles))
		for _, p := range profiles {
			byName[p.Name] = p
		}

		statuses := make(map[string]string, len(ssos))
		expiries := make(map[string]time.Time, len(ssos))
		for _, cred := range ssos {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			statuses[cred.Name] = provaws.DescribeSession(ctx, cred.Profile)
			cancel()

			if profile, ok := byName[cred.Profile]; ok {
				if expiry, found := provaws.SSOTokenExpiry(profile); found {
					expiries[cred.Name] = expiry
				}
			}
		}
		return ssoStatusMsg{statuses: statuses, expiries: expiries}
	}
}

// formatTokenRemaining renders how long is left until the cached SSO access
// token expires, phrased to follow the word "token" in the detail line.
func formatTokenRemaining(exp time.Time) string {
	return formatRemaining(time.Until(exp))
}

func formatRemaining(d time.Duration) string {
	switch {
	case d <= 0:
		return "expired"
	case d < time.Minute:
		return "expires in less than a minute"
	case d < time.Hour:
		return fmt.Sprintf("expires in %dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("expires in %dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// loadCredentials loads credentials from file, importing SSO profiles from
// ~/.aws/config on the first run.
func (m CredentialsListModel) loadCredentials() tea.Cmd {
	return func() tea.Msg {
		creds, imported, err := loadCredentialsWithDiscovery(credentialsPath)
		if err != nil || creds == nil {
			return credentialsLoadedMsg{
				credentials: &credentials.CloudCredentials{AWS: []credentials.AWSCredential{}},
				names:       []string{},
			}
		}

		return credentialsLoadedMsg{
			credentials: creds,
			names:       creds.ListAWSCredentials(),
			imported:    imported,
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

// Message types
type credentialsLoadedMsg struct {
	credentials *credentials.CloudCredentials
	names       []string
	imported    int
}

type credentialDeletedMsg struct {
	name string
	err  error
}

// refreshingNotice is shown while a user-triggered refresh is in flight. The
// message that completes the refresh clears it.
const refreshingNotice = "Refreshing..."

type ssoStatusMsg struct {
	statuses map[string]string
	expiries map[string]time.Time
}
