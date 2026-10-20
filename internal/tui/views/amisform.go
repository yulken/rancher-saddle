package views

import (
	"fmt"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AMIsFormModel is the create/edit form for a single AMI entry.
type AMIsFormModel struct {
	width      int
	height     int
	inputs     []textinput.Model
	focusIndex int
	editMode   bool
	origDistro string
	origRegion string
}

// NewAMIsFormModel creates a new AMI entry form.
func NewAMIsFormModel() AMIsFormModel {
	m := AMIsFormModel{
		width:  80,
		height: 20,
		inputs: make([]textinput.Model, 3),
	}

	// Distro name
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "Ubuntu 22.04 LTS"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 60
	m.inputs[0].Width = 40

	// Region
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "us-east-1"
	m.inputs[1].CharLimit = 30
	m.inputs[1].Width = 30

	// AMI ID
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "ami-xxxxxxxxxxxxxxxxx"
	m.inputs[2].CharLimit = 60
	m.inputs[2].Width = 50

	return m
}

// Init initializes the form.
func (m AMIsFormModel) Init() tea.Cmd {
	return textinput.Blink
}

// SetSize updates the form dimensions.
func (m *AMIsFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetEditMode loads the existing entry identified by "distro|region".
func (m *AMIsFormModel) SetEditMode(key string) tea.Cmd {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		return nil
	}
	m.editMode = true
	m.origDistro = parts[0]
	m.origRegion = parts[1]

	return func() tea.Msg {
		amis, err := config.LoadAMIs("amis.yaml")
		if err != nil {
			return amiLoadErrorMsg{err: err}
		}
		amiID, _ := amis.GetAMI(m.origDistro, m.origRegion)
		return amiLoadedForEditMsg{
			distro: m.origDistro,
			region: m.origRegion,
			amiID:  amiID,
		}
	}
}

// Update handles messages.
func (m AMIsFormModel) Update(msg tea.Msg) (AMIsFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateAMIsList}
			}

		case "tab", "down", "enter":
			if m.focusIndex == len(m.inputs) {
				return m, m.saveEntry()
			}
			m.focusIndex++
			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			}
			return m, m.updateFocus()

		case "shift+tab", "up":
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}
			return m, m.updateFocus()
		}

	case amiLoadedForEditMsg:
		m.inputs[0].SetValue(msg.distro)
		m.inputs[1].SetValue(msg.region)
		m.inputs[2].SetValue(msg.amiID)
		return m, nil

	case amiSavedMsg:
		if msg.err != nil {
			return m, nil
		}
		return m, func() tea.Msg {
			return StateChangeMsg{NewState: StateAMIsList}
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

// View renders the form.
func (m AMIsFormModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(22)

	var title string
	if m.editMode {
		title = titleStyle.Render(fmt.Sprintf("Edit AMI: %s / %s", m.origDistro, m.origRegion))
	} else {
		title = titleStyle.Render("Add AMI Entry")
	}

	labels := []string{"Distro Name:", "Region:", "AMI ID:"}

	form := title + "\n\n"
	for i := 0; i < len(m.inputs); i++ {
		label := labelStyle.Render(labels[i])
		if m.focusIndex == i {
			form += lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ") + label + " " + m.inputs[i].View() + "\n"
		} else {
			form += "  " + label + " " + m.inputs[i].View() + "\n"
		}
	}

	saveButton := ""
	if m.focusIndex == len(m.inputs) {
		saveButton = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("▶ [ Save ]")
	} else {
		saveButton = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  [ Save ]")
	}
	form += "\n" + saveButton + "\n\n"
	form += lipgloss.NewStyle().Faint(true).Render("tab/enter: next • shift+tab/↑: prev • esc: cancel")

	return lipgloss.NewStyle().Padding(2, 4).Render(form)
}

func (m *AMIsFormModel) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focusIndex {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

func (m *AMIsFormModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m AMIsFormModel) saveEntry() tea.Cmd {
	distro := m.inputs[0].Value()
	region := m.inputs[1].Value()
	amiID := m.inputs[2].Value()
	origDistro := m.origDistro
	origRegion := m.origRegion
	editMode := m.editMode

	return func() tea.Msg {
		if distro == "" {
			return amiSavedMsg{err: fmt.Errorf("distro name is required")}
		}
		if region == "" {
			return amiSavedMsg{err: fmt.Errorf("region is required")}
		}
		if amiID == "" {
			return amiSavedMsg{err: fmt.Errorf("AMI ID is required")}
		}

		amis, err := config.LoadAMIs("amis.yaml")
		if err != nil {
			return amiSavedMsg{err: err}
		}

		// In edit mode, remove the old entry if the key changed
		if editMode && (origDistro != distro || origRegion != region) {
			_ = amis.DeleteEntry(origDistro, origRegion)
		}

		amis.AddEntry(config.AMIEntry{
			Distro: distro,
			Region: region,
			AMIID:  amiID,
		})

		if err := amis.Save("amis.yaml"); err != nil {
			return amiSavedMsg{err: err}
		}
		return amiSavedMsg{}
	}
}

// Message types
type amiLoadedForEditMsg struct {
	distro string
	region string
	amiID  string
}

type amiLoadErrorMsg struct {
	err error
}

type amiSavedMsg struct {
	err error
}
