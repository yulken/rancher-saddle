package views

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Felipalds/rancher-tui/internal/providers/libvirt"
	tea "github.com/charmbracelet/bubbletea"
)

// isoVersionRe extracts the version from a Harvester ISO filename, e.g.
// "harvester-v1.4.1-amd64.iso" -> "v1.4.1".
var isoVersionRe = regexp.MustCompile(`^harvester-(.+)-amd64\.iso$`)

// deriveVersionFromISOPath returns the Harvester version encoded in an ISO file
// name, or "" when it can't be parsed.
func deriveVersionFromISOPath(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	if m := isoVersionRe.FindStringSubmatch(base); m != nil {
		return m[1]
	}
	return ""
}

// customVersionOption is the sentinel (always last) that reveals the free-text
// version field, mirroring the "Custom" option used for AMIs.
const customVersionOption = "Custom"

// downloadedSuffix labels cached versions in the picker.
const downloadedSuffix = " (downloaded)"

// awsOSImageOptions is the default OS Image option set for the AWS path; it is
// replaced by the AMI catalog once loaded.
var awsOSImageOptions = []string{"Ubuntu 22.04 LTS", "RHEL 9", "SLES 15 SP5", "Custom"}

// defaultHarvesterVersions is the fallback list shown when the GitHub releases
// API is unavailable (offline / rate-limited).
var defaultHarvesterVersions = []string{"v1.4.1", "v1.4.0", "v1.3.2", "v1.3.1", "master"}

// harvesterVersionsLoadedMsg carries the version lists into the form.
type harvesterVersionsLoadedMsg struct {
	available []string // from GitHub releases (or fallback)
	cached    []string // ISOs already downloaded locally
}

// loadHarvesterVersions fetches available releases from GitHub and scans the
// local ISO cache, off the UI thread.
func loadHarvesterVersions() tea.Cmd {
	return func() tea.Msg {
		return harvesterVersionsLoadedMsg{
			available: fetchHarvesterReleases(),
			cached:    libvirt.ListCachedVersions("."),
		}
	}
}

// ghRelease is the subset of the GitHub release object we care about.
type ghRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
}

// fetchHarvesterReleases returns Harvester release tags from the GitHub API, or
// the fallback list on any error.
func fetchHarvesterReleases() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet,
		"https://api.github.com/repos/harvester/harvester/releases?per_page=30", nil)
	if err != nil {
		return defaultHarvesterVersions
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return defaultHarvesterVersions
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return defaultHarvesterVersions
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return defaultHarvesterVersions
	}

	var tags []string
	for _, r := range releases {
		if r.Draft || r.TagName == "" {
			continue
		}
		tags = append(tags, r.TagName)
	}
	if len(tags) == 0 {
		return defaultHarvesterVersions
	}
	return tags
}

// harvesterVersionOptions builds the picker options: cached versions first (so
// "already downloaded" ISOs are easy to pick), then available releases, then the
// custom sentinel.
func (m *CreateFormModel) harvesterVersionOptions() []string {
	var opts []string
	seen := map[string]bool{}

	for _, v := range m.harvesterCached {
		opts = append(opts, v+downloadedSuffix)
		seen[v] = true
	}

	available := m.harvesterAvailable
	if len(available) == 0 {
		available = defaultHarvesterVersions
	}
	for _, v := range available {
		if seen[v] {
			continue
		}
		opts = append(opts, v)
		seen[v] = true
	}

	opts = append(opts, customVersionOption)
	return opts
}

// applyLocalVersionPicker repurposes the OS Image + Custom AMI fields as the
// Harvester version picker, preserving the current selection where possible.
func (m *CreateFormModel) applyLocalVersionPicker() {
	prev := ""
	if m.fields[fieldOSImage].selected < len(m.fields[fieldOSImage].options) {
		prev = m.fields[fieldOSImage].options[m.fields[fieldOSImage].selected]
	}

	opts := m.harvesterVersionOptions()
	m.fields[fieldOSImage].label = "Harvester Version"
	m.fields[fieldOSImage].options = opts
	m.fields[fieldOSImage].hidden = false

	// Restore the previous selection if it still exists, else default to first.
	m.fields[fieldOSImage].selected = 0
	for i, o := range opts {
		if o == prev {
			m.fields[fieldOSImage].selected = i
			break
		}
	}

	m.fields[fieldCustomAMI].label = "Custom Version"
	m.fields[fieldCustomAMI].input.Placeholder = "v1.4.1"
	m.syncCustomAMIVisibility()
}

// applyAWSOSImage restores the OS Image + Custom AMI fields for the AWS path.
func (m *CreateFormModel) applyAWSOSImage() {
	opts := m.awsOSOptions
	if len(opts) == 0 {
		opts = awsOSImageOptions
	}
	m.fields[fieldOSImage].label = "OS Image"
	m.fields[fieldOSImage].options = opts
	if m.fields[fieldOSImage].selected >= len(opts) {
		m.fields[fieldOSImage].selected = 0
	}
	m.fields[fieldCustomAMI].label = "Custom AMI ID"
	m.fields[fieldCustomAMI].input.Placeholder = "ami-xxxxx"
}

// selectedHarvesterVersion resolves the version string from the picker: the
// custom text field when "Custom" is chosen, otherwise the selected option with
// any "(downloaded)" marker stripped.
func (m *CreateFormModel) selectedHarvesterVersion() string {
	opts := m.fields[fieldOSImage].options
	if len(opts) == 0 {
		return ""
	}
	sel := opts[m.fields[fieldOSImage].selected]
	if sel == customVersionOption {
		return strings.TrimSpace(m.fields[fieldCustomAMI].input.Value())
	}
	return strings.TrimSuffix(sel, downloadedSuffix)
}
