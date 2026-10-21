package views

import (
	"testing"

	"github.com/Felipalds/rancher-tui/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func treeWithClusters(names ...string) ClusterTreeModel {
	m := NewClusterTreeModel()
	clusters := map[string]*config.ClusterConfig{}
	for _, n := range names {
		clusters[n] = &config.ClusterConfig{Status: "running"}
	}
	var cmd tea.Cmd
	m, cmd = m.Update(clustersLoadedMsg{clusters: clusters, names: names})
	_ = cmd
	return m
}

func TestClusterTree_Navigation(t *testing.T) {
	m := treeWithClusters("a", "b", "c")
	assert.Equal(t, 0, m.cursor)

	m, _ = m.Update(keyRunes("j"))
	assert.Equal(t, 1, m.cursor)
	m, _ = m.Update(keyRunes("j"))
	m, _ = m.Update(keyRunes("j")) // clamp at last
	assert.Equal(t, 2, m.cursor)

	m, _ = m.Update(keyRunes("k"))
	assert.Equal(t, 1, m.cursor)

	m, _ = m.Update(keyRunes("g"))
	assert.Equal(t, 0, m.cursor)
	m, _ = m.Update(keyRunes("G"))
	assert.Equal(t, 2, m.cursor)
}

func TestClusterTree_ExpandCollapse(t *testing.T) {
	m := treeWithClusters("a", "b")
	assert.False(t, m.expanded["a"])

	m, _ = m.Update(keyRunes("l"))
	assert.True(t, m.expanded["a"])
	m, _ = m.Update(keyRunes("h"))
	assert.False(t, m.expanded["a"])
}

func TestClusterTree_EnterOpensLogs(t *testing.T) {
	m := treeWithClusters("a", "b")
	m, _ = m.Update(keyRunes("j")) // cursor -> "b"

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	open, ok := msg.(OpenClusterLogsMsg)
	require.True(t, ok, "enter should emit OpenClusterLogsMsg")
	assert.Equal(t, "b", open.ClusterName)
}

func TestClusterTree_SelectedName_Empty(t *testing.T) {
	m := NewClusterTreeModel()
	assert.Equal(t, "", m.SelectedName())
}

func TestClusterTree_CursorClampsOnReload(t *testing.T) {
	m := treeWithClusters("a", "b", "c")
	m, _ = m.Update(keyRunes("G")) // cursor = 2
	// Reload with fewer clusters.
	m, _ = m.Update(clustersLoadedMsg{
		clusters: map[string]*config.ClusterConfig{"a": {}},
		names:    []string{"a"},
	})
	assert.Equal(t, 0, m.cursor)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "ab…", truncate("abcdef", 3))
	assert.Equal(t, "", truncate("abc", 0))
}
