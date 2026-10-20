package views

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/credentials"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CredentialsFormModel represents the AWS credentials form
type CredentialsFormModel struct {
	width      int
	height     int
	inputs     []textinput.Model
	focusIndex int
	editMode   bool // true if editing existing credential
	credName   string
	returnTo   AppState // State to return to after saving
}

// NewCredentialsFormModel creates a new credentials form
func NewCredentialsFormModel() CredentialsFormModel {
	m := CredentialsFormModel{
		width:      80,
		height:     20,
		inputs:     make([]textinput.Model, 4),
		focusIndex: 0,
		editMode:   false,
		returnTo:   StateCredentialsList, // Default return to credentials list
	}

	// Field 0: Credential Name
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "my-aws-account"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 50
	m.inputs[0].Width = 40

	// Field 1: AWS Access Key
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "AKIA..."
	m.inputs[1].CharLimit = 50
	m.inputs[1].Width = 40

	// Field 2: AWS Secret Key
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "Secret key..."
	m.inputs[2].EchoMode = textinput.EchoPassword
	m.inputs[2].EchoCharacter = '•'
	m.inputs[2].CharLimit = 100
	m.inputs[2].Width = 40

	// Field 3: Default Region
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "us-east-1"
	m.inputs[3].CharLimit = 20
	m.inputs[3].Width = 40

	return m
}

// Init initializes the form
func (m CredentialsFormModel) Init() tea.Cmd {
	return textinput.Blink
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

	return func() tea.Msg {
		creds, err := credentials.LoadCredentials("cloud-credentials.yaml")
		if err != nil {
			return credentialLoadErrorMsg{err: err}
		}

		cred, err := creds.GetAWSCredential(credName)
		if err != nil {
			return credentialLoadErrorMsg{err: err}
		}

		return credentialLoadedForEditMsg{credential: *cred}
	}
}

// SetReturnTo sets the state to return to after saving
func (m *CredentialsFormModel) SetReturnTo(state AppState) {
	m.returnTo = state
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

		case "tab", "down", "enter":
			// Move to next field or save
			if m.focusIndex == len(m.inputs) {
				// Save button focused - save the credential
				return m, m.saveCredential()
			}

			m.focusIndex++
			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			}

			return m, m.updateFocus()

		case "shift+tab", "up":
			// Move to previous field
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			return m, m.updateFocus()
		}

	case credentialLoadedForEditMsg:
		// Populate form with existing credential
		m.inputs[0].SetValue(msg.credential.Name)
		m.inputs[1].SetValue(msg.credential.AccessKey)
		m.inputs[2].SetValue(msg.credential.SecretKey)
		m.inputs[3].SetValue(msg.credential.DefaultRegion)
		return m, nil

	case credentialSavedMsg:
		if msg.err != nil {
			// TODO: Show error message
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

// View renders the form
func (m CredentialsFormModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(20)

	var title string
	if m.editMode {
		title = titleStyle.Render(fmt.Sprintf("Edit AWS Credentials: %s", m.credName))
	} else {
		title = titleStyle.Render("Add AWS Credentials")
	}

	labels := []string{
		"Credential Name:",
		"Access Key:",
		"Secret Key:",
		"Default Region:",
	}

	form := title + "\n\n"

	for i := 0; i < len(m.inputs); i++ {
		label := labelStyle.Render(labels[i])

		if m.focusIndex == i {
			form += lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ") + label + " " + m.inputs[i].View() + "\n"
		} else {
			form += "  " + label + " " + m.inputs[i].View() + "\n"
		}
	}

	// Save button
	saveButton := "[ Save ]"
	if m.focusIndex == len(m.inputs) {
		saveButton = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Render("▶ [ Save ]")
	} else {
		saveButton = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("  [ Save ]")
	}

	form += "\n" + saveButton + "\n\n"

	helpText := lipgloss.NewStyle().
		Faint(true).
		Render("tab/enter: next • shift+tab/↑: prev • esc: cancel")

	form += helpText

	return lipgloss.NewStyle().
		Padding(2, 4).
		Render(form)
}

// updateFocus updates which field is focused
func (m *CredentialsFormModel) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := 0; i < len(m.inputs); i++ {
		if i == m.focusIndex {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}

	return tea.Batch(cmds...)
}

// updateInputs updates the focused input
func (m *CredentialsFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := 0; i < len(m.inputs); i++ {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

// saveCredential saves the credential to file
func (m CredentialsFormModel) saveCredential() tea.Cmd {
	return func() tea.Msg {
		cred := credentials.AWSCredential{
			Name:          m.inputs[0].Value(),
			AccessKey:     m.inputs[1].Value(),
			SecretKey:     m.inputs[2].Value(),
			DefaultRegion: m.inputs[3].Value(),
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
