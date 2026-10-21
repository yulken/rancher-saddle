package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/config"
	"github.com/Felipalds/rancher-tui/internal/core"
	"github.com/Felipalds/rancher-tui/internal/credentials"
	"github.com/Felipalds/rancher-tui/internal/workflow"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FieldType represents the type of form field
type FieldType int

const (
	FieldText FieldType = iota
	FieldSelect
)

// FormField represents a single form field
type FormField struct {
	fieldType   FieldType
	label       string
	input       textinput.Model
	options     []string // For select fields
	selected    int      // Selected option index
	placeholder string
	hidden      bool
}

// Field indices — kept as constants so the rest of the file stays in sync.
const (
	fieldInstallMethod  = 0
	fieldProvider       = 1
	fieldCredentials    = 2
	fieldK8sDistro      = 3
	fieldClusterName    = 4
	fieldNodePrefix     = 5
	fieldRegion         = 6
	fieldSubnetID       = 7
	fieldSecurityGroup  = 8
	fieldOSImage        = 9
	fieldCustomAMI      = 10
	fieldInstanceType   = 11
	fieldInstanceCount  = 12
	fieldSSHKeyName     = 13
	fieldSSHKeyPath     = 14
	fieldSSHUser        = 15
	fieldK8sVersion     = 16
	fieldDeployRancher  = 17
	fieldRancherPrime   = 18
	fieldRancherVersion = 19
	fieldBootstrapPwd   = 20
	fieldImageTag       = 21
	fieldDebugMode      = 22
	fieldHostPort       = 23 // Docker-only: custom HTTPS port

	// Harvester/Vagrant local-install fields (appended so existing indices stay
	// stable). Shown only on the Vagrant + Harvester path.
	fieldHvPassword  = 24
	fieldHvToken     = 25
	fieldHvVIP       = 26
	fieldHvVIPMode   = 27
	fieldHvNodeCPU   = 28
	fieldHvNodeMem   = 29
	fieldHvNodeDisk  = 30
	fieldHvSubnet    = 31
	fieldHvNTP       = 32
	fieldHvMgmtIface = 33
	fieldHvISOPath   = 34
	fieldHvFirmware  = 35
)

// harvesterOnlyFields are shown only on the local Harvester (libvirt) path.
// fieldHvPXEIP is intentionally omitted — there is no PXE server in the
// ISO-install flow.
var harvesterOnlyFields = []int{
	fieldHvPassword, fieldHvToken, fieldHvVIP, fieldHvVIPMode,
	fieldHvNodeCPU, fieldHvNodeMem, fieldHvNodeDisk,
	fieldHvSubnet, fieldHvNTP, fieldHvMgmtIface, fieldHvFirmware, fieldHvISOPath,
}

// CreateFormModel represents the simplified cluster creation form
type CreateFormModel struct {
	width  int
	height int

	// Form fields
	fields      []FormField
	focusIndex  int
	credentials *credentials.CloudCredentials
	profiles    *config.ProfilesConfig
	amis        *config.AMIsConfig

	// Scroll state for tall forms
	scrollOffset       int
	showProfileSelect  bool
	profileSelectIndex int

	// statusMsg surfaces inline notices (e.g. not-yet-implemented selections)
	statusMsg string

	// Harvester version picker (local-VM path): available = releases from GitHub,
	// cached = versions with an ISO already downloaded under harvester-isos/.
	harvesterAvailable []string
	harvesterCached    []string

	// awsOSOptions holds the AMI distro options (+"Custom") once loaded, so the
	// shared OS Image field can be restored after leaving the Vagrant path.
	awsOSOptions []string
}

// NewCreateFormModel creates a new simplified creation form
func NewCreateFormModel() CreateFormModel {
	m := CreateFormModel{
		width:        80,
		height:       20,
		focusIndex:   0,
		scrollOffset: 0,
	}

	m.initFields()

	return m
}

// Init initializes the form
func (m CreateFormModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.loadCredentials(),
		m.loadProfiles(),
		m.loadAMIsForForm(),
		loadHarvesterVersions(),
	)
}

// SetSize updates dimensions
func (m *CreateFormModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *CreateFormModel) initFields() {
	m.fields = []FormField{
		// [0] Installation Method
		{
			fieldType: FieldSelect,
			label:     "Installation Method",
			options:   []string{"Local", "Docker"},
			selected:  0,
		},
		// [1] Provider selection
		{
			fieldType: FieldSelect,
			label:     "Provider",
			options:   []string{"AWS", "Local VM (libvirt)", "Azure (Coming soon)", "GCP (Coming soon)"},
			selected:  0,
		},
		// [2] Credentials selection (will be populated when credentials load)
		{
			fieldType: FieldSelect,
			label:     "Credentials",
			options:   []string{"Loading..."},
			selected:  0,
		},
		// [3] Kubernetes Distribution
		{
			fieldType: FieldSelect,
			label:     "Kubernetes Distribution",
			options:   []string{"RKE2", "K3s", "Harvester"},
			selected:  0,
		},
		// [4] Cluster Name
		{
			fieldType:   FieldText,
			label:       "Cluster Name",
			input:       m.createTextInput("my-cluster", 50, 40),
			placeholder: "my-cluster",
		},
		// [5] Node Prefix
		{
			fieldType:   FieldText,
			label:       "Node Prefix",
			input:       m.createTextInput("k8s-node", 30, 40),
			placeholder: "k8s-node",
		},
		// [6] Region
		{
			fieldType:   FieldText,
			label:       "Region",
			input:       m.createTextInput("us-east-1", 20, 40),
			placeholder: "us-east-1",
		},
		// [7] Subnet ID
		{
			fieldType:   FieldText,
			label:       "Subnet ID",
			input:       m.createTextInput("subnet-xxxxx", 50, 50),
			placeholder: "subnet-xxxxx",
		},
		// [8] Security Group ID
		{
			fieldType:   FieldText,
			label:       "Security Group ID",
			input:       m.createTextInput("sg-xxxxx", 50, 50),
			placeholder: "sg-xxxxx",
		},
		// [9] OS Image (distro picker — resolves to an AMI ID at submit time).
		// Reused as the Harvester version picker on the local-VM (Vagrant) path.
		{
			fieldType: FieldSelect,
			label:     "OS Image",
			options:   awsOSImageOptions,
			selected:  0,
		},
		// [10] Custom AMI ID (only visible when OS Image == "Custom")
		{
			fieldType:   FieldText,
			label:       "Custom AMI ID",
			input:       m.createTextInput("ami-xxxxx", 50, 50),
			placeholder: "ami-xxxxx",
			hidden:      true,
		},
		// [11] Instance Type
		{
			fieldType:   FieldText,
			label:       "Instance Type",
			input:       m.createTextInput("t3.xlarge", 20, 40),
			placeholder: "t3.xlarge",
		},
		// [12] Instance Count
		{
			fieldType:   FieldText,
			label:       "Instance Count",
			input:       m.createTextInput("3", 2, 10),
			placeholder: "3",
		},
		// [13] SSH Key Name
		{
			fieldType:   FieldText,
			label:       "SSH Key Name",
			input:       m.createTextInput("my-key", 50, 40),
			placeholder: "my-key",
		},
		// [14] SSH Private Key Path
		{
			fieldType:   FieldText,
			label:       "SSH Private Key Path",
			input:       m.createTextInput("~/.ssh/my-key.pem", 100, 60),
			placeholder: "~/.ssh/my-key.pem",
		},
		// [15] SSH User
		{
			fieldType:   FieldText,
			label:       "SSH User",
			input:       m.createTextInput("ubuntu", 20, 30),
			placeholder: "ubuntu",
		},
		// [16] K8s Version
		{
			fieldType:   FieldText,
			label:       "K8s Version",
			input:       m.createTextInput("v1.33.7+rke2r1", 30, 40),
			placeholder: "v1.33.7+rke2r1",
		},
		// [17] Deploy Rancher
		{
			fieldType: FieldSelect,
			label:     "Deploy Rancher",
			options:   []string{"No", "Yes"},
			selected:  0,
		},
		// [18] Rancher Prime
		{
			fieldType: FieldSelect,
			label:     "Rancher Prime",
			options:   []string{"No", "Yes"},
			selected:  0,
		},
		// [19] Rancher Version
		{
			fieldType:   FieldText,
			label:       "Rancher Version",
			input:       m.createTextInput("2.11.7", 20, 30),
			placeholder: "2.11.7",
		},
		// [20] Bootstrap Password
		{
			fieldType:   FieldText,
			label:       "Bootstrap Password",
			input:       m.createTextInput("admin", 50, 40),
			placeholder: "admin",
		},
		// [21] Image Tag (hotfix)
		{
			fieldType:   FieldText,
			label:       "Image Tag (hotfix)",
			input:       m.createTextInput("", 60, 50),
			placeholder: "e.g. v0.0.0-hotfix-abc123.1",
		},
		// [22] Debug Mode
		{
			fieldType: FieldSelect,
			label:     "Debug Mode",
			options:   []string{"No", "Yes"},
			selected:  0,
		},
		// [23] Host Port (Docker-only)
		{
			fieldType:   FieldText,
			label:       "Host Port (HTTPS)",
			input:       m.createTextInput("443", 6, 10),
			placeholder: "443",
			hidden:      true, // Only visible in Docker mode
		},
		// [24] Harvester: Node Password
		{
			fieldType:   FieldText,
			label:       "Node Password",
			input:       m.createTextInput("p@ssword", 50, 40),
			placeholder: "p@ssword",
			hidden:      true,
		},
		// [25] Harvester: Cluster Token
		{
			fieldType:   FieldText,
			label:       "Cluster Token",
			input:       m.createTextInput("token", 60, 40),
			placeholder: "token",
			hidden:      true,
		},
		// [26] Harvester: Cluster VIP (static, outside the DHCP range .100-.200)
		{
			fieldType:   FieldText,
			label:       "Cluster VIP",
			input:       m.createTextInput("192.168.0.50", 20, 30),
			placeholder: "192.168.0.50",
			hidden:      true,
		},
		// [27] Harvester: VIP Mode (static default so the VIP is deterministic)
		{
			fieldType: FieldSelect,
			label:     "VIP Mode",
			options:   []string{"static", "DHCP"},
			selected:  0,
			hidden:    true,
		},
		// [28] Harvester: CPUs per Node
		{
			fieldType:   FieldText,
			label:       "CPUs per Node",
			input:       m.createTextInput("8", 3, 10),
			placeholder: "8",
			hidden:      true,
		},
		// [29] Harvester: Memory per Node (MB)
		{
			fieldType:   FieldText,
			label:       "Memory per Node (MB)",
			input:       m.createTextInput("16384", 7, 12),
			placeholder: "16384",
			hidden:      true,
		},
		// [30] Harvester: Disk per Node (GB)
		{
			fieldType:   FieldText,
			label:       "Disk per Node (GB)",
			input:       m.createTextInput("250", 5, 10),
			placeholder: "250",
			hidden:      true,
		},
		// [31] Harvester: Network Subnet
		{
			fieldType:   FieldText,
			label:       "Network Subnet",
			input:       m.createTextInput("192.168.0.0/24", 20, 24),
			placeholder: "192.168.0.0/24",
			hidden:      true,
		},
		// [32] Harvester: NTP Servers
		{
			fieldType:   FieldText,
			label:       "NTP Servers",
			input:       m.createTextInput("0.suse.pool.ntp.org", 100, 40),
			placeholder: "comma-separated",
			hidden:      true,
		},
		// [33] Harvester: Management Interface
		{
			fieldType:   FieldText,
			label:       "Mgmt Interface",
			input:       m.createTextInput("enp1s0", 20, 20),
			placeholder: "enp1s0",
			hidden:      true,
		},
		// [34] Harvester: Local ISO file (optional override)
		{
			fieldType:   FieldText,
			label:       "Local ISO File (optional)",
			input:       m.createTextInput("", 200, 50),
			placeholder: "/path/to/harvester-v1.4.1-amd64.iso",
			hidden:      true,
		},
		// [35] Harvester: Firmware (UEFI needs OVMF/edk2 on the host)
		{
			fieldType: FieldSelect,
			label:     "Firmware",
			options:   []string{"uefi", "bios"},
			selected:  0,
			hidden:    true,
		},
	}

	// Focus first field
	if len(m.fields) > 0 && m.fields[0].fieldType == FieldText {
		m.fields[0].input.Focus()
	}

	// Apply provider-dependent visibility (e.g. hide Credentials for Vagrant)
	m.syncProviderVisibility()
}

func (m *CreateFormModel) createTextInput(placeholder string, charLimit int, width int) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = charLimit
	input.Width = width
	return input
}

// isDockerMode returns true when the Installation Method is set to Docker.
func (m *CreateFormModel) isDockerMode() bool {
	return m.fields[fieldInstallMethod].options[m.fields[fieldInstallMethod].selected] == "Docker"
}

// syncInstallMethodVisibility shows/hides fields based on the Installation Method.
// Docker mode still uses cloud infrastructure (AWS) — it only skips K8s distribution
// and installs Docker + Rancher directly on the EC2 instance.
func (m *CreateFormModel) syncInstallMethodVisibility() {
	isDocker := m.isDockerMode()

	// In Docker mode, hide K8s-specific fields only (cloud fields stay visible)
	m.fields[fieldK8sDistro].hidden = isDocker
	m.fields[fieldK8sVersion].hidden = isDocker
	m.fields[fieldDeployRancher].hidden = isDocker // Always true in Docker mode

	// Host Port is only visible in Docker mode
	m.fields[fieldHostPort].hidden = !isDocker

	// Force instance count to 1 in Docker mode
	if isDocker {
		m.fields[fieldInstanceCount].input.SetValue("1")
	}

	// Always respect Custom AMI visibility
	m.syncCustomAMIVisibility()
}

// isLocalProvider returns true when the selected Provider is Vagrant (local VMM).
func (m *CreateFormModel) isLocalProvider() bool {
	provider := m.fields[fieldProvider].options[m.fields[fieldProvider].selected]
	return strings.Contains(provider, "libvirt")
}

// isHarvesterDistro returns true when the selected distribution is Harvester.
func (m *CreateFormModel) isHarvesterDistro() bool {
	distro := m.fields[fieldK8sDistro].options[m.fields[fieldK8sDistro].selected]
	return strings.Contains(distro, "Harvester")
}

// selectOption sets a select field to the first option containing substr.
func (m *CreateFormModel) selectOption(fieldIdx int, substr string) {
	for i, opt := range m.fields[fieldIdx].options {
		if strings.Contains(opt, substr) {
			m.fields[fieldIdx].selected = i
			return
		}
	}
}

// coupleProviderAndDistro keeps Provider and Distribution consistent: Vagrant
// only runs Harvester, and Harvester only runs on Vagrant. Call after either of
// those two selects changes.
func (m *CreateFormModel) coupleProviderAndDistro(changed int) {
	switch changed {
	case fieldProvider:
		if m.isLocalProvider() {
			m.selectOption(fieldK8sDistro, "Harvester")
		} else if m.isHarvesterDistro() {
			m.selectOption(fieldK8sDistro, "RKE2")
		}
	case fieldK8sDistro:
		if m.isHarvesterDistro() {
			m.selectOption(fieldProvider, "libvirt")
		} else if m.isLocalProvider() {
			m.selectOption(fieldProvider, "AWS")
		}
	}
	m.syncProviderVisibility()
}

// awsOnlyFields are form fields that only apply to the AWS/cloud flow. They are
// hidden for the local-VM (Vagrant + Harvester) path, which is self-contained.
// Note: fieldOSImage/fieldCustomAMI are NOT here — they are reused as the
// Harvester version picker on the Vagrant path (see syncProviderVisibility).
var awsOnlyFields = []int{
	fieldCredentials, fieldRegion, fieldSubnetID, fieldSecurityGroup,
	fieldInstanceType,
	fieldSSHKeyName, fieldSSHKeyPath, fieldSSHUser,
	fieldRancherPrime, fieldRancherVersion, fieldBootstrapPwd,
	fieldImageTag, fieldDebugMode, fieldHostPort,
}

// syncProviderVisibility shows/hides fields based on the selected Provider.
// Vagrant (local VMM) runs a self-contained Harvester install and does not use
// any of the AWS/cloud fields, so those are hidden; for AWS we restore them and
// defer to the install-method/AMI visibility rules.
func (m *CreateFormModel) syncProviderVisibility() {
	if m.isLocalProvider() {
		for _, idx := range awsOnlyFields {
			m.fields[idx].hidden = true
		}
		// Installation Method (Local/Docker) and K8s Version are cloud concepts.
		m.fields[fieldInstallMethod].hidden = true
		m.fields[fieldK8sVersion].hidden = true
		// Distribution (Harvester) and Deploy Rancher stay available (a prior
		// Docker-mode toggle may have hidden them).
		m.fields[fieldK8sDistro].hidden = false
		m.fields[fieldDeployRancher].hidden = false

		// Show the Harvester installer fields.
		for _, idx := range harvesterOnlyFields {
			m.fields[idx].hidden = false
		}

		// Reuse the OS Image + Custom AMI pair as the Harvester version picker.
		m.applyLocalVersionPicker()
		return
	}

	// AWS: restore everything, then let the method/AMI rules refine visibility.
	m.fields[fieldInstallMethod].hidden = false
	m.fields[fieldK8sVersion].label = "K8s Version"
	for _, idx := range awsOnlyFields {
		m.fields[idx].hidden = false
	}
	for _, idx := range harvesterOnlyFields {
		m.fields[idx].hidden = true
	}
	m.applyAWSOSImage()
	m.syncInstallMethodVisibility()
}

// Update handles messages
func (m CreateFormModel) Update(msg tea.Msg) (CreateFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle special keys only - let normal typing pass through
		key := msg.String()

		// Only intercept specific navigation/control keys
		switch key {
		case "esc":
			// Close profile selector if open
			if m.showProfileSelect {
				m.showProfileSelect = false
				return m, nil
			}
			// Cancel and return to cluster list
			return m, func() tea.Msg {
				return StateChangeMsg{NewState: StateClusterList}
			}

		case "ctrl+p":
			// Toggle profile selector (changed from 'p' to ctrl+p)
			if m.profiles != nil && m.profiles.HasProfiles() {
				m.showProfileSelect = !m.showProfileSelect
				m.profileSelectIndex = 0
			}
			return m, nil

		case "enter":
			// Handle profile selection
			if m.showProfileSelect {
				profileNames := m.profiles.ListProfiles()
				if m.profileSelectIndex < len(profileNames) {
					m.loadProfileIntoForm(profileNames[m.profileSelectIndex])
					m.showProfileSelect = false
				}
				return m, nil
			}
			// Submit only when the Apply button is focused
			if m.focusIndex == len(m.fields) {
				return m.handleSubmit()
			}
			// Otherwise, move to next field
			m.focusIndex = m.nextVisibleFieldOrApply(m.focusIndex)
			return m, m.updateFocus()

		case "tab", "down":
			// Handle profile selector navigation
			if m.showProfileSelect {
				profileNames := m.profiles.ListProfiles()
				m.profileSelectIndex++
				if m.profileSelectIndex >= len(profileNames) {
					m.profileSelectIndex = 0
				}
				return m, nil
			}
			// Move to next visible field (including Apply button)
			m.focusIndex = m.nextVisibleFieldOrApply(m.focusIndex)
			return m, m.updateFocus()

		case "shift+tab", "up":
			// Handle profile selector navigation
			if m.showProfileSelect {
				profileNames := m.profiles.ListProfiles()
				m.profileSelectIndex--
				if m.profileSelectIndex < 0 {
					m.profileSelectIndex = len(profileNames) - 1
				}
				return m, nil
			}
			// Move to previous visible field (including Apply button)
			m.focusIndex = m.prevVisibleFieldOrApply(m.focusIndex)
			return m, m.updateFocus()

		case "left":
			// For select fields, move to previous option
			if m.focusIndex < len(m.fields) && m.fields[m.focusIndex].fieldType == FieldSelect {
				m.fields[m.focusIndex].selected--
				if m.fields[m.focusIndex].selected < 0 {
					m.fields[m.focusIndex].selected = len(m.fields[m.focusIndex].options) - 1
				}
				if m.focusIndex == fieldOSImage {
					m.syncCustomAMIVisibility()
				}
				if m.focusIndex == fieldInstallMethod {
					m.syncInstallMethodVisibility()
				}
				if m.focusIndex == fieldProvider || m.focusIndex == fieldK8sDistro {
					m.coupleProviderAndDistro(m.focusIndex)
				}
				return m, nil
			}
			// Let text inputs handle left arrow

		case "right":
			// For select fields, move to next option
			if m.focusIndex < len(m.fields) && m.fields[m.focusIndex].fieldType == FieldSelect {
				m.fields[m.focusIndex].selected++
				if m.fields[m.focusIndex].selected >= len(m.fields[m.focusIndex].options) {
					m.fields[m.focusIndex].selected = 0
				}
				if m.focusIndex == fieldOSImage {
					m.syncCustomAMIVisibility()
				}
				if m.focusIndex == fieldInstallMethod {
					m.syncInstallMethodVisibility()
				}
				if m.focusIndex == fieldProvider || m.focusIndex == fieldK8sDistro {
					m.coupleProviderAndDistro(m.focusIndex)
				}
				return m, nil
			}
			// Let text inputs handle right arrow
		}

	case credentialsLoadedForWizardMsg:
		m.credentials = msg.credentials
		// Update credentials field options
		if m.credentials != nil && m.credentials.HasAWSCredentials() {
			credNames := m.credentials.ListAWSCredentials()
			if len(credNames) > 0 {
				m.fields[fieldCredentials].options = credNames
				m.fields[fieldCredentials].selected = 0
			}
		}
		return m, nil

	case clusterCreatedMsg:
		// Cluster created successfully, return to cluster list
		return m, func() tea.Msg {
			return StateChangeMsg{NewState: StateClusterList}
		}

	case clusterCreationErrorMsg:
		// TODO: Show error message to user
		fmt.Printf("Error creating cluster: %v\n", msg.err)
		return m, nil

	case profilesLoadedForFormMsg:
		m.profiles = msg.profiles
		return m, nil

	case amisLoadedForFormMsg:
		m.amis = msg.amis
		if m.amis != nil {
			distros := m.amis.ListDistros()
			m.awsOSOptions = append(distros, "Custom")
			// Only apply to the field when AWS owns it; in Vagrant mode the field
			// is the Harvester version picker.
			if !m.isLocalProvider() {
				m.fields[fieldOSImage].options = m.awsOSOptions
				if m.fields[fieldOSImage].selected >= len(m.awsOSOptions) {
					m.fields[fieldOSImage].selected = 0
				}
				m.syncCustomAMIVisibility()
			}
		}
		return m, nil

	case harvesterVersionsLoadedMsg:
		m.harvesterAvailable = msg.available
		m.harvesterCached = msg.cached
		// Refresh the picker if the Vagrant path currently owns the field.
		if m.isLocalProvider() {
			m.applyLocalVersionPicker()
		}
		return m, nil
	}

	// Update focused text input
	if m.focusIndex < len(m.fields) && m.fields[m.focusIndex].fieldType == FieldText {
		var cmd tea.Cmd
		m.fields[m.focusIndex].input, cmd = m.fields[m.focusIndex].input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m CreateFormModel) handleSubmit() (CreateFormModel, tea.Cmd) {
	provider := m.fields[fieldProvider].options[m.fields[fieldProvider].selected]
	distro := m.fields[fieldK8sDistro].options[m.fields[fieldK8sDistro].selected]

	// Still-unimplemented cloud providers.
	if strings.Contains(provider, "Coming soon") {
		m.statusMsg = "That provider is not implemented yet."
		return m, nil
	}
	m.statusMsg = ""

	// Local VM path: Vagrant + Harvester (self-contained, no cloud credentials).
	if m.isLocalProvider() || strings.Contains(distro, "Harvester") {
		return m.handleLocalVMSubmit()
	}

	if m.isDockerMode() {
		return m.handleDockerSubmit()
	}
	return m.handleLocalSubmit()
}

// handleLocalVMSubmit builds and deploys a local Harvester cluster via Vagrant +
// libvirt. It requires none of the AWS fields (credentials, subnet, AMI, …);
// provider/orchestrator defaults fill in the rest.
// localVMParams gathers every Harvester/Vagrant form value for a local install.
type localVMParams struct {
	name          string
	nodePrefix    string
	nodeCount     int
	version       string
	isoPath       string
	deployRancher bool

	password   string
	token      string
	vip        string
	vipMode    string
	ntpServers string
	mgmtIface  string

	nodeCPU    int
	nodeMemMB  int
	nodeDiskGB int
	subnet     string
	firmware   string
}

func (m CreateFormModel) handleLocalVMSubmit() (CreateFormModel, tea.Cmd) {
	clusterName := m.fields[fieldClusterName].input.Value()
	if clusterName == "" {
		m.statusMsg = "Cluster name is required."
		return m, nil
	}

	p := localVMParams{
		name:          clusterName,
		nodePrefix:    valueOr(m.fields[fieldNodePrefix].input.Value(), "harvester-node"),
		nodeCount:     atoiOr(m.fields[fieldInstanceCount].input.Value(), 3),
		version:       m.selectedHarvesterVersion(),
		isoPath:       strings.TrimSpace(m.fields[fieldHvISOPath].input.Value()),
		deployRancher: m.fields[fieldDeployRancher].options[m.fields[fieldDeployRancher].selected] == "Yes",

		password:   valueOr(m.fields[fieldHvPassword].input.Value(), "p@ssword"),
		token:      valueOr(m.fields[fieldHvToken].input.Value(), "token"),
		vip:        valueOr(m.fields[fieldHvVIP].input.Value(), "192.168.0.131"),
		vipMode:    m.fields[fieldHvVIPMode].options[m.fields[fieldHvVIPMode].selected],
		ntpServers: valueOr(m.fields[fieldHvNTP].input.Value(), "0.suse.pool.ntp.org"),
		mgmtIface:  valueOr(m.fields[fieldHvMgmtIface].input.Value(), "enp1s0"),

		nodeCPU:    atoiOr(m.fields[fieldHvNodeCPU].input.Value(), 8),
		nodeMemMB:  atoiOr(m.fields[fieldHvNodeMem].input.Value(), 16384),
		nodeDiskGB: atoiOr(m.fields[fieldHvNodeDisk].input.Value(), 250),
		subnet:     valueOr(m.fields[fieldHvSubnet].input.Value(), "192.168.0.0/24"),
		firmware:   m.fields[fieldHvFirmware].options[m.fields[fieldHvFirmware].selected],
	}

	// A specific ISO file overrides the version picker; derive the version from
	// its filename so provider and orchestrator stay consistent.
	if p.isoPath != "" {
		if v := deriveVersionFromISOPath(p.isoPath); v != "" {
			p.version = v
		}
	}

	return m, m.createLocalVMCluster(p)
}

// valueOr returns v, or fallback when v is empty.
func valueOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// atoiOr parses v as an int, returning fallback on empty/invalid input.
func atoiOr(v string, fallback int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	n := fallback
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return fallback
	}
	return n
}

func (m CreateFormModel) handleDockerSubmit() (CreateFormModel, tea.Cmd) {
	// Docker mode reuses the cloud provider (AWS) but with docker orchestrator.
	// Collect all cloud/SSH fields the same way as Local, plus Docker-specific fields.
	provider := m.fields[fieldProvider].options[m.fields[fieldProvider].selected]
	credential := ""
	if len(m.fields[fieldCredentials].options) > 0 {
		credential = m.fields[fieldCredentials].options[m.fields[fieldCredentials].selected]
	}
	clusterName := m.fields[fieldClusterName].input.Value()
	nodePrefix := m.fields[fieldNodePrefix].input.Value()
	region := m.fields[fieldRegion].input.Value()
	subnet := m.fields[fieldSubnetID].input.Value()
	securityGroup := m.fields[fieldSecurityGroup].input.Value()
	osImageOption := m.fields[fieldOSImage].options[m.fields[fieldOSImage].selected]
	var ami string
	if osImageOption == "Custom" {
		ami = m.fields[fieldCustomAMI].input.Value()
	} else if m.amis != nil {
		resolvedRegion := region
		if resolvedRegion == "" {
			resolvedRegion = "us-east-1"
		}
		ami, _ = m.amis.GetAMI(osImageOption, resolvedRegion)
	}
	instanceType := m.fields[fieldInstanceType].input.Value()
	sshKeyName := m.fields[fieldSSHKeyName].input.Value()
	sshPrivateKeyPath := m.fields[fieldSSHKeyPath].input.Value()
	sshUser := m.fields[fieldSSHUser].input.Value()
	rancherPrime := m.fields[fieldRancherPrime].options[m.fields[fieldRancherPrime].selected] == "Yes"
	rancherVersion := m.fields[fieldRancherVersion].input.Value()
	bootstrapPassword := m.fields[fieldBootstrapPwd].input.Value()
	imageTag := m.fields[fieldImageTag].input.Value()
	debugMode := m.fields[fieldDebugMode].options[m.fields[fieldDebugMode].selected] == "Yes"
	hostPort := m.fields[fieldHostPort].input.Value()

	// Validate required fields
	if clusterName == "" {
		return m, nil
	}
	if subnet == "" {
		return m, nil
	}
	if securityGroup == "" {
		return m, nil
	}
	if ami == "" {
		return m, nil
	}
	if sshKeyName == "" {
		return m, nil
	}
	if sshPrivateKeyPath == "" {
		return m, nil
	}

	// Defaults
	if nodePrefix == "" {
		nodePrefix = "rancher-docker"
	}
	if region == "" {
		region = "us-east-1"
	}
	if instanceType == "" {
		instanceType = "t3.medium"
	}
	if sshUser == "" {
		sshUser = "ubuntu"
	}
	if rancherVersion == "" {
		rancherVersion = "2.11.7"
	}
	if bootstrapPassword == "" {
		bootstrapPassword = "admin"
	}
	if hostPort == "" {
		hostPort = "443"
	}

	// Docker mode: always deploy Rancher, always 1 instance, distribution = "docker"
	return m, m.createCluster(clusterName, provider, credential, "Docker", "", /*k8sVersion not used*/
		region, subnet, securityGroup, ami, instanceType, nodePrefix, 1,
		sshKeyName, sshPrivateKeyPath, sshUser, true, rancherPrime, rancherVersion, bootstrapPassword,
		imageTag, debugMode)
}

func (m CreateFormModel) handleLocalSubmit() (CreateFormModel, tea.Cmd) {
	// Collect form data
	provider := m.fields[fieldProvider].options[m.fields[fieldProvider].selected]
	credential := ""
	if len(m.fields[fieldCredentials].options) > 0 {
		credential = m.fields[fieldCredentials].options[m.fields[fieldCredentials].selected]
	}
	k8sDistro := m.fields[fieldK8sDistro].options[m.fields[fieldK8sDistro].selected]
	clusterName := m.fields[fieldClusterName].input.Value()
	nodePrefix := m.fields[fieldNodePrefix].input.Value()
	region := m.fields[fieldRegion].input.Value()
	subnet := m.fields[fieldSubnetID].input.Value()
	securityGroup := m.fields[fieldSecurityGroup].input.Value()
	osImageOption := m.fields[fieldOSImage].options[m.fields[fieldOSImage].selected]
	var ami string
	if osImageOption == "Custom" {
		ami = m.fields[fieldCustomAMI].input.Value()
	} else if m.amis != nil {
		resolvedRegion := region
		if resolvedRegion == "" {
			resolvedRegion = "us-east-1"
		}
		ami, _ = m.amis.GetAMI(osImageOption, resolvedRegion)
	}
	instanceType := m.fields[fieldInstanceType].input.Value()
	instanceCountStr := m.fields[fieldInstanceCount].input.Value()
	sshKeyName := m.fields[fieldSSHKeyName].input.Value()
	sshPrivateKeyPath := m.fields[fieldSSHKeyPath].input.Value()
	sshUser := m.fields[fieldSSHUser].input.Value()
	k8sVersion := m.fields[fieldK8sVersion].input.Value()
	deployRancher := m.fields[fieldDeployRancher].options[m.fields[fieldDeployRancher].selected] == "Yes"
	rancherPrime := m.fields[fieldRancherPrime].options[m.fields[fieldRancherPrime].selected] == "Yes"
	rancherVersion := m.fields[fieldRancherVersion].input.Value()
	bootstrapPassword := m.fields[fieldBootstrapPwd].input.Value()
	imageTag := m.fields[fieldImageTag].input.Value()
	debugMode := m.fields[fieldDebugMode].options[m.fields[fieldDebugMode].selected] == "Yes"

	// Validate required fields
	if clusterName == "" {
		return m, nil
	}
	if subnet == "" {
		return m, nil
	}
	if securityGroup == "" {
		return m, nil
	}
	if ami == "" {
		return m, nil
	}
	if sshKeyName == "" {
		return m, nil
	}
	if sshPrivateKeyPath == "" {
		return m, nil
	}

	// Set defaults
	if nodePrefix == "" {
		nodePrefix = "k8s-node"
	}
	if region == "" {
		region = "us-east-1"
	}
	if instanceType == "" {
		instanceType = "t3.xlarge"
	}
	if sshUser == "" {
		sshUser = "ubuntu"
	}
	if k8sVersion == "" {
		if k8sDistro == "RKE2" {
			k8sVersion = "v1.33.7+rke2r1"
		} else {
			k8sVersion = "v1.33.7+k3s1"
		}
	}

	if rancherVersion == "" {
		rancherVersion = "2.11.7"
	}
	if bootstrapPassword == "" {
		bootstrapPassword = "admin"
	}

	instanceCount := 3
	if instanceCountStr != "" {
		fmt.Sscanf(instanceCountStr, "%d", &instanceCount)
	}

	return m, m.createCluster(clusterName, provider, credential, k8sDistro, k8sVersion,
		region, subnet, securityGroup, ami, instanceType, nodePrefix, instanceCount,
		sshKeyName, sshPrivateKeyPath, sshUser, deployRancher, rancherPrime, rancherVersion, bootstrapPassword,
		imageTag, debugMode)
}

func (m CreateFormModel) createCluster(name, provider, credential, distro, version,
	region, subnet, securityGroup, ami, instanceType, nodePrefix string, instanceCount int,
	sshKeyName, sshPrivateKeyPath, sshUser string, deployRancher, rancherPrime bool, rancherVersion, bootstrapPassword string,
	imageTag string, debugMode bool) tea.Cmd {
	return func() tea.Msg {
		// Load clusters config
		cfg, err := config.LoadClustersConfig("config.yaml")
		if err != nil {
			return clusterCreationErrorMsg{err: err}
		}

		// Load credentials to get credential details
		creds, err := credentials.LoadCredentials("cloud-credentials.yaml")
		if err != nil {
			return clusterCreationErrorMsg{err: fmt.Errorf("failed to load credentials: %w", err)}
		}

		awsCred, err := creds.GetAWSCredential(credential)
		if err != nil {
			return clusterCreationErrorMsg{err: fmt.Errorf("credential not found: %w", err)}
		}

		// Create cluster configuration
		cluster := &config.ClusterConfig{
			Provider: config.ProviderSection{
				Type: strings.ToLower(provider),
				Config: map[string]interface{}{
					"region":            region,
					"instance_type":     instanceType,
					"subnet_id":         subnet,
					"security_group_id": securityGroup,
					"ami":               ami,
					"access_key":        awsCred.AccessKey,
					"secret_key":        awsCred.SecretKey,
				},
			},
			Kubernetes: config.KubernetesSection{
				Distribution: strings.ToLower(distro),
				Config: map[string]interface{}{
					"version": version,
				},
			},
			SSH: config.SSHSection{
				KeyName:        sshKeyName,
				PrivateKeyPath: sshPrivateKeyPath,
				User:           sshUser,
			},
			Cluster: config.ClusterSection{
				NodePrefix:    nodePrefix,
				InstanceCount: instanceCount,
			},
			Rancher: config.RancherSection{
				Deploy:            deployRancher,
				Version:           rancherVersion,
				Prime:             rancherPrime,
				BootstrapPassword: bootstrapPassword,
				ImageTag:          imageTag,
				Debug:             debugMode,
			},
			Status: "creating",
		}

		// Add cluster to config
		cfg.AddCluster(name, cluster)

		// Save config
		if err := cfg.Save("config.yaml"); err != nil {
			return clusterCreationErrorMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		// Trigger cluster deployment in background
		go deployCluster(name, cluster)

		return clusterCreatedMsg{name: name}
	}
}

// createLocalVMCluster persists and deploys a local Harvester cluster (Vagrant +
// libvirt) using all the installer values collected from the form.
func (m CreateFormModel) createLocalVMCluster(p localVMParams) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadClustersConfig("config.yaml")
		if err != nil {
			return clusterCreationErrorMsg{err: err}
		}

		// Vagrant provider config (infrastructure: VMs, network, media).
		providerCfg := map[string]interface{}{
			"node_count":     p.nodeCount,
			"node_prefix":    p.nodePrefix,
			"node_cpu":       p.nodeCPU,
			"node_memory_mb": p.nodeMemMB,
			"node_disk_gb":   p.nodeDiskGB,
			"network_subnet": p.subnet,
			"firmware":       p.firmware,
		}
		if p.version != "" {
			providerCfg["harvester_version"] = p.version
		}
		if p.isoPath != "" {
			providerCfg["harvester_iso_path"] = p.isoPath
		}

		// Harvester orchestrator config (installer options).
		kubernetesCfg := map[string]interface{}{
			"password":       p.password,
			"cluster_token":  p.token,
			"vip":            p.vip,
			"vip_mode":       p.vipMode,
			"ntp_servers":    p.ntpServers,
			"mgmt_interface": p.mgmtIface,
		}
		if p.version != "" {
			kubernetesCfg["harvester_version"] = p.version
		}

		cluster := &config.ClusterConfig{
			Provider: config.ProviderSection{
				Type:   "libvirt",
				Config: providerCfg,
			},
			Kubernetes: config.KubernetesSection{
				Distribution: "harvester",
				Config:       kubernetesCfg,
			},
			Cluster: config.ClusterSection{
				NodePrefix:    p.nodePrefix,
				InstanceCount: p.nodeCount,
			},
			Rancher: config.RancherSection{
				Deploy: p.deployRancher,
			},
			Status: "creating",
		}

		cfg.AddCluster(p.name, cluster)
		if err := cfg.Save("config.yaml"); err != nil {
			return clusterCreationErrorMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		go deployCluster(p.name, cluster)

		return clusterCreatedMsg{name: p.name}
	}
}

func (m *CreateFormModel) updateFocus() tea.Cmd {
	for i := range m.fields {
		if m.fields[i].fieldType == FieldText {
			if i == m.focusIndex && !m.fields[i].hidden {
				m.fields[i].input.Focus()
			} else {
				m.fields[i].input.Blur()
			}
		}
	}
	// Blur all text inputs when on the Apply button
	if m.focusIndex == len(m.fields) {
		for i := range m.fields {
			if m.fields[i].fieldType == FieldText {
				m.fields[i].input.Blur()
			}
		}
	}
	return nil
}

// syncCustomAMIVisibility shows/hides field[10] based on the OS Image selection.
// "Custom" is always the last option in field[9].
func (m *CreateFormModel) syncCustomAMIVisibility() {
	customIdx := len(m.fields[fieldOSImage].options) - 1
	isCustom := m.fields[fieldOSImage].selected == customIdx
	m.fields[fieldCustomAMI].hidden = !isCustom
	if m.fields[fieldCustomAMI].hidden {
		m.fields[fieldCustomAMI].input.Blur()
	}
}

// nextVisibleField returns the next non-hidden field index after `from`,
// wrapping around to 0 when the end is reached.
func (m *CreateFormModel) nextVisibleField(from int) int {
	n := len(m.fields)
	for step := 1; step <= n; step++ {
		idx := (from + step) % n
		if !m.fields[idx].hidden {
			return idx
		}
	}
	return from
}

// prevVisibleField returns the previous non-hidden field index before `from`,
// wrapping around to the last field when the start is reached.
func (m *CreateFormModel) prevVisibleField(from int) int {
	n := len(m.fields)
	for step := 1; step <= n; step++ {
		idx := (from - step + n) % n
		if !m.fields[idx].hidden {
			return idx
		}
	}
	return from
}

// nextVisibleFieldOrApply advances to the next visible field, treating
// len(m.fields) as the Apply button position at the end.
func (m *CreateFormModel) nextVisibleFieldOrApply(from int) int {
	// If we're on the Apply button, wrap to the first visible field
	if from == len(m.fields) {
		return m.nextVisibleField(len(m.fields) - 1)
	}
	// Try to find a visible field after `from`
	n := len(m.fields)
	for step := 1; step <= n; step++ {
		idx := from + step
		if idx >= n {
			// Reached the end — land on the Apply button
			return n
		}
		if !m.fields[idx].hidden {
			return idx
		}
	}
	return n
}

// prevVisibleFieldOrApply goes back to the previous visible field, treating
// len(m.fields) as the Apply button position at the end.
func (m *CreateFormModel) prevVisibleFieldOrApply(from int) int {
	// If we're on the Apply button, go to the last visible field
	if from == len(m.fields) {
		for i := len(m.fields) - 1; i >= 0; i-- {
			if !m.fields[i].hidden {
				return i
			}
		}
		return 0
	}
	// If we're on the first visible field, wrap to Apply button
	first := m.nextVisibleField(len(m.fields) - 1)
	if from == first {
		return len(m.fields)
	}
	return m.prevVisibleField(from)
}

func (m CreateFormModel) loadCredentials() tea.Cmd {
	return func() tea.Msg {
		creds, _ := credentials.LoadCredentials("cloud-credentials.yaml")
		return credentialsLoadedForWizardMsg{credentials: creds}
	}
}

func (m CreateFormModel) loadProfiles() tea.Cmd {
	return func() tea.Msg {
		profiles, _ := config.LoadProfiles("profiles.yaml")
		return profilesLoadedForFormMsg{profiles: profiles}
	}
}

func (m CreateFormModel) loadAMIsForForm() tea.Cmd {
	return func() tea.Msg {
		amis, _ := config.LoadAMIs("amis.yaml")
		return amisLoadedForFormMsg{amis: amis}
	}
}

func (m *CreateFormModel) loadProfileIntoForm(profileName string) {
	profile, err := m.profiles.GetProfile(profileName)
	if err != nil {
		return
	}

	// Load profile values into form fields
	m.fields[fieldRegion].input.SetValue(profile.Region)
	m.fields[fieldSubnetID].input.SetValue(profile.SubnetID)
	m.fields[fieldSecurityGroup].input.SetValue(profile.SecurityGroupID)

	// Reverse-lookup the AMI to see if it matches a known distro
	customIdx := len(m.fields[fieldOSImage].options) - 1 // "Custom" is always last
	if m.amis != nil {
		if distro, found := m.amis.FindDistro(profile.AMI, profile.Region); found {
			for i, opt := range m.fields[fieldOSImage].options {
				if opt == distro {
					m.fields[fieldOSImage].selected = i
					break
				}
			}
			m.fields[fieldCustomAMI].input.SetValue("")
		} else {
			m.fields[fieldOSImage].selected = customIdx
			m.fields[fieldCustomAMI].input.SetValue(profile.AMI)
		}
	} else {
		m.fields[fieldOSImage].selected = customIdx
		m.fields[fieldCustomAMI].input.SetValue(profile.AMI)
	}
	m.syncCustomAMIVisibility()

	m.fields[fieldInstanceType].input.SetValue(profile.InstanceType)
	m.fields[fieldSSHKeyName].input.SetValue(profile.SSHKeyName)
	m.fields[fieldSSHKeyPath].input.SetValue(profile.SSHPrivateKeyPath)
	m.fields[fieldSSHUser].input.SetValue(profile.SSHUser)
}

// View renders the form
func (m CreateFormModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(22)

	title := titleStyle.Render("Create New Cluster")
	content := title + "\n\n"

	if m.statusMsg != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		content += statusStyle.Render("⚠ "+m.statusMsg) + "\n\n"
	}

	// Render all fields
	for i, field := range m.fields {
		if field.hidden {
			continue
		}
		focused := i == m.focusIndex
		label := labelStyle.Render(field.label + ":")

		var fieldView string

		if field.fieldType == FieldSelect {
			// Render select field
			selectedValue := ""
			if len(field.options) > 0 && field.selected >= 0 && field.selected < len(field.options) {
				selectedValue = field.options[field.selected]
			}

			if focused {
				fieldView = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ") + label + " "
				fieldView += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render(selectedValue)
				fieldView += lipgloss.NewStyle().Faint(true).Render(" ◀ ▶")
			} else {
				fieldView = "  " + label + " " + selectedValue
			}
		} else {
			// Render text input field
			if focused {
				fieldView = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("▶ ") + label + " " + field.input.View()
			} else {
				fieldView = "  " + label + " " + field.input.View()
			}
		}

		content += fieldView + "\n"
	}

	// Docker mode warning
	if m.isDockerMode() {
		content += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("  ⚠ Docker install is for dev/test only.") + "\n"
	}

	// Apply button
	if m.focusIndex == len(m.fields) {
		content += "\n" + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("▶ [ Apply ]") + "\n"
	} else {
		content += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  [ Apply ]") + "\n"
	}

	help := lipgloss.NewStyle().
		Faint(true).
		Render("\nenter: next/apply • ctrl+p: load profile • esc: cancel • tab/↓: next • shift+tab/↑: prev • ◀/▶: select")

	view := lipgloss.NewStyle().
		Padding(2, 4).
		Render(content + help)

	// Show profile selector overlay if active
	if m.showProfileSelect && m.profiles != nil {
		view = m.renderProfileSelector(view)
	}

	return view
}

func (m CreateFormModel) renderProfileSelector(baseView string) string {
	profileNames := m.profiles.ListProfiles()
	if len(profileNames) == 0 {
		return baseView
	}

	selectorStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Background(lipgloss.Color("235"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	selector := titleStyle.Render("Load Profile") + "\n\n"

	for i, name := range profileNames {
		if i == m.profileSelectIndex {
			selector += lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Render("▶ " + name + "\n")
		} else {
			selector += "  " + name + "\n"
		}
	}

	selector += "\n" + lipgloss.NewStyle().Faint(true).Render("enter: load • esc: cancel")

	selectorBox := selectorStyle.Render(selector)

	// Overlay the selector in the center
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		selectorBox,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

// deployCluster runs the cluster deployment in background using the actual workflow
func deployCluster(name string, clusterConfig *config.ClusterConfig) {
	// Create log file for this deployment
	logPath := fmt.Sprintf("logs/%s.log", name)
	os.MkdirAll("logs", 0755)

	// Create log file
	logFile, err := os.Create(logPath)
	if err != nil {
		updateClusterStatus(name, "failed")
		return
	}
	defer logFile.Close()

	// Helper to write logs
	writeLog := func(message string) {
		timestamp := getTimestamp()
		logLine := fmt.Sprintf("[%s] %s\n", timestamp, message)
		logFile.WriteString(logLine)
		logFile.Sync() // Flush to disk immediately
	}

	writeLog(fmt.Sprintf("=== Starting deployment for cluster: %s ===", name))
	writeLog("Converting cluster configuration...")

	// Convert ClusterConfig to config.Config for the workflow
	cfg := clusterConfig.ToModernConfig()
	cfg.ClusterName = name

	writeLog(fmt.Sprintf("Provider: %s", cfg.Provider))
	writeLog(fmt.Sprintf("Orchestrator: %s", cfg.Orchestrator))
	writeLog(fmt.Sprintf("Instance Count: %d", cfg.InstanceCount))

	// Create build directory for this cluster
	buildDir := filepath.Join("clusters", name)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		writeLog(fmt.Sprintf("ERROR: Failed to create build directory: %v", err))
		updateClusterStatus(name, "failed")
		return
	}

	writeLog(fmt.Sprintf("Build directory: %s", buildDir))

	// Update cluster config with build directory
	clusterConfig.BuildDir = buildDir
	clustersConfig, _ := config.LoadClustersConfig("config.yaml")
	clustersConfig.AddCluster(name, clusterConfig)
	clustersConfig.Save("config.yaml")

	// Create the workflow runner
	writeLog("Creating workflow runner...")
	runner, err := workflow.NewModularRunner(cfg, core.GlobalRegistry)
	if err != nil {
		writeLog(fmt.Sprintf("ERROR: Failed to create runner: %v", err))
		updateClusterStatus(name, "failed")
		return
	}

	// Redirect runner output to log file
	writeLog("Starting infrastructure deployment...")

	// Run the deployment with output capture
	if err := runDeploymentWithLogging(runner, buildDir, logFile, writeLog); err != nil {
		writeLog(fmt.Sprintf("ERROR: Deployment failed: %v", err))
		updateClusterStatus(name, "failed")
		return
	}

	writeLog(fmt.Sprintf("✓ Cluster %s deployed successfully!", name))

	// Get outputs and update cluster config
	// TODO: Pass proper context instead of nil
	if outputs, err := runner.Provider.GetOutputs(nil, buildDir); err == nil {
		clusterConfig.InstanceIPs = outputs.InstanceIPs
		clusterConfig.InstanceDNS = outputs.InstanceDNSNames
		if len(outputs.InstanceDNSNames) > 0 {
			clusterConfig.RancherURL = fmt.Sprintf("https://%s", outputs.InstanceDNSNames[0])
		}
		// Harvester (libvirt) dashboard lives on the VIP; surface it as the URL.
		if vip, ok := clusterConfig.Kubernetes.Config["vip"].(string); ok && vip != "" {
			clusterConfig.RancherURL = fmt.Sprintf("https://%s", vip)
		}
		clustersConfig.AddCluster(name, clusterConfig)
		clustersConfig.Save("config.yaml")

		writeLog(fmt.Sprintf("Instance IPs: %v", outputs.InstanceIPs))
		writeLog(fmt.Sprintf("Instance DNS: %v", outputs.InstanceDNSNames))
	}

	// Update cluster status to running
	updateClusterStatus(name, "running")
}

// runDeploymentWithLogging runs the deployment and captures all output
func runDeploymentWithLogging(runner *workflow.ModularRunner, buildDir string, logFile *os.File, writeLog func(string)) error {
	// Create a multi-writer to write to both log file and capture output
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	// Channel to signal completion
	done := make(chan error, 1)

	// Goroutine to copy output to log file
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				logFile.Write(buf[:n])
				logFile.Sync()
			}
			if err != nil {
				break
			}
		}
	}()

	// Run the deployment
	go func() {
		done <- runner.RunWithBuildDir(buildDir)
	}()

	// Wait for completion
	err := <-done

	// Restore stdout/stderr
	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return err
}

// getTimestamp returns a formatted timestamp
func getTimestamp() string {
	return time.Now().Format("15:04:05")
}

// updateClusterStatus updates the status of a cluster in config.yaml
func updateClusterStatus(name, status string) {
	cfg, err := config.LoadClustersConfig("config.yaml")
	if err != nil {
		return
	}

	cluster, exists := cfg.GetCluster(name)
	if !exists {
		return
	}

	cluster.Status = status
	cfg.AddCluster(name, cluster)
	cfg.Save("config.yaml")
}

// Message types
type credentialsLoadedForWizardMsg struct {
	credentials *credentials.CloudCredentials
}

type clusterCreatedMsg struct {
	name string
}

type clusterCreationErrorMsg struct {
	err error
}

type profilesLoadedForFormMsg struct {
	profiles *config.ProfilesConfig
}

type amisLoadedForFormMsg struct {
	amis *config.AMIsConfig
}
