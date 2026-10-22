package views

import (
	"fmt"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/credentials"
	provaws "github.com/Felipalds/rancher-tui/internal/providers/aws"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Form fields, in display order. Some are hidden depending on the auth type.
const (
	credFieldType = iota
	credFieldName
	credFieldAccessKey
	credFieldSecretKey
	credFieldProfile
	credFieldRegion
	credFieldSave
)

// Text input indices. The type selector is not a text input.
const (
	inputName = iota
	inputAccessKey
	inputSecretKey
	inputProfile
	inputRegion
	inputCount
)

// CredentialsFormModel represents the AWS credentials form
type CredentialsFormModel struct {
	width       int
	height      int
	inputs      []textinput.Model
	authType    string
	focusIndex  int  // index into visibleFields()
	editMode    bool // true if editing existing credential
	credName    string
	returnTo    AppState // State to return to after saving
	notice      string
	ssoProfiles []provaws.SSOProfile
}

// NewCredentialsFormModel creates a new credentials form
func NewCredentialsFormModel() CredentialsFormModel {
	m := CredentialsFormModel{
		width:      80,
		height:     20,
		inputs:     make([]textinput.Model, inputCount),
		focusIndex: 1, // start on the name; type is pre-selected as static
		authType:   core.AuthStatic,
		editMode:   false,
		returnTo:   StateCredentialsList, // Default return to credentials list
	}

	// Input 0: Credential Name
	m.inputs[inputName] = textinput.New()
	m.inputs[inputName].Placeholder = "my-aws-account"
	m.inputs[inputName].CharLimit = 50
	m.inputs[inputName].Width = 40
	m.inputs[inputName].Focus()

	// Input 1: AWS Access Key (static only)
	m.inputs[inputAccessKey] = textinput.New()
	m.inputs[inputAccessKey].Placeholder = "AKIA..."
	m.inputs[inputAccessKey].CharLimit = 50
	m.inputs[inputAccessKey].Width = 40

	// Input 2: AWS Secret Key (static only)
	m.inputs[inputSecretKey] = textinput.New()
	m.inputs[inputSecretKey].Placeholder = "Secret key..."
	m.inputs[inputSecretKey].EchoMode = textinput.EchoPassword
	m.inputs[inputSecretKey].EchoCharacter = '•'
	m.inputs[inputSecretKey].CharLimit = 100
	m.inputs[inputSecretKey].Width = 40

	// Input 3: SSO Profile (sso only)
	m.inputs[inputProfile] = textinput.New()
	m.inputs[inputProfile].Placeholder = "acme-prod"
	m.inputs[inputProfile].CharLimit = 80
	m.inputs[inputProfile].Width = 40

	// Input 4: Default Region
	m.inputs[inputRegion] = textinput.New()
	m.inputs[inputRegion].Placeholder = "us-east-1"
	m.inputs[inputRegion].CharLimit = 20
	m.inputs[inputRegion].Width = 40

	return m
}

// Init initializes the form
func (m CredentialsFormModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.loadSSOProfiles())
}

// SetSize updates the form dimensions
func (m *CredentialsFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetEditMode sets the form to edit an existing credential
func (m *CredentialsFormModel) SetEditMode(credName string) tea.Cmd {
	m.editMode = true
	m.credName = credName

	return tea.Batch(
		func() tea.Msg {
			creds, err := credentials.LoadCredentials("cloud-credentials.yaml")
			if err != nil {
				return credentialLoadErrorMsg{err: err}
			}

			cred, err := creds.GetAWSCredential(credName)
			if err != nil {
				return credentialLoadErrorMsg{err: err}
			}

			return credentialLoadedForEditMsg{credential: *cred}
		},
		m.loadSSOProfiles(),
	)
}

// SetReturnTo sets the state to return to after saving
func (m *CredentialsFormModel) SetReturnTo(state AppState) {
	m.returnTo = state
}

// visibleFields returns the field IDs shown for the current auth type.
func (m CredentialsFormModel) visibleFields() []int {
	fields := []int{credFieldType, credFieldName}
	if m.authType == core.AuthSSO {
		fields = append(fields, credFieldProfile)
	} else {
		fields = append(fields, credFieldAccessKey, credFieldSecretKey)
	}
	return append(fields, credFieldRegion, credFieldSave)
}

// currentField returns the focused field ID.
func (m CredentialsFormModel) currentField() int {
	fields := m.visibleFields()
	if m.focusIndex < 0 || m.focusIndex >= len(fields) {
		return credFieldSave
	}
	return fields[m.focusIndex]
}

// fieldInput maps a field ID to its textinput index (-1 when it has none).
func fieldInput(field int) int {
	switch field {
	case credFieldName:
		return inputName
	case credFieldAccessKey:
		return inputAccessKey
	case credFieldSecretKey:
		return inputSecretKey
	case credFieldProfile:
		return inputProfile
	case credFieldRegion:
		return inputRegion
	default:
		return -1
	}
}

// Update handles messages
func (m CredentialsFormModel) Update(msg tea.Msg) (CredentialsFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel and go back
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: m.returnTo}
			}

		case "tab", "down":
			return m.moveFocus(1)

		case "shift+tab", "up":
			return m.moveFocus(-1)

		case "enter":
			if m.currentField() == credFieldSave {
				return m, m.saveCredential()
			}
			return m.moveFocus(1)

		case "left", "h", "right", "l", " ", "space":
			// Toggle the auth type only while the type row is focused; otherwise
			// these keys belong to the text input.
			if m.currentField() == credFieldType {
				if m.authType == core.AuthSSO {
					m.authType = core.AuthStatic
				} else {
					m.authType = core.AuthSSO
				}
				m.notice = ""
				return m, m.focusCurrentField()
			}
		}

	case credentialLoadedForEditMsg:
		// Populate form with existing credential
		m.editMode = true
		m.credName = msg.credential.Name
		m.authType = msg.credential.AuthType()
		m.inputs[inputName].SetValue(msg.credential.Name)
		m.inputs[inputAccessKey].SetValue(msg.credential.AccessKey)
		m.inputs[inputSecretKey].SetValue(msg.credential.SecretKey)
		m.inputs[inputProfile].SetValue(msg.credential.Profile)
		m.inputs[inputRegion].SetValue(msg.credential.DefaultRegion)
		return m, nil

	case ssoProfilesLoadedMsg:
		m.ssoProfiles = msg.profiles
		return m, nil

	case credentialSavedMsg:
		if msg.err != nil {
			m.notice = msg.err.Error()
			return m, nil
		}

		// Return to previous state
		return m, func() tea.Msg {
			return StateChangeMsg{
				NewState: m.returnTo,
				Data:     "credential_saved", // Signal that a credential was saved
			}
		}
	}

	// Update focused input
	cmd := m.updateInputs(msg)
	return m, cmd
}

// moveFocus advances the focus over the currently visible fields.
func (m CredentialsFormModel) moveFocus(delta int) (CredentialsFormModel, tea.Cmd) {
	fields := m.visibleFields()
	m.focusIndex = (m.focusIndex + delta + len(fields)) % len(fields)
	m.notice = ""
	return m, m.focusCurrentField()
}

// focusCurrentField blurs every input and focuses the active one.
func (m *CredentialsFormModel) focusCurrentField() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i].Blur()
	}

	if idx := fieldInput(m.currentField()); idx >= 0 {
		cmds = append(cmds, m.inputs[idx].Focus())
	}

	return tea.Batch(cmds...)
}

// viewRow describes one labeled input row in the form.
type credFormRow struct {
	field int
	label string
	input int
}

// View renders the form
func (m CredentialsFormModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(18)

	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	var title string
	if m.editMode {
		title = titleStyle.Render(fmt.Sprintf("Edit AWS Credentials: %s", m.credName))
	} else {
		title = titleStyle.Render("Add AWS Credentials")
	}

	form := title + "\n\n"

	// Type row
	typeValue := "Static keys"
	if m.authType == core.AuthSSO {
		typeValue = "AWS SSO"
	}
	typeLabel := labelStyle.Render("Type:")
	if m.currentField() == credFieldType {
		form += activeStyle.Render("▶ ") + typeLabel + " " + activeStyle.Render("‹ "+typeValue+" ›") + "\n"
	} else {
		form += "  " + typeLabel + " " + typeValue + "\n"
	}

	rows := []credFormRow{
		{credFieldName, "Credential Name:", inputName},
	}
	if m.authType == core.AuthSSO {
		rows = append(rows, credFormRow{credFieldProfile, "SSO Profile:", inputProfile})
	} else {
		rows = append(rows,
			credFormRow{credFieldAccessKey, "Access Key:", inputAccessKey},
			credFormRow{credFieldSecretKey, "Secret Key:", inputSecretKey},
		)
	}
	rows = append(rows, credFormRow{credFieldRegion, "Default Region:", inputRegion})

	for _, row := range rows {
		label := labelStyle.Render(row.label)
		if m.currentField() == row.field {
			form += activeStyle.Render("▶ ") + label + " " + m.inputs[row.input].View() + "\n"
		} else {
			form += "  " + label + " " + m.inputs[row.input].View() + "\n"
		}
	}

	// Save button
	if m.currentField() == credFieldSave {
		form += "\n" + activeStyle.Bold(true).Render("▶ [ Save ]") + "\n"
	} else {
		form += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  [ Save ]") + "\n"
	}

	if m.authType == core.AuthSSO {
		form += m.ssoProfilesHelp() + "\n"
	}

	if m.notice != "" {
		form += lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("✗ "+m.notice) + "\n"
	}

	helpText := lipgloss.NewStyle().
		Faint(true).
		Render("tab/enter: next • shift+tab/↑: prev • ←/→: change type • esc: cancel")

	form += helpText

	return lipgloss.NewStyle().
		Padding(2, 4).
		Render(form)
}

// ssoProfilesHelp lists the discoverable profiles so the user knows what to type.
func (m CredentialsFormModel) ssoProfilesHelp() string {
	if len(m.ssoProfiles) == 0 {
		return lipgloss.NewStyle().
			Faint(true).
			Render("  No SSO profiles found in ~/.aws/config. Configure one with `aws configure sso`.")
	}

	names := make([]string, 0, len(m.ssoProfiles))
	for _, p := range m.ssoProfiles {
		names = append(names, p.Name)
	}
	if len(names) > 6 {
		names = append(names[:6], "…")
	}

	return lipgloss.NewStyle().
		Faint(true).
		Render("  Available profiles: " + strings.Join(names, ", "))
}

// updateInputs updates the focused input
func (m *CredentialsFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

// buildCredential assembles a credential from the form values.
func (m CredentialsFormModel) buildCredential() credentials.AWSCredential {
	cred := credentials.AWSCredential{
		Name:          strings.TrimSpace(m.inputs[inputName].Value()),
		Type:          m.authType,
		DefaultRegion: strings.TrimSpace(m.inputs[inputRegion].Value()),
	}

	if m.authType == core.AuthSSO {
		cred.Profile = strings.TrimSpace(m.inputs[inputProfile].Value())
		return cred
	}

	cred.AccessKey = strings.TrimSpace(m.inputs[inputAccessKey].Value())
	cred.SecretKey = strings.TrimSpace(m.inputs[inputSecretKey].Value())
	return cred
}

// saveCredential saves the credential to file
func (m CredentialsFormModel) saveCredential() tea.Cmd {
	return func() tea.Msg {
		cred := m.buildCredential()

		if m.authType == core.AuthSSO {
			profiles, err := provaws.ListSSOProfiles()
			if err == nil && len(profiles) > 0 {
				found := false
				for _, p := range profiles {
					if p.Name == cred.Profile {
						found = true
						cred.SSOStartURL = p.StartURL
						cred.SSORegion = p.SSORegion
						break
					}
				}
				if !found {
					return credentialSavedMsg{err: fmt.Errorf("profile %q not found in %s", cred.Profile, provaws.DefaultAWSConfigPath())}
				}
			}
		}

		// Validate
		if err := cred.Validate(); err != nil {
			return credentialSavedMsg{err: err}
		}

		// Load existing credentials
		creds, err := credentials.LoadCredentials("cloud-credentials.yaml")
		if err != nil {
			return credentialSavedMsg{err: err}
		}

		// Add or update credential
		if err := creds.AddAWSCredential(cred); err != nil {
			return credentialSavedMsg{err: err}
		}

		// Save to file
		if err := creds.Save("cloud-credentials.yaml"); err != nil {
			return credentialSavedMsg{err: err}
		}

		return credentialSavedMsg{name: cred.Name}
	}
}

// loadSSOProfiles discovers profiles from ~/.aws/config for autocomplete.
func (m CredentialsFormModel) loadSSOProfiles() tea.Cmd {
	return func() tea.Msg {
		profiles, err := provaws.ListSSOProfiles()
		if err != nil {
			return ssoProfilesLoadedMsg{}
		}
		return ssoProfilesLoadedMsg{profiles: profiles}
	}
}

// Message types
type credentialLoadedForEditMsg struct {
	credential credentials.AWSCredential
}

type credentialLoadErrorMsg struct {
	err error
}

type credentialSavedMsg struct {
	name string
	err  error
}

type ssoProfilesLoadedMsg struct {
	profiles []provaws.SSOProfile
}
