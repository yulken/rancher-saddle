package views

// AppState represents the current view/state of the application
type AppState int

const (
	StateClusterList AppState = iota
	StateCreateForm
	StateDeleteConfirm
	StateClusterDetails
	StateCredentialsList
	StateCredentialsForm
	StateProfilesList
	StateProfilesForm
	StateAMIsList
	StateAMIsForm
	StateUpgradeForm
	StateHelp
	StateQuitting
	StateDashboard
)

// String returns the string representation of the state
func (s AppState) String() string {
	switch s {
	case StateDashboard:
		return "Dashboard"
	case StateClusterList:
		return "Cluster List"
	case StateCreateForm:
		return "Create Cluster"
	case StateDeleteConfirm:
		return "Delete Cluster"
	case StateClusterDetails:
		return "Cluster Details"
	case StateCredentialsList:
		return "Credentials"
	case StateCredentialsForm:
		return "Edit Credentials"
	case StateProfilesList:
		return "Profiles"
	case StateProfilesForm:
		return "Edit Profile"
	case StateAMIsList:
		return "AMI Catalog"
	case StateAMIsForm:
		return "Edit AMI"
	case StateUpgradeForm:
		return "Upgrade Rancher"
	case StateHelp:
		return "Help"
	case StateQuitting:
		return "Quitting"
	default:
		return "Unknown"
	}
}

// StateChangeMsg is sent when a view wants to change the app state
type StateChangeMsg struct {
	NewState AppState
	Data     interface{}
}

// ClusterDeletedMsg is sent when a cluster is successfully deleted
type ClusterDeletedMsg struct {
	ClusterName string
}

// ClusterCreatedMsg is sent when a cluster is successfully created
type ClusterCreatedMsg struct {
	ClusterName string
}

// OpenClusterLogsMsg is emitted by the cluster tree to jump to the cluster list
// and open the log panel for a specific cluster.
type OpenClusterLogsMsg struct {
	ClusterName string
}
