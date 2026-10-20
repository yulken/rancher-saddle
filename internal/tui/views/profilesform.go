package views

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProfilesFormModel represents the profile creation/edit form
type ProfilesFormModel struct {
	width       int
	height      int
	inputs      []textinput.Model
	focusIndex  int
	editMode    bool
	profileName string
}

// NewProfilesFormModel creates a new profiles form
func NewProfilesFormModel() ProfilesFormModel {
	m := ProfilesFormModel{
		width:      80,
		height:     20,
		inputs:     make([]textinput.Model, 9),
		focusIndex: 0,
		editMode:   false,
	}

	// Profile Name
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "default"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 50
	m.inputs[0].Width = 40

	// Region
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "us-east-1"
	m.inputs[1].CharLimit = 20
	m.inputs[1].Width = 40

	// Subnet ID
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "subnet-xxxxx"
	m.inputs[2].CharLimit = 50
	m.inputs[2].Width = 50

	// Security Group ID
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "sg-xxxxx"
	m.inputs[3].CharLimit = 50
	m.inputs[3].Width = 50

	// AMI
	m.inputs[4] = textinput.New()
	m.inputs[4].Placeholder = "ami-xxxxx"
	m.inputs[4].CharLimit = 50
	m.inputs[4].Width = 50

	// Instance Type
	m.inputs[5] = textinput.New()
	m.inputs[5].Placeholder = "t3.xlarge"
	m.inputs[5].CharLimit = 20
	m.inputs[5].Width = 40

	// SSH Key Name
	m.inputs[6] = textinput.New()
	m.inputs[6].Placeholder = "my-key"
	m.inputs[6].CharLimit = 50
	m.inputs[6].Width = 40

	// SSH Private Key Path
	m.inputs[7] = textinput.New()
	m.inputs[7].Placeholder = "~/.ssh/my-key.pem"
	m.inputs[7].CharLimit = 100
	m.inputs[7].Width = 60

	// SSH User
	m.inputs[8] = textinput.New()
	m.inputs[8].Placeholder = "ubuntu"
	m.inputs[8].CharLimit = 20
	m.inputs[8].Width = 30

	return m
}

// Init initializes the form
func (m ProfilesFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// SetSize updates the form dimensions
func (m *ProfilesFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetEditMode sets the form to edit an existing profile
func (m *ProfilesFormModel) SetEditMode(profileName string) tea.Cmd {
	m.editMode = true
	m.profileName = profileName

	return func() tea.Msg {
		profiles, err := config.LoadProfiles("profiles.yaml")
		if err != nil {
			return profileLoadErrorMsg{err: err}
		}

		profile, err := profiles.GetProfile(profileName)
		if err != nil {
			return profileLoadErrorMsg{err: err}
		}

		return profileLoadedForEditMsg{profile: *profile}
	}
}

// Update handles messages
func (m ProfilesFormModel) Update(msg tea.Msg) (ProfilesFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Cancel and go back
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateProfilesList}
			}

		case "tab", "down", "enter":
			// Move to next field or save
			if m.focusIndex == len(m.inputs) {
				// Save button focused
				return m, m.saveProfile()
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

	case profileLoadedForEditMsg:
		// Populate form with existing profile
		m.inputs[0].SetValue(msg.profile.Name)
		m.inputs[1].SetValue(msg.profile.Region)
		m.inputs[2].SetValue(msg.profile.SubnetID)
		m.inputs[3].SetValue(msg.profile.SecurityGroupID)
		m.inputs[4].SetValue(msg.profile.AMI)
		m.inputs[5].SetValue(msg.profile.InstanceType)
		m.inputs[6].SetValue(msg.profile.SSHKeyName)
		m.inputs[7].SetValue(msg.profile.SSHPrivateKeyPath)
		m.inputs[8].SetValue(msg.profile.SSHUser)
		return m, nil

	case profileSavedMsg:
		if msg.err != nil {
			// TODO: Show error message
			return m, nil
		}

		// Return to profiles list
		return m, func() tea.Msg {
			return StateChangeMsg{NewState: StateProfilesList}
		}
	}

	// Update focused input
	cmd := m.updateInputs(msg)
	return m, cmd
}

// View renders the form
func (m ProfilesFormModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(22)

	var title string
	if m.editMode {
		title = titleStyle.Render("Edit Profile: " + m.profileName)
	} else {
		title = titleStyle.Render("Create New Profile")
	}

	labels := []string{
		"Profile Name:",
		"Region:",
		"Subnet ID:",
		"Security Group ID:",
		"AMI:",
		"Instance Type:",
		"SSH Key Name:",
		"SSH Private Key Path:",
		"SSH User:",
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
func (m *ProfilesFormModel) updateFocus() tea.Cmd {
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
func (m *ProfilesFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := 0; i < len(m.inputs); i++ {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

// saveProfile saves the profile to file
func (m ProfilesFormModel) saveProfile() tea.Cmd {
	return func() tea.Msg {
		profile := &config.Profile{
			Name:              m.inputs[0].Value(),
			Region:            m.inputs[1].Value(),
			SubnetID:          m.inputs[2].Value(),
			SecurityGroupID:   m.inputs[3].Value(),
			AMI:               m.inputs[4].Value(),
			InstanceType:      m.inputs[5].Value(),
			SSHKeyName:        m.inputs[6].Value(),
			SSHPrivateKeyPath: m.inputs[7].Value(),
			SSHUser:           m.inputs[8].Value(),
		}

		// Validate
		if profile.Name == "" {
			return profileSavedMsg{err: fmt.Errorf("profile name is required")}
		}

		// Load existing profiles
		profiles, err := config.LoadProfiles("profiles.yaml")
		if err != nil {
			return profileSavedMsg{err: err}
		}

		// Add or update profile
		profiles.AddProfile(profile.Name, profile)

		// Save to file
		if err := profiles.Save("profiles.yaml"); err != nil {
			return profileSavedMsg{err: err}
		}

		return profileSavedMsg{name: profile.Name}
	}
}

// Message types
type profileLoadedForEditMsg struct {
	profile config.Profile
}

type profileLoadErrorMsg struct {
	err error
}

type profileSavedMsg struct {
	name string
	err  error
}
