package tui

import (
	"fmt"

	"github.com/Felipalds/rancher-tui/internal/tui/views"
	tea "github.com/charmbracelet/bubbletea"
)

// RootModel is the main state machine that routes between different views
type RootModel struct {
	state           views.AppState
	width           int
	height          int
	header          HeaderModel
	footer          FooterModel
	help            HelpModel
	clusterList     views.ClusterListModel
	createForm      views.CreateFormModel
	deleteModal     views.DeleteModalModel
	credentialsList views.CredentialsListModel
	credentialsForm views.CredentialsFormModel
	profilesList    views.ProfilesListModel
	profilesForm    views.ProfilesFormModel
	amisList        views.AMIsListModel
	amisForm        views.AMIsFormModel
	upgradeForm     views.UpgradeFormModel
	ready           bool
}

// NewRootModel creates a new root model with initial state
func NewRootModel() RootModel {
	return RootModel{
		state:           views.StateClusterList,
		width:           80,
		height:          24,
		header:          NewHeaderModel(),
		footer:          NewFooterModel(),
		help:            NewHelpModel(),
		clusterList:     views.NewClusterListModel(),
		createForm:      views.NewCreateFormModel(),
		deleteModal:     views.NewDeleteModalModel(),
		credentialsList: views.NewCredentialsListModel(),
		credentialsForm: views.NewCredentialsFormModel(),
		profilesList:    views.NewProfilesListModel(),
		profilesForm:    views.NewProfilesFormModel(),
		amisList:        views.NewAMIsListModel(),
		amisForm:        views.NewAMIsFormModel(),
		upgradeForm:     views.NewUpgradeFormModel(),
		ready:           false,
	}
}

// Init initializes the root model
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		m.clusterList.Init(),
	)
}

// Update handles all messages and routes them appropriately
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update all child components with new size
		m.header.SetWidth(m.width)
		m.footer.SetWidth(m.width)
		m.footer.SetHeight(m.height)

		// Calculate content height: full height minus header(2) and footer(3)
		contentHeight := m.height - 5
		if contentHeight < 10 {
			contentHeight = 10
		}
		m.clusterList.SetSize(m.width, contentHeight)
		m.createForm.SetSize(m.width, contentHeight)
		m.deleteModal.SetSize(m.width, contentHeight)
		m.credentialsList.SetSize(m.width, contentHeight)
		m.credentialsForm.SetSize(m.width, contentHeight)
		m.profilesList.SetSize(m.width, contentHeight)
		m.profilesForm.SetSize(m.width, contentHeight)
		m.amisList.SetSize(m.width, contentHeight)
		m.amisForm.SetSize(m.width, contentHeight)
		m.upgradeForm.SetSize(m.width, contentHeight)

		return m, nil

	case tea.KeyMsg:
		// Global keybindings
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			// Only quit from cluster list view
			if m.state == views.StateClusterList {
				return m, tea.Quit
			}

		case "?":
			// Toggle help overlay
			if m.help.IsVisible() {
				m.help.Toggle()
			} else {
				m.help.Toggle()
			}
			return m, nil

		case "esc":
			// Close help if open
			if m.help.IsVisible() {
				m.help.Toggle()
				return m, nil
			}
			// Otherwise, navigate back to cluster list
			if m.state != views.StateClusterList {
				m.state = views.StateClusterList
				return m, m.clusterList.Init()
			}
		}

		// Route to active view if help is not visible
		if !m.help.IsVisible() {
			return m.routeUpdate(msg)
		}

		// Handle help overlay
		if m.help.IsVisible() {
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}

	case views.StateChangeMsg:
		// Handle state changes from child views
		m.state = msg.NewState

		// Initialize the new view
		switch msg.NewState {
		case views.StateCreateForm:
			return m, m.createForm.Init()
		case views.StateDeleteConfirm:
			m.deleteModal.SetCluster(msg.Data.(string))
			return m, nil
		case views.StateClusterList:
			return m, m.clusterList.Init()
		case views.StateCredentialsList:
			return m, m.credentialsList.Init()
		case views.StateCredentialsForm:
			if msg.Data != nil {
				// Check if it's a signal to return to create form
				if str, ok := msg.Data.(string); ok {
					if str == "return_to_create" {
						m.credentialsForm.SetReturnTo(views.StateCreateForm)
						return m, m.credentialsForm.Init()
					} else if str != "credential_saved" {
						// Edit existing credential
						return m, m.credentialsForm.SetEditMode(str)
					}
				}
			}
			// Create new credential
			return m, m.credentialsForm.Init()
		case views.StateProfilesList:
			return m, m.profilesList.Init()
		case views.StateProfilesForm:
			if msg.Data != nil {
				if profileName, ok := msg.Data.(string); ok {
					// Edit existing profile
					return m, m.profilesForm.SetEditMode(profileName)
				}
			}
			// Create new profile
			return m, m.profilesForm.Init()
		case views.StateAMIsList:
			return m, m.amisList.Init()
		case views.StateAMIsForm:
			if msg.Data != nil {
				if key, ok := msg.Data.(string); ok && key != "" {
					// Edit existing entry
					return m, m.amisForm.SetEditMode(key)
				}
			}
			// Create new entry
			m.amisForm = views.NewAMIsFormModel()
			m.amisForm.SetSize(m.width, m.height-5)
			return m, m.amisForm.Init()
		case views.StateUpgradeForm:
			if msg.Data != nil {
				if clusterName, ok := msg.Data.(string); ok {
					return m, m.upgradeForm.SetCluster(clusterName)
				}
			}
			return m, nil
		}
		return m, nil

	case views.ClusterDeletedMsg:
		// Refresh cluster list after deletion
		m.state = views.StateClusterList
		return m, m.clusterList.Init()
	}

	// Route to active view
	return m.routeUpdate(msg)
}

// routeUpdate routes messages to the active view
func (m RootModel) routeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.state {
	case views.StateClusterList:
		m.clusterList, cmd = m.clusterList.Update(msg)
	case views.StateCreateForm:
		m.createForm, cmd = m.createForm.Update(msg)
	case views.StateDeleteConfirm:
		m.deleteModal, cmd = m.deleteModal.Update(msg)
	case views.StateCredentialsList:
		m.credentialsList, cmd = m.credentialsList.Update(msg)
	case views.StateCredentialsForm:
		m.credentialsForm, cmd = m.credentialsForm.Update(msg)
	case views.StateProfilesList:
		m.profilesList, cmd = m.profilesList.Update(msg)
	case views.StateProfilesForm:
		m.profilesForm, cmd = m.profilesForm.Update(msg)
	case views.StateAMIsList:
		m.amisList, cmd = m.amisList.Update(msg)
	case views.StateAMIsForm:
		m.amisForm, cmd = m.amisForm.Update(msg)
	case views.StateUpgradeForm:
		m.upgradeForm, cmd = m.upgradeForm.Update(msg)
	}

	return m, cmd
}

// View renders the entire application
func (m RootModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Show help overlay if visible
	if m.help.IsVisible() {
		return m.help.View()
	}

	// Build the layout
	header := m.header.View()
	headerHeight := 2    // header + newline
	footerBarHeight := 3 // border + 1 line + padding

	// Get content from active view
	var content string
	var footer string
	switch m.state {
	case views.StateClusterList:
		// Check if logs should be shown
		if selectedCluster, showLogs := m.clusterList.GetSelectedCluster(); showLogs {
			// Log panel takes 33% of screen; shrink cluster list to fit
			logPanelHeight := m.height / 3
			if logPanelHeight < 6 {
				logPanelHeight = 6
			}
			reducedHeight := m.height - headerHeight - logPanelHeight - 1
			if reducedHeight < 6 {
				reducedHeight = 6
			}
			m.clusterList.SetSize(m.width, reducedHeight)
			content = m.clusterList.View()
			footer = m.footer.ViewWithLogs(selectedCluster)
		} else {
			// No logs: cluster list gets full content area
			fullHeight := m.height - headerHeight - footerBarHeight
			if fullHeight < 10 {
				fullHeight = 10
			}
			m.clusterList.SetSize(m.width, fullHeight)
			content = m.clusterList.View()
			footer = m.footer.ViewForStateWithError(m.state, m.clusterList.GetCursorClusterError())
		}
	case views.StateCreateForm:
		content = m.createForm.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateDeleteConfirm:
		// Show cluster list in background with modal overlay
		content = m.clusterList.View()
		content = m.deleteModal.ViewOver(content)
		footer = m.footer.ViewForState(m.state)
	case views.StateCredentialsList:
		content = m.credentialsList.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateCredentialsForm:
		content = m.credentialsForm.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateProfilesList:
		content = m.profilesList.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateProfilesForm:
		content = m.profilesForm.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateAMIsList:
		content = m.amisList.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateAMIsForm:
		content = m.amisForm.View()
		footer = m.footer.ViewForState(m.state)
	case views.StateUpgradeForm:
		content = m.upgradeForm.View()
		footer = m.footer.ViewForState(m.state)
	default:
		content = fmt.Sprintf("State: %s (not implemented)", m.state)
		footer = m.footer.ViewForState(m.state)
	}

	// Combine all parts
	return fmt.Sprintf("%s\n%s\n%s", header, content, footer)
}
