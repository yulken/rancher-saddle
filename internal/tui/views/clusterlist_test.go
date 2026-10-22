package views

import (
	"os"
	"testing"

	"github.com/Felipalds/rancher-tui/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestClusterList_AutoShowFailedLogs(t *testing.T) {
	m := NewClusterListModel()
	m.clusters = map[string]*config.ClusterConfig{
		"ok":     {Status: "running"},
		"broken": {Status: "failed"},
	}
	m.clusterNames = []string{"ok", "broken"}

	// A failed cluster auto-opens its logs.
	m.autoShowFailedLogs()
	assert.True(t, m.showLogs)
	assert.Equal(t, "broken", m.selectedCluster)
}

func TestClusterList_AutoShow_DoesNotReopenAfterHide(t *testing.T) {
	m := NewClusterListModel()
	m.clusters = map[string]*config.ClusterConfig{"broken": {Status: "failed"}}
	m.clusterNames = []string{"broken"}

	m.autoShowFailedLogs()
	assert.True(t, m.showLogs)

	// User hides the logs.
	m.showLogs = false
	m.selectedCluster = ""

	// Still failed on the next refresh -> must NOT re-open.
	m.autoShowFailedLogs()
	assert.False(t, m.showLogs, "should not reopen logs the user hid")
}

func TestClusterList_AutoShow_ReopensOnNewFailure(t *testing.T) {
	m := NewClusterListModel()
	m.clusters = map[string]*config.ClusterConfig{"c": {Status: "failed"}}
	m.clusterNames = []string{"c"}

	m.autoShowFailedLogs()
	m.showLogs = false // user hides

	// Cluster recovers...
	m.clusters["c"].Status = "running"
	m.autoShowFailedLogs()
	assert.False(t, m.showLogs)

	// ...then fails again -> reopens.
	m.clusters["c"].Status = "failed"
	m.autoShowFailedLogs()
	assert.True(t, m.showLogs)
	assert.Equal(t, "c", m.selectedCluster)
}

func TestCopyLogsCmd(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("logs", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("logs/demo.log", []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	msg := copyLogsCmd("demo")()
	copied, ok := msg.(logsCopiedMsg)
	assert.True(t, ok)
	assert.Contains(t, copied.notice, "copied 3 lines")
}

func TestCopyLogsCmd_NoFile(t *testing.T) {
	t.Chdir(t.TempDir())
	msg := copyLogsCmd("missing")()
	copied, ok := msg.(logsCopiedMsg)
	assert.True(t, ok)
	assert.Equal(t, "no logs to copy", copied.notice)
}

func TestClusterList_ShowLogsFor(t *testing.T) {
	m := NewClusterListModel()
	m.clusters = map[string]*config.ClusterConfig{"a": {}, "b": {}}
	m.clusterNames = []string{"a", "b"}

	m.ShowLogsFor("b")
	assert.True(t, m.showLogs)
	assert.Equal(t, "b", m.selectedCluster)
}

// Regression: a refresh that sees config.yaml mid-write empties the table and
// leaves the cursor at -1. The next key press then indexed clusterNames[-1] and
// panicked with "index out of range [-1]" (seen right after a deploy started).
func TestClusterList_EmptyRefreshThenKeyDoesNotPanic(t *testing.T) {
	m := NewClusterListModel()

	// Startup load: no clusters.
	m, _ = m.Update(clustersLoadedMsg{
		clusters: map[string]*config.ClusterConfig{},
		names:    []string{},
	})

	// Deploy wrote config.yaml; the list now has the cluster.
	m, _ = m.Update(clustersLoadedMsg{
		clusters: map[string]*config.ClusterConfig{"test": {Status: "creating"}},
		names:    []string{"test"},
	})

	assert.Equal(t, 0, m.table.Cursor())

	// Toggle logs on the selected cluster: must select "test", not panic.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.showLogs)
	assert.Equal(t, "test", m.selectedCluster)
}

func TestClusterList_KeysWithNoRowsAreIgnored(t *testing.T) {
	m := NewClusterListModel()
	m, _ = m.Update(clustersLoadedMsg{
		clusters: map[string]*config.ClusterConfig{},
		names:    []string{},
	})

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune("d")},
		{Type: tea.KeyRunes, Runes: []rune("u")},
		{Type: tea.KeyRunes, Runes: []rune("y")},
	} {
		m, _ = m.Update(key)
	}

	assert.False(t, m.showLogs)
	assert.Empty(t, m.selectedCluster)
}
