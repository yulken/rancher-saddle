package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Felipalds/rancher-tui/internal/model"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	helpStyle           = blurredStyle.Copy()
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

type Model struct {
	focusIndex int
	inputs     []textinput.Model
	config     *model.Config
	done       bool
	quitting   bool

	// NEW: For distribution selection
	distributionCursor    int  // 0=RKE2, 1=K3s
	selectingDistribution bool // True when in distribution selection mode
}

func NewModel(cfg *model.Config) Model {
	// Initialize distribution cursor based on config
	distCursor := 0
	if cfg.KubernetesDistribution == "k3s" {
		distCursor = 1
	}

	m := Model{
		inputs:                make([]textinput.Model, 14), // Increased to 14 for version field
		config:                cfg,
		distributionCursor:    distCursor,
		selectingDistribution: false,
	}

	for i := range m.inputs {
		t := textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64 // increased limit

		switch i {
		case 0:
			t.Prompt = "AWS Access Key: "
			t.Placeholder = "AKIA..."
			t.SetValue(cfg.AWSAccessKey)
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Prompt = "AWS Secret Key: "
			t.Placeholder = "Secret..."
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
			t.SetValue(cfg.AWSSecretKey)
		case 2:
			t.Prompt = "AWS Region: "
			t.Placeholder = "us-east-1"
			t.SetValue(cfg.AWSRegion)
		case 3:
			t.Prompt = "Subnet ID: "
			t.Placeholder = "subnet-..."
			t.SetValue(cfg.SubnetID)
		case 4:
			t.Prompt = "Security Group ID: "
			t.Placeholder = "sg-..."
			t.SetValue(cfg.SecurityGroupID)
		case 5:
			t.Prompt = "SSH Key Name: "
			t.Placeholder = "my-key-pair"
			t.SetValue(cfg.SSHKeyName)
		case 6:
			t.Prompt = "Private Key Path: "
			t.Placeholder = "/home/user/.ssh/id_rsa"
			t.SetValue(cfg.SSHPrivateKeyPath)
		case 7:
			t.Prompt = "Node Prefix: "
			t.Placeholder = "rancher-node"
			t.SetValue(cfg.NodePrefix)
		case 8:
			t.Prompt = "AMI ID: "
			t.Placeholder = "ami-..."
			t.SetValue(cfg.AMI)
		case 9:
			t.Prompt = "Instance Count: "
			t.Placeholder = "1"
			t.SetValue(strconv.Itoa(cfg.InstanceCount))
			t.Validate = func(s string) error {
				_, err := strconv.Atoi(s)
				return err
			}
		case 10:
			t.Prompt = "Root Volume Size (GB): "
			t.Placeholder = "20"
			t.SetValue(strconv.Itoa(cfg.RootVolumeSize))
			t.Validate = func(s string) error {
				val, err := strconv.Atoi(s)
				if err != nil {
					return err
				}
				if val < 10 {
					return fmt.Errorf("minimum 10 GB required")
				}
				return nil
			}
		case 11:
			// Kubernetes Distribution (display only, use left/right arrows to select)
			t.Prompt = ""
			t.Placeholder = "Use ← → to select"
			t.SetValue(cfg.KubernetesDistribution)
		case 12:
			// Kubernetes Version (dynamic based on distribution)
			distName := "Kubernetes"
			if cfg.KubernetesDistribution == "rke2" {
				distName = "RKE2"
			} else if cfg.KubernetesDistribution == "k3s" {
				distName = "K3s"
			}
			t.Prompt = fmt.Sprintf("%s Version: ", distName)
			t.Placeholder = getVersionPlaceholder(cfg.KubernetesDistribution)
			t.SetValue(cfg.KubernetesVersion)
			t.CharLimit = 30
		case 13:
			// Rancher Version
			t.Prompt = "Rancher Version: "
			t.Placeholder = "2.x.x"
			t.SetValue(cfg.RancherVersion)
		}

		m.inputs[i] = t
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		// Handle distribution selection when focused on field 11 (use left/right arrows)
		case "left", "right":
			if m.focusIndex == 11 {
				if msg.String() == "left" && m.distributionCursor > 0 {
					m.distributionCursor--
					m.updateDistribution()
				} else if msg.String() == "right" && m.distributionCursor < 1 {
					m.distributionCursor++
					m.updateDistribution()
				}
				return m, nil
			}

		// Change focus
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			if s == "enter" && m.focusIndex == len(m.inputs) {
				m.done = true
				m.updateConfig()
				return m, tea.Quit
			}

			// Navigation (works everywhere including distribution selector)
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			return m, tea.Batch(cmds...)
		}
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	return m, cmd
}

func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only update the focused input
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex], cmds[m.focusIndex] = m.inputs[m.focusIndex].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.done {
		return "Deploying Rancher...\n"
	}
	if m.quitting {
		return "Aborted.\n"
	}

	var b strings.Builder

	b.WriteString("Forge Config\n\n")

	// Render fields 0-10 normally
	for i := 0; i <= 10; i++ {
		b.WriteString(m.inputs[i].View())
		b.WriteRune('\n')
	}

	// Field 11: Kubernetes Distribution (special rendering)
	if m.focusIndex == 11 {
		b.WriteString(focusedStyle.Render("Kubernetes Distribution:"))
		b.WriteString("\n")
		b.WriteString(renderDistributionSelector(m.distributionCursor, true))
	} else {
		b.WriteString(noStyle.Render("Kubernetes Distribution:"))
		b.WriteString("\n")
		b.WriteString(renderDistributionSelector(m.distributionCursor, false))
	}

	// Field 12: Kubernetes Version
	b.WriteString(m.inputs[12].View())
	b.WriteRune('\n')

	// Field 13: Rancher Version
	b.WriteString(m.inputs[13].View())

	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	fmt.Fprintf(&b, "\n\n%s\n\n", *button)

	// Show context-sensitive help
	if m.focusIndex == 11 {
		b.WriteString(helpStyle.Render("Use ← → arrows to select distribution • tab/enter to continue"))
	} else {
		b.WriteString(helpStyle.Render("tab/enter: next • shift+tab/↑: prev • ctrl+c: quit"))
	}

	return b.String()
}

func (m *Model) updateConfig() {
	m.config.AWSAccessKey = m.inputs[0].Value()
	m.config.AWSSecretKey = m.inputs[1].Value()
	m.config.AWSRegion = m.inputs[2].Value()
	m.config.SubnetID = m.inputs[3].Value()
	m.config.SecurityGroupID = m.inputs[4].Value()
	m.config.SSHKeyName = m.inputs[5].Value()
	m.config.SSHPrivateKeyPath = m.inputs[6].Value()
	m.config.NodePrefix = m.inputs[7].Value()
	m.config.AMI = m.inputs[8].Value()

	if i, err := strconv.Atoi(m.inputs[9].Value()); err == nil {
		m.config.InstanceCount = i
	}

	if i, err := strconv.Atoi(m.inputs[10].Value()); err == nil {
		m.config.RootVolumeSize = i
	}

	// Use the selected distribution and version
	distributions := []string{"rke2", "k3s"}
	m.config.KubernetesDistribution = distributions[m.distributionCursor]
	m.config.KubernetesVersion = m.inputs[12].Value()

	// Set default version if empty
	if m.config.KubernetesVersion == "" {
		if m.config.KubernetesDistribution == "rke2" {
			m.config.KubernetesVersion = "v1.33.7+rke2r1"
		} else if m.config.KubernetesDistribution == "k3s" {
			m.config.KubernetesVersion = "v1.30.3+k3s1"
		}
	}

	m.config.RancherVersion = m.inputs[13].Value()

	// Backward compatibility: set RKE2Version if distribution is RKE2
	if m.config.KubernetesDistribution == "rke2" {
		m.config.RKE2Version = m.config.KubernetesVersion
	}
}

func (m *Model) updateDistribution() {
	distributions := []string{"rke2", "k3s"}
	selected := distributions[m.distributionCursor]

	m.inputs[11].SetValue(selected)
	m.config.KubernetesDistribution = selected

	// Update version field prompt and placeholder
	distName := "Kubernetes"
	if selected == "rke2" {
		distName = "RKE2"
	} else if selected == "k3s" {
		distName = "K3s"
	}
	m.inputs[12].Prompt = fmt.Sprintf("%s Version: ", distName)
	m.inputs[12].Placeholder = getVersionPlaceholder(selected)

	// Set default version when switching if not already set
	if m.config.KubernetesVersion == "" ||
		(selected == "rke2" && !strings.Contains(m.config.KubernetesVersion, "rke2")) ||
		(selected == "k3s" && !strings.Contains(m.config.KubernetesVersion, "k3s")) {
		if selected == "rke2" {
			m.inputs[12].SetValue("v1.33.7+rke2r1")
			m.config.KubernetesVersion = "v1.33.7+rke2r1"
		} else {
			m.inputs[12].SetValue("v1.30.3+k3s1")
			m.config.KubernetesVersion = "v1.30.3+k3s1"
		}
	}
}

func renderDistributionSelector(cursor int, focused bool) string {
	options := []string{"RKE2", "K3s"}
	var b strings.Builder

	for i, opt := range options {
		if i == cursor {
			if focused {
				b.WriteString(focusedStyle.Render(fmt.Sprintf("▶ %s", opt)))
			} else {
				b.WriteString(noStyle.Render(fmt.Sprintf("▶ %s", opt)))
			}
		} else {
			b.WriteString(noStyle.Render(fmt.Sprintf("  %s", opt)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func getVersionPlaceholder(distribution string) string {
	switch distribution {
	case "rke2":
		return "e.g., v1.33.7+rke2r1"
	case "k3s":
		return "e.g., v1.30.3+k3s1"
	default:
		return "version"
	}
}

func (m Model) Done() bool {
	return m.done
}
